package claim

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

const (
	typeClaim    = "claim"
	typeDecision = "decision"
)

type status int

const (
	pending status = iota
	confirmed
	rejected
)

// nsLog is the storage of one namespace: an append-only JSONL file with one
// writer. The file is the truth; everything in memory is folded from it.
// Methods are safe for concurrent use, but only one nsLog may have the file open.
type nsLog struct {
	f   *os.File
	now func() time.Time

	mu     sync.Mutex // guards every field below
	order  []string   // claim ids in log order
	claims map[string]Claim
	state  map[string]status
}

// openNSLog opens the log at path, creating it if missing, and folds what is there.
func openNSLog(path string) (*nsLog, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("claim: open log: %w", err)
	}
	l := &nsLog{
		f:      f,
		now:    func() time.Time { return time.Now().UTC() },
		claims: map[string]Claim{},
		state:  map[string]status{},
	}
	if err := l.replay(); err != nil {
		f.Close()
		return nil, fmt.Errorf("claim: read %s: %w", path, err)
	}
	return l, nil
}

// Close closes the file.
func (l *nsLog) Close() error { return l.f.Close() }

func (l *nsLog) replay() error {
	sc := bufio.NewScanner(l.f)
	sc.Buffer(nil, 1<<24)
	for n := 1; sc.Scan(); n++ {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var head struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(sc.Bytes(), &head); err != nil {
			return fmt.Errorf("line %d: %w", n, err)
		}
		switch head.Type {
		case typeClaim:
			var c Claim
			if err := json.Unmarshal(sc.Bytes(), &c); err != nil {
				return fmt.Errorf("line %d: %w", n, err)
			}
			l.applyClaim(c)
		case typeDecision:
			var d Decision
			if err := json.Unmarshal(sc.Bytes(), &d); err != nil {
				return fmt.Errorf("line %d: %w", n, err)
			}
			l.applyDecision(d)
		default:
			return fmt.Errorf("line %d: unknown type %q", n, head.Type)
		}
	}
	return sc.Err()
}

func (l *nsLog) applyClaim(c Claim) {
	l.order = append(l.order, c.ID)
	l.claims[c.ID] = c
	if c.Author.Kind == User {
		l.state[c.ID] = confirmed
	} else {
		l.state[c.ID] = pending
	}
}

func (l *nsLog) applyDecision(d Decision) {
	if d.Outcome == Confirm {
		l.state[d.ClaimID] = confirmed
	} else {
		l.state[d.ClaimID] = rejected
	}
}

// liveSet is the fold: a claim is live when it is confirmed and nothing live
// supersedes or retracts it. A link only points at an earlier claim, so one
// pass from the newest entry back decides every claim.
func (l *nsLog) liveSet() map[string]bool {
	live := map[string]bool{}
	gone := map[string]bool{}
	for i := len(l.order) - 1; i >= 0; i-- {
		id := l.order[i]
		if l.state[id] != confirmed || gone[id] {
			continue
		}
		c := l.claims[id]
		for _, target := range c.Supersedes {
			gone[target] = true
		}
		if c.Retracted != "" {
			gone[c.Retracted] = true
			continue
		}
		live[id] = true
	}
	return live
}

// Live returns the live claims in log order.
func (l *nsLog) Live() []Claim {
	l.mu.Lock()
	defer l.mu.Unlock()
	live := l.liveSet()
	var out []Claim
	for _, id := range l.order {
		if live[id] {
			out = append(out, l.claims[id])
		}
	}
	return out
}

// Proposals returns the claims that wait for a decision, in log order.
func (l *nsLog) Proposals() []Claim {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []Claim
	for _, id := range l.order {
		if l.state[id] == pending {
			out = append(out, l.claims[id])
		}
	}
	return out
}

// Append checks c and writes it. A claim by a User is live at once; a claim by
// a Proposer is a proposal until a User decides it. An empty ID is filled in,
// and RecordedAt is always set by the log. It returns the claim as recorded.
func (l *nsLog) Append(c Claim) (Claim, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if c.ID == "" {
		id, err := newID()
		if err != nil {
			return Claim{}, err
		}
		c.ID = id
	} else if _, dup := l.claims[c.ID]; dup {
		return Claim{}, invalid("id %q is already in the log", c.ID)
	}
	live := l.liveSet()
	if err := validClaim(c, func(id string) bool { return live[id] }); err != nil {
		return Claim{}, err
	}
	c.RecordedAt = l.now()
	if err := l.write(typeClaim, c); err != nil {
		return Claim{}, err
	}
	l.applyClaim(c)
	return c, nil
}

// Decide settles a pending proposal. Only a User may decide. The rules are not
// checked again: they hold for a claim when it is written.
func (l *nsLog) Decide(d Decision) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if d.Author.Kind != User || d.Author.ID == "" {
		return invalid("only a user decides")
	}
	if d.Outcome != Confirm && d.Outcome != Reject {
		return invalid("unknown outcome %q", d.Outcome)
	}
	if l.state[d.ClaimID] != pending || l.claims[d.ClaimID].ID == "" {
		return invalid("claim %q is not a pending proposal", d.ClaimID)
	}
	d.RecordedAt = l.now()
	if err := l.write(typeDecision, d); err != nil {
		return err
	}
	l.applyDecision(d)
	return nil
}

// write appends one line and syncs it. v's fields are inlined next to "type".
func (l *nsLog) write(kind string, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("claim: encode %s: %w", kind, err)
	}
	line := append([]byte(`{"type":"`+kind+`",`), body[1:]...)
	line = append(line, '\n')
	if _, err := l.f.Write(line); err != nil {
		return fmt.Errorf("claim: append %s: %w", kind, err)
	}
	if err := l.f.Sync(); err != nil {
		return fmt.Errorf("claim: sync log: %w", err)
	}
	return nil
}

func newID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("claim: new id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

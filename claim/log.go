package claim

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"slices"
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
	gone   map[string]bool // ids a confirmed claim supersedes or retracts
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
		gone:   map[string]bool{},
	}
	if err := l.replay(); err != nil {
		f.Close()
		return nil, fmt.Errorf("claim: read %s: %w", path, err)
	}
	return l, nil
}

// Close closes the file.
func (l *nsLog) Close() error { return l.f.Close() }

// entry is one line of the log. A line holds a claim or a decision; decoding
// both shapes into one struct reads each line once.
type entry struct {
	Type string `json:"type"`
	Claim
	ClaimID string  `json:"claim_id"`
	Outcome outcome `json:"outcome"`
}

func (l *nsLog) replay() error {
	sc := bufio.NewScanner(l.f)
	sc.Buffer(nil, 1<<24)
	for n := 1; sc.Scan(); n++ {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var e entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			return fmt.Errorf("line %d: %w", n, err)
		}
		switch e.Type {
		case typeClaim:
			l.applyClaim(e.Claim)
		case typeDecision:
			l.applyDecision(decision{ClaimID: e.ClaimID, Outcome: e.Outcome, Author: e.Author, RecordedAt: e.RecordedAt})
		default:
			return fmt.Errorf("line %d: unknown type %q", n, e.Type)
		}
	}
	return sc.Err()
}

func (l *nsLog) applyClaim(c Claim) {
	l.order = append(l.order, c.ID)
	l.claims[c.ID] = c
	if c.Author.Kind == User {
		l.confirm(c)
	} else {
		l.state[c.ID] = pending
	}
}

func (l *nsLog) applyDecision(d decision) {
	if d.Outcome == confirmOutcome {
		l.confirm(l.claims[d.ClaimID])
	} else {
		l.state[d.ClaimID] = rejected
	}
}

// confirm is the fold, one claim at a time: a confirmed claim takes what it
// supersedes or retracts out of the live set for good, even when the claim
// itself is later superseded. So the order of confirmations does not matter.
func (l *nsLog) confirm(c Claim) {
	l.state[c.ID] = confirmed
	for _, target := range c.Supersedes {
		l.gone[target] = true
	}
	if c.Retracted != "" {
		l.gone[c.Retracted] = true
	}
}

// isLive tells if id is live: confirmed, not superseded or retracted, and no
// retraction itself. The caller holds mu.
func (l *nsLog) isLive(id string) bool {
	return l.state[id] == confirmed && !l.gone[id] && l.claims[id].Retracted == ""
}

// appendLive appends the live claims in log order to dst, each marked with ns.
func (l *nsLog) appendLive(dst []Claim, ns Namespace) []Claim {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.appendWhere(dst, ns, l.isLive)
}

// appendProposals appends the claims that wait for a decision, in log order.
func (l *nsLog) appendProposals(dst []Claim, ns Namespace) []Claim {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.appendWhere(dst, ns, func(id string) bool { return l.state[id] == pending })
}

// appendWhere counts first, so dst grows once. The caller holds mu.
func (l *nsLog) appendWhere(dst []Claim, ns Namespace, keep func(id string) bool) []Claim {
	n := 0
	for _, id := range l.order {
		if keep(id) {
			n++
		}
	}
	dst = slices.Grow(dst, n)
	for _, id := range l.order {
		if keep(id) {
			c := l.claims[id]
			c.Namespace = ns
			dst = append(dst, c)
		}
	}
	return dst
}

// Live returns the live claims in log order.
func (l *nsLog) Live() []Claim { return l.appendLive(nil, Namespace{}) }

// Proposals returns the claims that wait for a decision, in log order.
func (l *nsLog) Proposals() []Claim { return l.appendProposals(nil, Namespace{}) }

// hasLive tells if id is live, for a citation from another namespace.
func (l *nsLog) hasLive(id string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.isLive(id)
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
	if err := validClaim(c, l.isLive); err != nil {
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
func (l *nsLog) Decide(d decision) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if d.Author.Kind != User || d.Author.ID == "" {
		return invalid("only a user decides")
	}
	if d.Outcome != confirmOutcome && d.Outcome != rejectOutcome {
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

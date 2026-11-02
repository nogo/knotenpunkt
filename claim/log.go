package claim

import (
	"bufio"
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

const (
	typeClaim    = "claim"
	typeDecision = "decision"
)

// status is where an id stands in the fold. Only live and pending claims keep
// their body in memory; for the rest the status is all that is left.
type status uint8

const (
	pending    status = iota + 1
	live              // confirmed, and nothing confirmed supersedes or retracts it
	rejected          // a user rejected the proposal
	gone              // a confirmed claim superseded or retracted it
	retraction        // a confirmed retraction; never live itself
)

// nsLog is the storage of one namespace: an append-only JSONL file with one
// writer. The file is the history and the truth; memory holds only the working
// tree, folded from it. Reads load a snapshot and take no lock; writes take mu.
// Only one nsLog may have the file open.
type nsLog struct {
	f   *os.File
	now func() time.Time

	mu     sync.Mutex        // orders writes; guards f's appends and status
	status map[string]status // every id in the log

	snap atomic.Pointer[snapshot] // what readers see; replaced, never changed
}

// snapshot is the working tree of a namespace: the live claims and the
// proposals that wait, each in the order they got there. Readers share it
// without a lock, so a writer never changes a published snapshot: it builds
// and publishes a new one. A writer may append into spare capacity, because
// no published slice reaches past its own length; removing always copies.
type snapshot struct {
	live      []Claim
	proposals []Claim
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
		status: map[string]status{},
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

// replay folds the file into the working tree. A body leaves memory as soon
// as its claim is rejected or taken out, so even opening never holds the
// whole history at once.
func (l *nsLog) replay() error {
	type held struct {
		c    Claim
		rank int // when the claim became live or pending
	}
	tree := map[string]held{}
	rank := 0
	keep := func(c Claim) {
		rank++
		tree[c.ID] = held{c, rank}
	}
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
			c := e.Claim
			if c.Author.Kind != User {
				l.status[c.ID] = pending
				keep(c)
				continue
			}
			for _, id := range l.settle(c) {
				delete(tree, id)
			}
			if l.status[c.ID] == live {
				keep(c)
			}
		case typeDecision:
			h, ok := tree[e.ClaimID]
			if !ok || l.status[e.ClaimID] != pending {
				continue
			}
			delete(tree, e.ClaimID)
			if e.Outcome != confirmOutcome {
				l.status[e.ClaimID] = rejected
				continue
			}
			for _, id := range l.settle(h.c) {
				delete(tree, id)
			}
			if l.status[h.c.ID] == live {
				keep(h.c)
			}
		default:
			return fmt.Errorf("line %d: unknown type %q", n, e.Type)
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	ids := make([]string, 0, len(tree))
	nLive := 0
	for id := range tree {
		ids = append(ids, id)
		if l.status[id] == live {
			nLive++
		}
	}
	slices.SortFunc(ids, func(a, b string) int { return cmp.Compare(tree[a].rank, tree[b].rank) })
	s := snapshot{live: make([]Claim, 0, nLive), proposals: make([]Claim, 0, len(ids)-nLive)}
	for _, id := range ids {
		if l.status[id] == live {
			s.live = append(s.live, tree[id].c)
		} else {
			s.proposals = append(s.proposals, tree[id].c)
		}
	}
	l.snap.Store(&s)
	return nil
}

// settle is the fold for one confirmed claim: c becomes live, or a retraction,
// and what it supersedes or retracts is gone for good, even if c itself is
// superseded later. So the order of confirmations does not matter. It returns
// the ids that left the live set. The caller holds mu, or is replay.
func (l *nsLog) settle(c Claim) (out []string) {
	if c.Retracted != "" {
		l.status[c.ID] = retraction
	} else {
		l.status[c.ID] = live
	}
	for _, id := range append(slices.Clip(c.Supersedes), c.Retracted) {
		st, ok := l.status[id]
		if !ok {
			continue
		}
		if st == live {
			out = append(out, id)
		}
		l.status[id] = gone
	}
	return out
}

// without returns cs minus ids. With nothing to remove it returns cs itself;
// otherwise a new slice, so a published snapshot never changes.
func without(cs []Claim, ids ...string) []Claim {
	if len(ids) == 0 {
		return cs
	}
	out := make([]Claim, 0, len(cs))
	for _, c := range cs {
		if !slices.Contains(ids, c.ID) {
			out = append(out, c)
		}
	}
	return out
}

// Live returns the live claims in the order they became live. The slice is
// shared with readers: never change it.
func (l *nsLog) Live() []Claim { return l.snap.Load().live }

// Proposals returns the claims that wait for a decision, in log order. The
// slice is shared with readers: never change it.
func (l *nsLog) Proposals() []Claim { return l.snap.Load().proposals }

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
	} else if _, dup := l.status[c.ID]; dup {
		return Claim{}, invalid("id %q is already in the log", c.ID)
	}
	if err := validClaim(c, func(id string) bool { return l.status[id] == live }); err != nil {
		return Claim{}, err
	}
	c.Namespace = Namespace{}
	c.RecordedAt = l.now()
	if err := l.write(typeClaim, c); err != nil {
		return Claim{}, err
	}
	old := l.snap.Load()
	next := *old
	if c.Author.Kind == User {
		next.live = without(old.live, l.settle(c)...)
		if l.status[c.ID] == live {
			next.live = append(next.live, c)
		}
	} else {
		l.status[c.ID] = pending
		next.proposals = append(old.proposals, c)
	}
	l.snap.Store(&next)
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
	old := l.snap.Load()
	i := slices.IndexFunc(old.proposals, func(c Claim) bool { return c.ID == d.ClaimID })
	if l.status[d.ClaimID] != pending || i < 0 {
		return invalid("claim %q is not a pending proposal", d.ClaimID)
	}
	d.RecordedAt = l.now()
	if err := l.write(typeDecision, d); err != nil {
		return err
	}
	c := old.proposals[i]
	next := snapshot{live: old.live, proposals: without(old.proposals, c.ID)}
	if d.Outcome == rejectOutcome {
		l.status[c.ID] = rejected
	} else {
		next.live = without(old.live, l.settle(c)...)
		if l.status[c.ID] == live {
			next.live = append(next.live, c)
		}
	}
	l.snap.Store(&next)
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

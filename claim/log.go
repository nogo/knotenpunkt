package claim

import (
	"bufio"
	"bytes"
	"cmp"
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
	superseded        // a confirmed claim superseded it
	retracted         // a confirmed retraction took it back
	retraction        // a confirmed retraction; never live itself
)

// history names a status that is no longer live or pending, for HistoryError.
func (s status) history() History {
	switch s {
	case rejected:
		return Rejected
	case superseded:
		return Superseded
	case retracted:
		return Retracted
	}
	return Retraction
}

// nsLog is the storage of one namespace: an append-only JSONL file with one
// writer. The file is the history and the truth; memory holds only the working
// tree, folded from it. Reads load a snapshot and take no lock; writes take mu.
// Only one writable nsLog may have the file open; a read-only one may read
// beside it.
type nsLog struct {
	ns       Namespace // part of every claim's id
	f        *os.File
	readOnly bool // opened by a read-only Store: never writes, never truncates
	now      func() time.Time

	mu     sync.Mutex        // orders writes; guards f's appends, status and by
	status map[string]status // every id in the log
	by     map[string]string // superseded or retracted id -> the claim that did it

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

// openNSLog opens the log at path and folds what is there. A writable log is
// created if missing; a read-only one must exist.
func openNSLog(path string, ns Namespace, readOnly bool) (*nsLog, error) {
	flag := os.O_RDWR | os.O_CREATE | os.O_APPEND
	if readOnly {
		flag = os.O_RDONLY
	}
	f, err := os.OpenFile(path, flag, 0o600)
	if err != nil {
		return nil, fmt.Errorf("claim: open log: %w", err)
	}
	l := &nsLog{
		ns:       ns,
		f:        f,
		readOnly: readOnly,
		now:      func() time.Time { return time.Now().UTC() },
		status:   map[string]status{},
		by:       map[string]string{},
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
// whole history at once. A last line without its newline is unfinished: a
// write was cut off, or a writer is still writing it. Replay skips it; a
// writable log then cuts it off, so the next line starts clean.
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
	unfinished := 0 // bytes after the last newline
	sc.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		if atEOF && len(data) > 0 && bytes.IndexByte(data, '\n') < 0 {
			unfinished = len(data)
			return len(data), nil, nil
		}
		return bufio.ScanLines(data, atEOF)
	})
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
	if unfinished > 0 && !l.readOnly {
		if err := l.cutOff(int64(unfinished)); err != nil {
			return err
		}
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

// cutOff drops the last n bytes of the file: an unfinished line.
func (l *nsLog) cutOff(n int64) error {
	info, err := l.f.Stat()
	if err != nil {
		return err
	}
	if err := l.f.Truncate(info.Size() - n); err != nil {
		return fmt.Errorf("cut off an unfinished line: %w", err)
	}
	return l.f.Sync()
}

// settle is the fold for one confirmed claim: c becomes live, or a retraction,
// and what it supersedes or retracts leaves the live set for good, even if c
// itself is superseded later. So the order of confirmations does not matter.
// It returns the ids that left the live set. The caller holds mu, or is replay.
func (l *nsLog) settle(c Claim) (out []string) {
	if c.Retracted != "" {
		l.status[c.ID] = retraction
		out = l.takeOut(c.Retracted, retracted, c.ID, out)
	} else {
		l.status[c.ID] = live
	}
	for _, id := range c.Supersedes {
		out = l.takeOut(id, superseded, c.ID, out)
	}
	return out
}

// takeOut marks id as superseded or retracted by the claim by, and adds it to
// out if it was live. A claim keeps the first claim that took it out.
func (l *nsLog) takeOut(id string, why status, by string, out []string) []string {
	st := l.status[id]
	if st != live && st != pending {
		return out
	}
	if st == live {
		out = append(out, id)
	}
	l.status[id] = why
	l.by[id] = by
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
// a Proposer is a proposal until a User decides it. The id is the hash of the
// content; RecordedAt is always set by the log. A claim already in the log is
// not written again: Append returns it as recorded while it is live or
// pending, and returns a HistoryError once it is history. check, if set, runs after the
// rules with mu held. It returns the claim as recorded.
func (l *nsLog) Append(c Claim, check func() error) (Claim, error) {
	id := contentID(l.ns, c)
	if c.ID != "" && c.ID != id {
		return Claim{}, invalid("id %q is not the hash of the claim's content", c.ID)
	}
	c.ID = id
	l.mu.Lock()
	defer l.mu.Unlock()
	if st, ok := l.status[id]; ok {
		return l.recorded(id, st)
	}
	if err := l.admit(c, check); err != nil {
		return Claim{}, err
	}
	if l.readOnly {
		return Claim{}, ErrReadOnly
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

// Check is why Append would refuse c, nil when Append would take it or treat it
// as a retry. It writes nothing.
func (l *nsLog) Check(c Claim, check func() error) error {
	id := contentID(l.ns, c)
	if c.ID != "" && c.ID != id {
		return invalid("id %q is not the hash of the claim's content", c.ID)
	}
	c.ID = id
	l.mu.Lock()
	defer l.mu.Unlock()
	if st, ok := l.status[id]; ok {
		_, err := l.recorded(id, st)
		return err
	}
	return l.admit(c, check)
}

// admit checks a claim that is not in the log yet: its rules, that a
// proposal repeats no live claim or proposal, and check, if set. A user's own
// claim is never a repeat: the person decides what they state twice. The
// caller holds mu.
func (l *nsLog) admit(c Claim, check func() error) error {
	if err := validClaim(c, func(id string) bool { return l.status[id] == live }); err != nil {
		return err
	}
	s := l.snap.Load()
	if c.Author.Kind == Proposer {
		if o, ok := duplicateOf(c, s.live); ok {
			return invalid("already a live claim (%s): supersede it to change it", o.ID)
		}
		if o, ok := duplicateOf(c, s.proposals); ok {
			return invalid("already proposed (%s)", o.ID)
		}
	}
	if check != nil {
		return check()
	}
	return nil
}

// Lookup finds id in the log: its body while it is live or pending, and where
// it stands.
func (l *nsLog) Lookup(id string) (Claim, Standing, bool) {
	l.mu.Lock() // status and the snapshot change together under mu
	defer l.mu.Unlock()
	st, ok := l.status[id]
	if !ok {
		return Claim{}, Standing{}, false
	}
	if st != live && st != pending {
		return Claim{}, Standing{History: st.history(), By: l.by[id]}, true
	}
	s := l.snap.Load()
	tree := s.proposals
	if st == live {
		tree = s.live
	}
	i := slices.IndexFunc(tree, func(c Claim) bool { return c.ID == id })
	return tree[i], Standing{Live: st == live, Pending: st == pending}, true
}

// recorded returns a claim that is already in the log, for a repeated Append.
// A repeat is a retry: while the claim is live or pending it is returned as
// recorded; once it is history, a HistoryError says what became of it. The
// caller holds mu.
func (l *nsLog) recorded(id string, st status) (Claim, error) {
	s := l.snap.Load()
	for _, tree := range [][]Claim{s.live, s.proposals} {
		if i := slices.IndexFunc(tree, func(c Claim) bool { return c.ID == id }); i >= 0 {
			return tree[i], nil
		}
	}
	return Claim{}, &HistoryError{ID: id, State: st.history(), By: l.by[id]}
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
	if l.readOnly {
		return ErrReadOnly
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

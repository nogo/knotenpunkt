package claim

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Namespace is where a claim lives: an org, and optionally one engagement in
// it. It is a confidentiality boundary. An engagement reads its own claims and
// its org's claims; an org reads only its own; nothing reads a sibling.
type Namespace struct {
	Org        string
	Engagement string
}

var slug = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ParseNamespace reads "org" or "org/engagement". On error it returns the
// zero Namespace, which every Store method refuses.
func ParseNamespace(s string) (Namespace, error) {
	org, eng, nested := strings.Cut(s, "/")
	ns := Namespace{Org: org, Engagement: eng}
	if nested && eng == "" {
		return Namespace{}, invalid("namespace %q ends with a slash", s)
	}
	if err := ns.valid(); err != nil {
		return Namespace{}, err
	}
	return ns, nil
}

// String is "org" or "org/engagement".
func (ns Namespace) String() string {
	if ns.Engagement == "" {
		return ns.Org
	}
	return ns.Org + "/" + ns.Engagement
}

func (ns Namespace) valid() error {
	if !slug.MatchString(ns.Org) {
		return invalid("namespace org %q is not lowercase letters, digits and dashes", ns.Org)
	}
	if ns.Engagement != "" && !slug.MatchString(ns.Engagement) {
		return invalid("namespace engagement %q is not lowercase letters, digits and dashes", ns.Engagement)
	}
	return nil
}

// readable is the namespaces ns reads, its own first.
func (ns Namespace) readable() []Namespace {
	if ns.Engagement == "" {
		return []Namespace{ns}
	}
	return []Namespace{ns, {Org: ns.Org}}
}

// CiteSystem is the Source.System of a claim that cites a claim in another
// namespace. Its Ref is "<namespace>#<id>"; Cite builds it.
const CiteSystem = "knotenpunkt"

// Cite is the source for a new claim that carries a fact across namespaces.
// The cited claim must be live when the new claim is appended.
func Cite(ns Namespace, id string) *Source {
	return &Source{System: CiteSystem, Ref: ns.String() + "#" + id}
}

// Store is the claim layer of one vault: every namespace in it. How claims are
// laid out on disk is the store's own business; callers name a namespace.
type Store struct {
	dir string

	mu   sync.Mutex // guards logs
	logs map[Namespace]*nsLog
}

// OpenStore opens the vault's claims in dir, creating dir if missing.
func OpenStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("claim: open store: %w", err)
	}
	return &Store{dir: dir, logs: map[Namespace]*nsLog{}}, nil
}

// Close closes every open namespace.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var first error
	for ns, l := range s.logs {
		if err := l.Close(); err != nil && first == nil {
			first = err
		}
		delete(s.logs, ns)
	}
	return first
}

// path keeps an org's own claims in "_org.jsonl", which no engagement slug can
// be, so an org and its engagements share one directory.
func (s *Store) path(ns Namespace) string {
	name := "_org"
	if ns.Engagement != "" {
		name = ns.Engagement
	}
	return filepath.Join(s.dir, ns.Org, name+".jsonl")
}

// log returns the namespace's log. With create false, a namespace that has no
// claims yet returns nil, so reading never creates files.
func (s *Store) log(ns Namespace, create bool) (*nsLog, error) {
	if err := ns.valid(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if l, ok := s.logs[ns]; ok {
		return l, nil
	}
	path := s.path(ns)
	if _, err := os.Stat(path); os.IsNotExist(err) && !create {
		return nil, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("claim: open %s: %w", ns, err)
	}
	l, err := openNSLog(path)
	if err != nil {
		return nil, err
	}
	s.logs[ns] = l
	return l, nil
}

// Append checks c and writes it to ns. Supersedes, Retracted and Challenges
// name claims in ns itself. A claim whose source is a Cite must cite a live
// claim in this store. A claim whose Namespace is set must name ns. It
// returns the claim as recorded.
func (s *Store) Append(ns Namespace, c Claim) (Claim, error) {
	if c.Namespace != (Namespace{}) && c.Namespace != ns {
		return Claim{}, invalid("claim is from %s, not %s", c.Namespace, ns)
	}
	if c.Source != nil && c.Source.System == CiteSystem {
		if err := s.checkCite(c.Source.Ref); err != nil {
			return Claim{}, err
		}
	}
	l, err := s.log(ns, true)
	if err != nil {
		return Claim{}, err
	}
	got, err := l.Append(c)
	if err != nil {
		return Claim{}, err
	}
	got.Namespace = ns
	return got, nil
}

func (s *Store) checkCite(ref string) error {
	name, id, ok := strings.Cut(ref, "#")
	if !ok || id == "" {
		return invalid("cite %q is not <namespace>#<id>", ref)
	}
	ns, err := ParseNamespace(name)
	if err != nil {
		return err
	}
	l, err := s.log(ns, false)
	if err != nil {
		return err
	}
	if l == nil || !l.hasLive(id) {
		return invalid("cite %q names no live claim", ref)
	}
	return nil
}

// Confirm makes the proposal id in ns live. user names the person who
// confirms; only a person confirms, never a model or a job.
func (s *Store) Confirm(ns Namespace, id, user string) error {
	return s.decide(ns, decision{ClaimID: id, Outcome: confirmOutcome, Author: Author{Kind: User, ID: user}})
}

// Reject settles the proposal id in ns for good: it never becomes live.
func (s *Store) Reject(ns Namespace, id, user string) error {
	return s.decide(ns, decision{ClaimID: id, Outcome: rejectOutcome, Author: Author{Kind: User, ID: user}})
}

func (s *Store) decide(ns Namespace, d decision) error {
	l, err := s.log(ns, false)
	if err != nil {
		return err
	}
	if l == nil {
		return invalid("claim %q is not a pending proposal", d.ClaimID)
	}
	return l.Decide(d)
}

// Live returns the live claims ns reads: its own, then its org's. Each claim
// carries the namespace it lives in.
func (s *Store) Live(ns Namespace) ([]Claim, error) {
	var out []Claim
	for _, r := range ns.readable() {
		l, err := s.log(r, false)
		if err != nil {
			return nil, err
		}
		if l != nil {
			out = l.appendLive(out, r)
		}
	}
	return out, nil
}

// Proposals returns the claims in ns itself that wait for a decision.
func (s *Store) Proposals(ns Namespace) ([]Claim, error) {
	l, err := s.log(ns, false)
	if err != nil || l == nil {
		return nil, err
	}
	return l.appendProposals(nil, ns), nil
}

// Recall ranks the live claims ns reads against text: best first, at most
// limit, leaving out the ids in exclude.
func (s *Store) Recall(ns Namespace, text string, limit int, exclude ...string) ([]Match, error) {
	live, err := s.Live(ns)
	if err != nil {
		return nil, err
	}
	return recall(live, text, limit, exclude...), nil
}

// RecallClaim recalls candidates for c in ns, leaving out what c supersedes or
// challenges. c need not be appended.
func (s *Store) RecallClaim(ns Namespace, c Claim, limit int) ([]Match, error) {
	exclude := append(append([]string(nil), c.Supersedes...), c.Challenges...)
	return s.Recall(ns, c.Text, limit, exclude...)
}

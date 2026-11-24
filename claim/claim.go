// Package claim is the claim layer other systems embed as their only claim store.
// It holds the claim record, the rules checked when a claim is written, and an
// append-only log whose fold defines which claims are live. Reach for it to
// record, propose, confirm and correct claims. It never reads a source: the
// source is opaque and the embedding system resolves it.
package claim

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// ErrInvalid marks a claim, confirmation or rejection that breaks a rule.
// Nothing is appended.
var ErrInvalid = errors.New("claim: invalid")

// ErrHistory marks an Append of a claim that is already in the log and no
// longer live or pending. The error is a *HistoryError; nothing is appended.
var ErrHistory = errors.New("claim: already in the log as history")

// ErrReadOnly marks a write to a Store opened with OpenStoreReadOnly.
var ErrReadOnly = errors.New("claim: store is read-only")

// Standing is where a claim stands in the fold: live, pending, or what became
// of it. By names the claim that superseded or retracted it.
type Standing struct {
	Live    bool
	Pending bool
	History History // empty while it is live or pending
	By      string
}

// History is what became of a claim that is no longer live or pending.
type History string

const (
	Rejected   History = "rejected"   // a user rejected the proposal
	Superseded History = "superseded" // a confirmed claim replaced it
	Retracted  History = "retracted"  // a confirmed retraction took it back
	Retraction History = "retraction" // it is a confirmed retraction, never live itself
)

// HistoryError says that Append met a claim it already holds as history.
// Appending the same claim again is a retry, so it never revives a claim. By
// names the claim that superseded or retracted it; it is empty otherwise.
// A Superseded claim whose source still states it points at source drift.
type HistoryError struct {
	ID    string
	State History
	By    string
}

func (e *HistoryError) Error() string {
	if e.By == "" {
		return fmt.Sprintf("claim: %s is already in the log, %s", e.ID, e.State)
	}
	return fmt.Sprintf("claim: %s is already in the log, %s by %s", e.ID, e.State, e.By)
}

// Unwrap lets callers branch with errors.Is(err, ErrHistory).
func (e *HistoryError) Unwrap() error { return ErrHistory }

// Trust says what may be done with a claim.
type Trust string

const (
	Committed  Trust = "committed"
	Sourced    Trust = "sourced"
	Derived    Trust = "derived"
	Position   Trust = "position"
	Unverified Trust = "unverified"
	Contested  Trust = "contested"
)

// Trusts are the six trust values, in the order the claim system lists them.
// Never change the slice: it is shared.
var Trusts = []Trust{Committed, Sourced, Derived, Position, Unverified, Contested}

var knownTrust = map[Trust]bool{
	Committed: true, Sourced: true, Derived: true,
	Position: true, Unverified: true, Contested: true,
}

// AuthorKind tells who wrote a claim or a decision.
type AuthorKind string

const (
	User     AuthorKind = "user"
	Proposer AuthorKind = "proposer"
)

// Author is who wrote a claim. A User's claim is live at once; a Proposer's
// claim waits until a user confirms it.
type Author struct {
	Kind AuthorKind `json:"kind"`
	ID   string     `json:"id"`
}

// Source points at where a claim came from. knotenpunkt stores it and never
// resolves it: System names the embedding system, Ref its reference, and
// Version what the source was when the claim was recorded.
type Source struct {
	System  string `json:"system"`
	Ref     string `json:"ref"`
	Version string `json:"version"`
}

// Claim is one fact with its trust, source and the date it became true.
// Retracted names the claim this one retracts; a retraction is never live itself.
// Namespace is where the claim lives; a Store sets it on every claim it returns
// and never writes it. ID is the hash of the claim's content (contentID): leave
// it empty, or it must match. RecordedAt is always set by the store.
type Claim struct {
	ID         string    `json:"id"`
	Namespace  Namespace `json:"-"`
	Text       string    `json:"text"`
	Trust      Trust     `json:"trust"`
	AsOf       string    `json:"asof"`
	Source     *Source   `json:"source,omitempty"`
	Party      string    `json:"party,omitempty"`
	Quote      string    `json:"quote,omitempty"`
	Attributed bool      `json:"attributed,omitempty"`
	Supersedes []string  `json:"supersedes,omitempty"`
	Challenges []string  `json:"challenges,omitempty"`
	Retracted  string    `json:"retracted,omitempty"`
	Author     Author    `json:"author"`
	RecordedAt time.Time `json:"recorded_at"`
}

// outcome is what a decision does to a proposal.
type outcome string

const (
	confirmOutcome outcome = "confirm"
	rejectOutcome  outcome = "reject"
)

// decision is the log entry that settles a proposal. Only a User decides;
// Store.Confirm and Store.Reject write it.
type decision struct {
	ClaimID    string    `json:"claim_id"`
	Outcome    outcome   `json:"outcome"`
	Author     Author    `json:"author"`
	RecordedAt time.Time `json:"recorded_at"`
}

// contentID is a claim's id: the lowercase hex SHA-256 of its namespace and
// everything the caller sent. ID and RecordedAt are left out, so the same
// claim sent twice gets the same id, and two nodes never make the same id for
// different claims. An empty list hashes like a missing one.
func contentID(ns Namespace, c Claim) string {
	b, err := json.Marshal(struct {
		Namespace  string   `json:"namespace"`
		Text       string   `json:"text"`
		Trust      Trust    `json:"trust"`
		AsOf       string   `json:"asof"`
		Source     *Source  `json:"source"`
		Party      string   `json:"party"`
		Quote      string   `json:"quote"`
		Attributed bool     `json:"attributed"`
		Supersedes []string `json:"supersedes"`
		Challenges []string `json:"challenges"`
		Retracted  string   `json:"retracted"`
		Author     Author   `json:"author"`
	}{ns.String(), c.Text, c.Trust, c.AsOf, c.Source, c.Party, c.Quote, c.Attributed,
		nilIfEmpty(c.Supersedes), nilIfEmpty(c.Challenges), c.Retracted, c.Author})
	if err != nil {
		panic(err) // impossible: strings, bools and string slices always encode
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// IDOf is the id c gets in ns: the hash Append records it under. Reach for it
// to look a claim up before appending it.
func IDOf(ns Namespace, c Claim) string { return contentID(ns, c) }

func nilIfEmpty(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	return ids
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

const minTextLen = 10

// validClaim checks the rules of a claim against the live claims.
func validClaim(c Claim, isLive func(id string) bool) error {
	if utf8.RuneCountInString(c.Text) < minTextLen {
		return invalid("text is shorter than %d characters", minTextLen)
	}
	if !knownTrust[c.Trust] {
		return invalid("unknown trust %q", c.Trust)
	}
	if t, err := time.Parse("2006-01-02", c.AsOf); err != nil || t.Format("2006-01-02") != c.AsOf {
		return invalid("asof %q is not an ISO date", c.AsOf)
	}
	if c.Author.Kind != User && c.Author.Kind != Proposer {
		return invalid("unknown author kind %q", c.Author.Kind)
	}
	if c.Author.ID == "" {
		return invalid("author has no id")
	}
	if c.Trust == Committed && strings.TrimSpace(c.Party) == "" {
		return invalid("committed claim has no party")
	}
	if c.Trust == Contested && len(c.Challenges) == 0 {
		return invalid("contested claim challenges nothing")
	}
	if c.Source == nil {
		if c.Trust != Unverified && c.Trust != Position {
			return invalid("%s claim has no source", c.Trust)
		}
	} else if c.Source.System == "" || c.Source.Ref == "" {
		return invalid("source needs a system and a ref")
	}
	if err := validRefs("supersedes", c.Supersedes, isLive); err != nil {
		return err
	}
	if err := validRefs("challenges", c.Challenges, isLive); err != nil {
		return err
	}
	if c.Retracted != "" && !isLive(c.Retracted) {
		return invalid("retracted claim %q is not live", c.Retracted)
	}
	return nil
}

func validRefs(field string, ids []string, isLive func(id string) bool) error {
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		switch {
		case id == "":
			return invalid("%s has an empty reference", field)
		case seen[id]:
			return invalid("%s repeats %q", field, id)
		case !isLive(id):
			return invalid("%s names %q, which is not live", field, id)
		}
		seen[id] = true
	}
	return nil
}

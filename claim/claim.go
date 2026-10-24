// Package claim is the claim layer other systems embed as their only claim store.
// It holds the claim record, the rules checked when a claim is written, and an
// append-only log whose fold defines which claims are live. Reach for it to
// record, propose, confirm and correct claims. It never reads a source: the
// source is opaque and the embedding system resolves it.
package claim

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalid marks a claim or decision that breaks a rule. Nothing is appended.
var ErrInvalid = errors.New("claim: invalid")

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

// Author is who wrote an entry. Only a User decides.
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
type Claim struct {
	ID         string    `json:"id"`
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

// Outcome is what a decision does to a proposal.
type Outcome string

const (
	Confirm Outcome = "confirm"
	Reject  Outcome = "reject"
)

// Decision settles a proposal. Only a User may decide.
type Decision struct {
	ClaimID    string    `json:"claim_id"`
	Outcome    Outcome   `json:"outcome"`
	Author     Author    `json:"author"`
	RecordedAt time.Time `json:"recorded_at"`
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

const minTextLen = 10

// validClaim checks the rules of a claim against the live claims.
func validClaim(c Claim, isLive func(id string) bool) error {
	if len([]rune(c.Text)) < minTextLen {
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
	if c.Trust == Committed && c.Party == "" {
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

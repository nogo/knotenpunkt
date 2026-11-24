package claim

import (
	"cmp"
	"slices"
)

// Reason says why a claim is due.
type Reason string

const (
	DueContested  Reason = "contested"  // it challenges a live claim, or a live claim challenges it
	DueMoved      Reason = "moved"      // the owning system says its source moved
	DueUnverified Reason = "unverified" // unverified since before the caller's date
	DueWaiting    Reason = "waiting"    // a proposal that waits for a decision
)

// dueOrder is the order of the reasons in a Due list: what conflicts first,
// then what drifted, then what was never checked, then what waits.
var dueOrder = []Reason{DueContested, DueMoved, DueUnverified, DueWaiting}

// Due is one claim that needs a look, and why. With names the other claim of
// a contest.
type Due struct {
	Claim  Claim
	Reason Reason
	With   string
}

// DueQuery says what the caller knows and Due can't: which sources moved,
// and from what date an unverified claim is old.
type DueQuery struct {
	// Moved reports whether a source moved since a claim was recorded from
	// it. The source is opaque here, so only its owning system can say. Nil
	// means no source moved.
	Moved func(Source) bool
	// UnverifiedBefore is an ISO date: an unverified claim whose asof is
	// earlier is due. Empty leaves unverified claims out.
	UnverifiedBefore string
}

// Due lists the claims of ns that need a look now, with no question asked:
// live claims in a contest, live claims whose source moved, old unverified
// live claims, and the proposals that wait. It reads ns and its org, as Live
// does; proposals come from ns itself. A claim comes once, under its first
// reason in that order. Inside a reason the newest asof comes first, then the
// claim that became live or was proposed last. It needs no model and gives
// the same answer for the same log.
func (s *Store) Due(ns Namespace, q DueQuery) ([]Due, error) {
	live, err := s.Live(ns)
	if err != nil {
		return nil, err
	}
	proposals, err := s.Proposals(ns)
	if err != nil {
		return nil, err
	}
	var all []Claim
	for c := range live {
		all = append(all, c)
	}
	isLive := make(map[string]bool, len(all))
	for _, c := range all {
		isLive[c.ID] = true
	}
	with := map[string]string{} // a claim in a contest -> the other side
	for _, c := range all {
		for _, id := range c.Challenges {
			if isLive[id] {
				with[c.ID] = cmp.Or(with[c.ID], id)
				with[id] = cmp.Or(with[id], c.ID)
			}
		}
		if c.Trust == Contested && with[c.ID] == "" && len(c.Challenges) > 0 {
			with[c.ID] = c.Challenges[0]
		}
	}
	var out []Due
	for i := len(all) - 1; i >= 0; i-- { // newest first, so the sort keeps it inside a reason
		c := all[i]
		switch {
		case with[c.ID] != "":
			out = append(out, Due{Claim: c, Reason: DueContested, With: with[c.ID]})
		case q.Moved != nil && c.Source != nil && q.Moved(*c.Source):
			out = append(out, Due{Claim: c, Reason: DueMoved})
		case q.UnverifiedBefore != "" && c.Trust == Unverified && c.AsOf < q.UnverifiedBefore:
			out = append(out, Due{Claim: c, Reason: DueUnverified})
		}
	}
	waiting := slices.Collect(proposals)
	for _, c := range slices.Backward(waiting) {
		out = append(out, Due{Claim: c, Reason: DueWaiting})
	}
	slices.SortStableFunc(out, func(a, b Due) int {
		if d := slices.Index(dueOrder, a.Reason) - slices.Index(dueOrder, b.Reason); d != 0 {
			return d
		}
		return cmp.Compare(b.Claim.AsOf, a.Claim.AsOf)
	})
	return out, nil
}

package claim

import (
	"testing"
)

func TestDueListsWhatNeedsALookInOrder(t *testing.T) {
	s, _ := openStore(t)
	moved := &Source{System: "sidenote", Ref: "blk-moved", Version: "v1"}
	still := &Source{System: "sidenote", Ref: "blk-still", Version: "v1"}
	mk := func(text string, trust Trust, asof string, src *Source, a Author) Claim {
		return put(t, s, acmeMig, Claim{Text: text, Trust: trust, AsOf: asof, Source: src, Author: a})
	}
	target := mk("ACME moves OrderApp in March", Sourced, "2026-03-01", still, jane)
	contest := put(t, s, acmeMig, Claim{Text: "ACME moves OrderApp in April", Trust: Contested, AsOf: "2026-04-01",
		Source: still, Challenges: []string{target.ID}, Author: jane})
	drifted := mk("ACME runs OrderApp on AWS", Sourced, "2026-05-01", moved, jane)
	oldGuess := mk("ACME pays for the OrderApp move", Unverified, "2026-01-10", nil, jane)
	mk("ACME plans OrderApp review next year", Unverified, "2026-06-10", nil, jane) // recent: not due
	mk("ACME owns the OrderApp repository", Sourced, "2026-02-01", still, jane)     // fine: not due
	waiting := mk("ACME stops OrderApp in May", Unverified, "2026-01-01", nil, bot)

	due, err := s.Due(acmeMig, DueQuery{
		Moved:            func(src Source) bool { return src.Ref == "blk-moved" },
		UnverifiedBefore: "2026-06-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		id     string
		reason Reason
		with   string
	}{
		{contest.ID, DueContested, target.ID}, // newer asof first
		{target.ID, DueContested, contest.ID},
		{drifted.ID, DueMoved, ""},
		{oldGuess.ID, DueUnverified, ""},
		{waiting.ID, DueWaiting, ""},
	}
	if len(due) != len(want) {
		t.Fatalf("due = %d claims, want %d: %+v", len(due), len(want), due)
	}
	for i, w := range want {
		if due[i].Claim.ID != w.id || due[i].Reason != w.reason || due[i].With != w.with {
			t.Errorf("due[%d] = %s %q with %q, want %s %q with %q", i, due[i].Claim.Text, due[i].Reason, due[i].With, w.id[:8], w.reason, w.with)
		}
	}
}

func TestDueWithoutWhatTheCallerKnowsLeavesThoseOut(t *testing.T) {
	s, _ := openStore(t)
	put(t, s, acmeMig, Claim{Text: "ACME pays for the OrderApp move", Trust: Unverified, AsOf: "2026-01-10", Author: jane})
	put(t, s, acmeMig, sourced("ACME runs OrderApp on AWS", jane))
	due, err := s.Due(acmeMig, DueQuery{})
	if err != nil || len(due) != 0 {
		t.Fatalf("due = %+v, %v; want none", due, err)
	}
	if due, err := s.Due(acmeAPI, DueQuery{UnverifiedBefore: "2027-01-01"}); err != nil || len(due) != 0 {
		t.Fatalf("an empty namespace: %+v, %v", due, err)
	}
}

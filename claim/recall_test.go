package claim

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestRecallRanksDistinctWordOverlap(t *testing.T) {
	l, _ := openLog(t)
	best := mustAppend(t, l, sourced("ACME runs OrderApp in Zürich", jane))
	next := mustAppend(t, l, sourced("ACME ACME ACME owns warehouses", jane))
	mustAppend(t, l, sourced("Jane Roe leads the budget review", jane))
	got := l.Recall("ACME, ORDERAPP! Zürich? ACME", 10)
	if len(got) != 2 || got[0].Claim.ID != best.ID || got[0].Score != 3 || got[1].Claim.ID != next.ID || got[1].Score != 1 {
		t.Fatalf("matches = %+v, want scores 3 and 1", got)
	}
	if limited := l.Recall("ACME OrderApp Zürich", 1); len(limited) != 1 || limited[0].Claim.ID != best.ID {
		t.Fatalf("limited = %+v, want best claim only", limited)
	}
}

func TestRecallOnlyReturnsLiveClaims(t *testing.T) {
	l, path := openLog(t)
	old := mustAppend(t, l, sourced("ACME runs OrderApp version three", jane))
	correction := sourced("ACME runs OrderApp version four", jane)
	correction.Supersedes = []string{old.ID}
	live := mustAppend(t, l, correction)
	mustAppend(t, l, sourced("ACME proposes OrderApp version five", bot))
	rejected := mustAppend(t, l, sourced("ACME rejects OrderApp version six", bot))
	if err := l.Decide(decision{ClaimID: rejected.ID, Outcome: rejectOutcome, Author: jane}); err != nil {
		t.Fatal(err)
	}
	retracted := mustAppend(t, l, sourced("ACME retracts OrderApp version seven", jane))
	r := sourced("ACME retracts the incorrect OrderApp claim", jane)
	r.Retracted = retracted.ID
	mustAppend(t, l, r)
	confirmed := mustAppend(t, l, sourced("ACME confirms OrderApp version eight", bot))
	if err := l.Decide(decision{ClaimID: confirmed.ID, Outcome: confirmOutcome, Author: jane}); err != nil {
		t.Fatal(err)
	}
	check := func(log *nsLog) {
		t.Helper()
		got := log.Recall("ACME OrderApp", 100)
		if len(got) != 2 || got[0].Score != 2 || got[1].Score != 2 {
			t.Fatalf("matches = %+v, want two live claims", got)
		}
		ids := map[string]bool{got[0].Claim.ID: true, got[1].Claim.ID: true}
		if !ids[live.ID] || !ids[confirmed.ID] {
			t.Fatalf("ids = %v, want correction and confirmed proposal", ids)
		}
	}
	check(l)
	reopened, err := openNSLog(path, acmeMig)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	check(reopened)
}

func TestRecallTiesUseDateThenID(t *testing.T) {
	l, _ := openLog(t)
	var got []string
	for i, date := range []string{"2026-01-01", "2026-03-01", "2026-03-01"} {
		c := sourced("ACME runs OrderApp in production", jane)
		c.AsOf = date
		c.Source = &Source{System: "sidenote", Ref: fmt.Sprintf("blk-%d", i), Version: "v1"}
		got = append(got, mustAppend(t, l, c).ID)
	}
	old, newer := got[0], []string{got[1], got[2]}
	slices.Sort(newer)
	var ids []string
	for _, match := range l.Recall("OrderApp", 10) {
		ids = append(ids, match.Claim.ID)
	}
	if want := append(newer, old); !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}

func TestRecallIgnoresStopWordsAndEmptyInputs(t *testing.T) {
	l, _ := openLog(t)
	mustAppend(t, l, sourced("Die ACME ist in der Stadt und auf dem Land", jane))
	mustAppend(t, l, sourced("The warehouse is in the city and on the river", jane))
	for _, text := range []string{"", " \t\n", "?!", "die ist in der und auf dem", "the is in and on", "unrelated"} {
		if got := l.Recall(text, 10); len(got) != 0 {
			t.Errorf("Recall(%q) = %+v, want no matches", text, got)
		}
	}
	for _, limit := range []int{0, -1} {
		if got := l.Recall("ACME", limit); len(got) != 0 {
			t.Errorf("limit %d: %+v, want no matches", limit, got)
		}
	}
	if got := l.Recall("DIE ACME ist auf dem Land", 10); len(got) != 1 || got[0].Score != 2 {
		t.Fatalf("German overlap = %+v, want score 2", got)
	}
}

func TestRecallClaimExcludesLinkedClaims(t *testing.T) {
	l, _ := openLog(t)
	replaced := mustAppend(t, l, sourced("ACME runs OrderApp version three", jane))
	challenged := mustAppend(t, l, sourced("ACME runs OrderApp version four", jane))
	other := mustAppend(t, l, sourced("ACME runs OrderApp version five", jane))
	proposal := sourced("ACME runs OrderApp version six", bot)
	proposal.Supersedes = []string{replaced.ID}
	proposal.Challenges = []string{challenged.ID}
	before := proposal
	got := l.RecallClaim(proposal, 10)
	if len(got) != 1 || got[0].Claim.ID != other.ID {
		t.Fatalf("matches = %+v, want only unlinked claim", got)
	}
	if !reflect.DeepEqual(proposal, before) {
		t.Fatal("recall changed the input claim")
	}
}

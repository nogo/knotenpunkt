package claim

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var (
	jane  = Author{Kind: User, ID: "jane"}
	bot   = Author{Kind: Proposer, ID: "intake"}
	block = &Source{System: "sidenote", Ref: "blk-1", Version: "v1"}
)

func openLog(t *testing.T) (*nsLog, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "claims.jsonl")
	l, err := openNSLog(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	return l, path
}

func sourced(text string, a Author) Claim {
	return Claim{Text: text, Trust: Sourced, AsOf: "2026-01-15", Source: block, Author: a}
}

func mustAppend(t *testing.T, l *nsLog, c Claim) Claim {
	t.Helper()
	got, err := l.Append(c)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func liveIDs(l *nsLog) []string {
	var ids []string
	for _, c := range l.Live() {
		ids = append(ids, c.ID)
	}
	return ids
}

func TestAppendRefusesBrokenRules(t *testing.T) {
	l, path := openLog(t)
	base := mustAppend(t, l, sourced("ACME runs OrderApp in production", jane))

	tests := map[string]func(c *Claim){
		"short text":          func(c *Claim) { c.Text = "too short" },
		"unknown trust":       func(c *Claim) { c.Trust = "certain" },
		"asof with a time":    func(c *Claim) { c.AsOf = "2026-01-15T10:00:00Z" },
		"asof not a date":     func(c *Claim) { c.AsOf = "2026-13-01" },
		"asof unpadded":       func(c *Claim) { c.AsOf = "2026-1-5" },
		"committed no party":  func(c *Claim) { c.Trust = Committed },
		"contested no link":   func(c *Claim) { c.Trust = Contested },
		"sourced no source":   func(c *Claim) { c.Source = nil },
		"source no ref":       func(c *Claim) { c.Source = &Source{System: "sidenote"} },
		"unknown author kind": func(c *Claim) { c.Author.Kind = "robot" },
		"supersedes unknown":  func(c *Claim) { c.Supersedes = []string{"nope"} },
		"supersedes empty":    func(c *Claim) { c.Supersedes = []string{""} },
		"supersedes repeated": func(c *Claim) { c.Supersedes = []string{base.ID, base.ID} },
		"challenges unknown":  func(c *Claim) { c.Challenges = []string{"nope"} },
		"challenges repeated": func(c *Claim) { c.Challenges = []string{base.ID, base.ID} },
		"retracts unknown":    func(c *Claim) { c.Retracted = "nope" },
		"duplicate id":        func(c *Claim) { c.ID = base.ID },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			c := sourced("Jane Roe owns the OrderApp budget", jane)
			mutate(&c)
			if _, err := l.Append(c); !errors.Is(err, ErrInvalid) {
				t.Fatalf("got %v, want ErrInvalid", err)
			}
		})
	}
	if got := len(l.Live()); got != 1 {
		t.Fatalf("live = %d, want 1", got)
	}
	reopened, err := openNSLog(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if got := len(reopened.Live()); got != 1 {
		t.Fatalf("a refused claim reached the file: live = %d", got)
	}
}

func TestAppendAcceptsWhatTheRulesAllow(t *testing.T) {
	l, _ := openLog(t)
	base := mustAppend(t, l, sourced("ACME runs OrderApp in production", jane))
	for name, c := range map[string]Claim{
		"unverified without source": {Text: "ACME may move to a new vendor", Trust: Unverified, AsOf: "2026-02-01", Author: jane},
		"position without source":   {Text: "We prefer OrderApp over the old tool", Trust: Position, AsOf: "2026-02-01", Author: jane},
		"committed with party":      {Text: "Jane Roe promised delivery by June", Trust: Committed, AsOf: "2026-02-01", Party: "Jane Roe", Source: block, Author: jane},
		"contested with challenge":  {Text: "OrderApp does not run at ACME", Trust: Contested, AsOf: "2026-02-01", Source: block, Challenges: []string{base.ID}, Author: jane},
	} {
		if _, err := l.Append(c); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestProposalIsNotLiveUntilConfirmed(t *testing.T) {
	l, _ := openLog(t)
	p := mustAppend(t, l, sourced("ACME runs OrderApp in production", bot))
	if len(l.Live()) != 0 || len(l.Proposals()) != 1 {
		t.Fatalf("live=%d proposals=%d, want 0 and 1", len(l.Live()), len(l.Proposals()))
	}
	if err := l.Decide(decision{ClaimID: p.ID, Outcome: confirmOutcome, Author: jane}); err != nil {
		t.Fatal(err)
	}
	if got := liveIDs(l); len(got) != 1 || got[0] != p.ID {
		t.Fatalf("live = %v, want [%s]", got, p.ID)
	}
	if len(l.Proposals()) != 0 {
		t.Fatal("a confirmed claim is still a proposal")
	}
}

func TestUserClaimIsLiveAtOnce(t *testing.T) {
	l, _ := openLog(t)
	c := mustAppend(t, l, sourced("ACME runs OrderApp in production", jane))
	if got := liveIDs(l); len(got) != 1 || got[0] != c.ID {
		t.Fatalf("live = %v", got)
	}
}

func TestRejectedProposalNeverBecomesLive(t *testing.T) {
	l, path := openLog(t)
	p := mustAppend(t, l, sourced("ACME runs OrderApp in production", bot))
	if err := l.Decide(decision{ClaimID: p.ID, Outcome: rejectOutcome, Author: jane}); err != nil {
		t.Fatal(err)
	}
	if err := l.Decide(decision{ClaimID: p.ID, Outcome: confirmOutcome, Author: jane}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("confirming a rejected proposal: got %v, want ErrInvalid", err)
	}
	if len(l.Live()) != 0 {
		t.Fatal("rejected proposal is live")
	}
	re, err := openNSLog(path)
	if err != nil {
		t.Fatal(err)
	}
	defer re.Close()
	if len(re.Live()) != 0 {
		t.Fatal("rejected proposal is live after reopen")
	}
}

func TestDecisionByNonUserIsRefused(t *testing.T) {
	l, _ := openLog(t)
	p := mustAppend(t, l, sourced("ACME runs OrderApp in production", bot))
	for _, a := range []Author{bot, {Kind: User}, {}} {
		if err := l.Decide(decision{ClaimID: p.ID, Outcome: confirmOutcome, Author: a}); !errors.Is(err, ErrInvalid) {
			t.Errorf("author %+v: got %v, want ErrInvalid", a, err)
		}
	}
	if len(l.Live()) != 0 || len(l.Proposals()) != 1 {
		t.Fatal("a refused decision changed the log")
	}
}

func TestDecisionNeedsAPendingProposal(t *testing.T) {
	l, _ := openLog(t)
	c := mustAppend(t, l, sourced("ACME runs OrderApp in production", jane))
	for _, d := range []decision{
		{ClaimID: "nope", Outcome: confirmOutcome, Author: jane},
		{ClaimID: c.ID, Outcome: confirmOutcome, Author: jane},
		{ClaimID: c.ID, Outcome: "maybe", Author: jane},
	} {
		if err := l.Decide(d); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v: got %v, want ErrInvalid", d, err)
		}
	}
}

func TestCorrectionSupersedesTheOldClaim(t *testing.T) {
	l, path := openLog(t)
	old := mustAppend(t, l, sourced("ACME runs OrderApp version 3", jane))
	fix := sourced("ACME runs OrderApp version 4", bot)
	fix.Supersedes = []string{old.ID}
	p := mustAppend(t, l, fix)
	if got := liveIDs(l); len(got) != 1 || got[0] != old.ID {
		t.Fatalf("before confirm live = %v, want the old claim", got)
	}
	if err := l.Decide(decision{ClaimID: p.ID, Outcome: confirmOutcome, Author: jane}); err != nil {
		t.Fatal(err)
	}
	if got := liveIDs(l); len(got) != 1 || got[0] != p.ID {
		t.Fatalf("live = %v, want only the correction", got)
	}
	// The old claim stays in the log but cannot be superseded again.
	again := sourced("ACME runs OrderApp version 5", jane)
	again.Supersedes = []string{old.ID}
	if _, err := l.Append(again); !errors.Is(err, ErrInvalid) {
		t.Fatalf("superseding a superseded claim: got %v", err)
	}
	raw := readFile(t, path)
	if !strings.Contains(raw, old.Text) {
		t.Fatal("old claim left the log")
	}
}

func TestRejectedCorrectionLeavesTheOldClaimLive(t *testing.T) {
	l, _ := openLog(t)
	old := mustAppend(t, l, sourced("ACME runs OrderApp version 3", jane))
	fix := sourced("ACME runs OrderApp version 4", bot)
	fix.Supersedes = []string{old.ID}
	p := mustAppend(t, l, fix)
	if err := l.Decide(decision{ClaimID: p.ID, Outcome: rejectOutcome, Author: jane}); err != nil {
		t.Fatal(err)
	}
	if got := liveIDs(l); len(got) != 1 || got[0] != old.ID {
		t.Fatalf("live = %v, want the old claim", got)
	}
}

func TestRetractionLeavesNothingInItsPlace(t *testing.T) {
	l, _ := openLog(t)
	old := mustAppend(t, l, sourced("ACME runs OrderApp in production", jane))
	r := Claim{Text: "Retracting: the ACME claim was wrong", Trust: Unverified, AsOf: "2026-03-01", Retracted: old.ID, Author: jane}
	mustAppend(t, l, r)
	if got := liveIDs(l); len(got) != 0 {
		t.Fatalf("live = %v, want none", got)
	}
}

func TestChallengeKeepsBothClaimsLive(t *testing.T) {
	l, _ := openLog(t)
	old := mustAppend(t, l, sourced("ACME runs OrderApp in production", jane))
	c := Claim{Text: "OrderApp does not run at ACME", Trust: Contested, AsOf: "2026-03-01", Source: block, Challenges: []string{old.ID}, Author: jane}
	mustAppend(t, l, c)
	if got := len(l.Live()); got != 2 {
		t.Fatalf("live = %d, want 2", got)
	}
}

func TestReopenGivesTheSameLiveSet(t *testing.T) {
	l, path := openLog(t)
	a := mustAppend(t, l, sourced("ACME runs OrderApp version 3", jane))
	fix := sourced("ACME runs OrderApp version 4", bot)
	fix.Supersedes = []string{a.ID}
	p := mustAppend(t, l, fix)
	if err := l.Decide(decision{ClaimID: p.ID, Outcome: confirmOutcome, Author: jane}); err != nil {
		t.Fatal(err)
	}
	mustAppend(t, l, sourced("Jane Roe leads the OrderApp team", jane))
	mustAppend(t, l, sourced("ACME has a pending OrderApp review", bot))
	before := liveIDs(l)
	pending := len(l.Proposals())
	l.Close()

	re, err := openNSLog(path)
	if err != nil {
		t.Fatal(err)
	}
	defer re.Close()
	after := liveIDs(re)
	if strings.Join(before, ",") != strings.Join(after, ",") || len(after) != 2 {
		t.Fatalf("before %v, after %v", before, after)
	}
	if len(re.Proposals()) != pending {
		t.Fatalf("proposals = %d, want %d", len(re.Proposals()), pending)
	}
	// Appending after reopen keeps working.
	mustAppend(t, re, sourced("ACME renewed the OrderApp contract", jane))
}

func TestOpenRefusesACorruptLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "claims.jsonl")
	writeFile(t, path, "not json\n")
	if _, err := openNSLog(path); err == nil {
		t.Fatal("want an error")
	}
	writeFile(t, path, `{"type":"note"}`+"\n")
	if _, err := openNSLog(path); err == nil {
		t.Fatal("want an error for an unknown type")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func writeFile(t *testing.T, path, s string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(s), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSupersededCorrectionKeepsItsTargetGone(t *testing.T) {
	l, path := openLog(t)
	v3 := mustAppend(t, l, sourced("ACME runs OrderApp version 3", jane))
	v4 := sourced("ACME runs OrderApp version 4", jane)
	v4.Supersedes = []string{v3.ID}
	v4 = mustAppend(t, l, v4)
	v5 := sourced("ACME runs OrderApp version 5", jane)
	v5.Supersedes = []string{v4.ID}
	v5 = mustAppend(t, l, v5)
	if got := liveIDs(l); len(got) != 1 || got[0] != v5.ID {
		t.Fatalf("live = %v, want only version 5", got)
	}
	l.Close()
	re, err := openNSLog(path)
	if err != nil {
		t.Fatal(err)
	}
	defer re.Close()
	if got := liveIDs(re); len(got) != 1 || got[0] != v5.ID {
		t.Fatalf("after reopen live = %v, want only version 5", got)
	}
}

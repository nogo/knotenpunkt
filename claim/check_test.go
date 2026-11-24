package claim

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSameTextIgnoresCaseAndSpacing(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		same bool
	}{
		{"ACME moves OrderApp", "acme  moves\norderapp ", true},
		{"  Übergabe im März", "übergabe IM märz", true},
		{"ACME moves OrderApp", "ACME moves OrderApp.", false},
		{"ACME moves", "ACMEmoves", false},
		{"", "   ", true},
	} {
		if got := sameText(tc.a, tc.b); got != tc.same {
			t.Errorf("sameText(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.same)
		}
	}
}

func TestARepeatInOtherWordsOfTheSameSourceIsRefused(t *testing.T) {
	s, _ := openStore(t)
	first := put(t, s, acmeMig, sourced("ACME moves OrderApp in March", jane))
	again := sourced("acme moves  OrderApp in march", bot)
	if _, err := s.Append(acmeMig, again); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "already a live claim ("+first.ID+")") {
		t.Fatalf("repeat of a live claim: %v", err)
	}
	p := put(t, s, acmeMig, sourced("ACME stops OrderApp in April", bot))
	if _, err := s.Append(acmeMig, sourced("ACME stops OrderApp in April", Author{Kind: Proposer, ID: "other"})); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "already proposed ("+p.ID+")") {
		t.Fatalf("repeat of a proposal: %v", err)
	}
	other := sourced("ACME moves OrderApp in March", jane)
	other.Source = &Source{System: "sidenote", Ref: "blk-2"}
	put(t, s, acmeMig, other) // another source states it too
	change := sourced("ACME moves OrderApp in March", jane)
	change.Trust, change.Party, change.Supersedes = Committed, "ACME", []string{first.ID}
	put(t, s, acmeMig, change) // a change of trust keeps the text
}

func TestCheckSaysWhatAppendWouldDoAndWritesNothing(t *testing.T) {
	s, dir := openStore(t)
	if err := s.Check(acmeMig, sourced("ACME moves OrderApp in March", bot)); err != nil {
		t.Fatalf("check in an empty store: %v", err)
	}
	if err := s.Check(acmeMig, sourced("short", bot)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("check of a short text: %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("check created %d entries", len(entries))
	}
	live := put(t, s, acmeMig, sourced("ACME moves OrderApp in March", jane))
	if err := s.Check(acmeMig, sourced("ACME moves OrderApp in March", jane)); err != nil {
		t.Fatalf("check of a retry: %v", err)
	}
	if err := s.Check(acmeMig, sourced("ACME MOVES OrderApp in March", bot)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("check of a repeat: %v", err)
	}
	gone := sourced("ACME moves OrderApp in March", jane)
	gone.Retracted = live.ID
	put(t, s, acmeMig, gone)
	if err := s.Check(acmeMig, sourced("ACME moves OrderApp in March", jane)); !errors.Is(err, ErrHistory) {
		t.Fatalf("check of a retracted claim: %v", err)
	}
	if got := liveIn(t, s, acmeMig); len(got) != 0 {
		t.Fatalf("live after retraction = %v", got)
	}
}

func TestLookupSaysWhereAClaimStands(t *testing.T) {
	s, _ := openStore(t)
	old := put(t, s, acmeMig, sourced("ACME moves OrderApp in March", jane))
	newer := sourced("ACME moves OrderApp in April", jane)
	newer.Supersedes = []string{old.ID}
	newer = put(t, s, acmeMig, newer)
	p := put(t, s, acmeMig, sourced("ACME stops OrderApp in May", bot))
	no := put(t, s, acmeMig, sourced("ACME stops OrderApp in June", bot))
	if err := s.Reject(acmeMig, no.ID, "jane"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id   string
		want Standing
		text string
	}{
		{old.ID, Standing{History: Superseded, By: newer.ID}, ""},
		{newer.ID, Standing{Live: true}, newer.Text},
		{p.ID, Standing{Pending: true}, p.Text},
		{no.ID, Standing{History: Rejected}, ""},
	} {
		c, st, ok, err := s.Lookup(acmeMig, tc.id)
		if err != nil || !ok || st != tc.want || c.Text != tc.text {
			t.Errorf("Lookup(%s) = %q, %+v, %v, %v; want %q, %+v", tc.id[:8], c.Text, st, ok, err, tc.text, tc.want)
		}
	}
	if _, _, ok, err := s.Lookup(acme, old.ID); ok || err != nil {
		t.Fatalf("lookup in the org found an engagement's claim: %v %v", ok, err)
	}
}

func TestAReadOnlyStoreReadsBesideAWriter(t *testing.T) {
	w, dir := openStore(t)
	c := put(t, w, acmeMig, sourced("ACME moves OrderApp in March", jane))
	path := filepath.Join(dir, "acme", "migration.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"type":"claim","text":"ACME half`); err != nil { // a writer mid-line
		t.Fatal(err)
	}
	f.Close()
	before := readFile(t, path)

	r, err := OpenStoreReadOnly(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if got := liveIn(t, r, acmeMig); len(got) != 1 || got[0] != "acme/migration:"+c.Text {
		t.Fatalf("read-only live = %v", got)
	}
	if got := liveIn(t, r, acmeAPI); len(got) != 0 {
		t.Fatalf("read-only live of an empty namespace = %v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "acme", "warehouse-api.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("reading created a file: %v", err)
	}
	if _, err := r.Append(acmeMig, sourced("ACME stops OrderApp in May", bot)); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("append to a read-only store: %v", err)
	}
	if err := r.Confirm(acmeMig, c.ID, "jane"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("confirm in a read-only store: %v", err)
	}
	if after := readFile(t, path); after != before {
		t.Fatal("a read-only store changed the file")
	}
}

func TestAWriterCutsOffAnUnfinishedLineAtOpen(t *testing.T) {
	w, dir := openStore(t)
	c := put(t, w, acmeMig, sourced("ACME moves OrderApp in March", jane))
	w.Close()
	path := filepath.Join(dir, "acme", "migration.jsonl")
	whole := readFile(t, path)
	writeFile(t, path, whole+`{"type":"claim","text":"cut`)

	again, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if got := liveIn(t, again, acmeMig); len(got) != 1 || got[0] != "acme/migration:"+c.Text {
		t.Fatalf("live after reopen = %v", got)
	}
	if got := readFile(t, path); got != whole {
		t.Fatalf("file after reopen = %q, want the unfinished line cut off", got)
	}
	put(t, again, acmeMig, sourced("ACME stops OrderApp in May", jane))
}

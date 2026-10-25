package claim

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

var (
	acme      = Namespace{Org: "acme"}
	acmeMig   = Namespace{Org: "acme", Engagement: "migration"}
	acmeAPI   = Namespace{Org: "acme", Engagement: "warehouse-api"}
	orderShop = Namespace{Org: "orderapp", Engagement: "checkout"}
)

func openStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, dir
}

func put(t *testing.T, s *Store, ns Namespace, c Claim) Claim {
	t.Helper()
	got, err := s.Append(ns, c)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func liveIn(t *testing.T, s *Store, ns Namespace) []string {
	t.Helper()
	live, err := s.Live(ns)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, c := range live {
		out = append(out, c.Namespace.String()+":"+c.Text)
	}
	return out
}

func TestEngagementReadsItsOwnAndItsOrgsClaims(t *testing.T) {
	s, _ := openStore(t)
	put(t, s, acme, sourced("ACME projects are sponsored by the COO", jane))
	put(t, s, acmeMig, sourced("The migration starts on 2026-11-03", jane))
	put(t, s, acmeAPI, sourced("The warehouse API is REST over HTTPS", jane))
	put(t, s, orderShop, sourced("OrderApp checkout supports guest orders", jane))

	want := []string{"acme/migration:The migration starts on 2026-11-03", "acme:ACME projects are sponsored by the COO"}
	if got := liveIn(t, s, acmeMig); !reflect.DeepEqual(got, want) {
		t.Fatalf("engagement reads %q, want own then org %q", got, want)
	}
	if got := liveIn(t, s, acme); !reflect.DeepEqual(got, []string{"acme:ACME projects are sponsored by the COO"}) {
		t.Fatalf("org reads %q, want only its own claims", got)
	}
	if got := liveIn(t, s, Namespace{Org: "globex"}); got != nil {
		t.Fatalf("an empty namespace reads %q, want nothing", got)
	}
}

func TestRecallStaysInsideTheNamespace(t *testing.T) {
	s, _ := openStore(t)
	put(t, s, acmeMig, sourced("The ACME budget is capped at 420,000 EUR", jane))
	put(t, s, orderShop, sourced("The OrderApp budget is capped at 420,000 EUR", jane))
	put(t, s, acmeAPI, sourced("The warehouse API budget is capped at 420,000 EUR", jane))

	got, err := s.Recall(orderShop, "budget capped 420,000 EUR", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Claim.Namespace != orderShop {
		t.Fatalf("recall in orderapp/checkout = %+v, want only its own claim", got)
	}
}

func TestLinksStayInsideOneNamespace(t *testing.T) {
	s, _ := openStore(t)
	org := put(t, s, acme, sourced("ACME projects are sponsored by the COO", jane))
	for name, c := range map[string]Claim{
		"supersedes": func() Claim {
			c := sourced("ACME projects are sponsored by the CIO", jane)
			c.Supersedes = []string{org.ID}
			return c
		}(),
		"challenges": func() Claim {
			c := sourced("ACME projects are sponsored by the CFO", jane)
			c.Challenges = []string{org.ID}
			return c
		}(),
		"retracted": func() Claim { c := sourced("ACME no longer names a sponsor", jane); c.Retracted = org.ID; return c }(),
	} {
		if _, err := s.Append(acmeMig, c); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s across namespaces: err = %v, want ErrInvalid", name, err)
		}
	}
}

func TestCiteCarriesAFactAcrossNamespaces(t *testing.T) {
	s, _ := openStore(t)
	org := put(t, s, acme, sourced("ACME projects are sponsored by the COO", jane))
	c := Claim{Text: "The migration is sponsored by ACME's COO", Trust: Derived, AsOf: "2026-01-15", Source: Cite(acme, org.ID), Author: jane}
	if _, err := s.Append(orderShop, c); err != nil {
		t.Fatalf("cite of a live claim: %v", err)
	}
	for name, src := range map[string]*Source{
		"unknown id":        Cite(acme, "nope"),
		"empty namespace":   Cite(Namespace{Org: "globex"}, org.ID),
		"no id":             {System: CiteSystem, Ref: "acme"},
		"invalid namespace": {System: CiteSystem, Ref: "ACME#" + org.ID},
	} {
		c.Source = src
		if _, err := s.Append(acmeMig, c); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
	gone := sourced("ACME projects have no sponsor at all", jane)
	gone.Supersedes = []string{org.ID}
	put(t, s, acme, gone)
	c.Source = Cite(acme, org.ID)
	if _, err := s.Append(acmeMig, c); !errors.Is(err, ErrInvalid) {
		t.Errorf("cite of a superseded claim: err = %v, want ErrInvalid", err)
	}
}

func TestInvalidNamespaceIsRefused(t *testing.T) {
	s, _ := openStore(t)
	for _, name := range []string{"", "ACME", "acme/", "acme/Mig", "acme/mig/x", "-acme", "acme--x", "../acme"} {
		ns, err := ParseNamespace(name)
		if !errors.Is(err, ErrInvalid) {
			t.Errorf("ParseNamespace(%q) err = %v, want ErrInvalid", name, err)
		}
		if _, err := s.Append(ns, sourced("ACME projects are sponsored by the COO", jane)); !errors.Is(err, ErrInvalid) {
			t.Errorf("Append to %q err = %v, want ErrInvalid", name, err)
		}
	}
	if ns, err := ParseNamespace("acme/warehouse-api"); err != nil || ns != acmeAPI {
		t.Fatalf("ParseNamespace = %v, %v", ns, err)
	}
}

func TestDecideNeedsTheProposalsNamespace(t *testing.T) {
	s, _ := openStore(t)
	p := put(t, s, acmeMig, sourced("The migration starts on 2026-11-03", bot))
	if err := s.Decide(acmeAPI, Decision{ClaimID: p.ID, Outcome: Confirm, Author: jane}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("decide in the wrong namespace: err = %v, want ErrInvalid", err)
	}
	if err := s.Decide(acmeMig, Decision{ClaimID: p.ID, Outcome: Confirm, Author: jane}); err != nil {
		t.Fatal(err)
	}
	if got := liveIn(t, s, acmeMig); len(got) != 1 {
		t.Fatalf("live = %q, want the confirmed proposal", got)
	}
}

func TestReadingCreatesNothingAndReopenKeepsTheState(t *testing.T) {
	s, dir := openStore(t)
	if _, err := s.Live(acmeMig); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("reading created %d entries", len(entries))
	}
	put(t, s, acme, sourced("ACME projects are sponsored by the COO", jane))
	put(t, s, acmeMig, sourced("The migration starts on 2026-11-03", jane))
	p := put(t, s, acmeMig, sourced("The migration ends on 2026-12-15", bot))
	before := liveIn(t, s, acmeMig)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if got := liveIn(t, again, acmeMig); !reflect.DeepEqual(got, before) {
		t.Fatalf("after reopen %q, want %q", got, before)
	}
	props, err := again.Proposals(acmeMig)
	if err != nil || len(props) != 1 || props[0].ID != p.ID || props[0].Namespace != acmeMig {
		t.Fatalf("proposals after reopen = %+v, %v", props, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "acme")); err != nil {
		t.Fatal(err)
	}
}

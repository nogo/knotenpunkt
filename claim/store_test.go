package claim

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
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
	for c := range live {
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

func TestConfirmNeedsTheProposalsNamespace(t *testing.T) {
	s, _ := openStore(t)
	p := put(t, s, acmeMig, sourced("The migration starts on 2026-11-03", bot))
	if err := s.Confirm(acmeAPI, p.ID, jane.ID); !errors.Is(err, ErrInvalid) {
		t.Fatalf("decide in the wrong namespace: err = %v, want ErrInvalid", err)
	}
	if err := s.Confirm(acmeMig, p.ID, jane.ID); err != nil {
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
	seq, err := again.Proposals(acmeMig)
	if err != nil {
		t.Fatal(err)
	}
	props := slices.Collect(seq)
	if len(props) != 1 || props[0].ID != p.ID || props[0].Namespace != acmeMig {
		t.Fatalf("proposals after reopen = %+v", props)
	}
	if _, err := os.Stat(filepath.Join(dir, "acme")); err != nil {
		t.Fatal(err)
	}
}

func TestAppendRefusesAClaimFromAnotherNamespace(t *testing.T) {
	s, _ := openStore(t)
	c := put(t, s, acmeMig, sourced("ACME moves OrderApp in March", jane))
	c.ID = ""
	if _, err := s.Append(acmeAPI, c); !errors.Is(err, ErrInvalid) {
		t.Fatalf("append to another namespace: got %v", err)
	}
	if got := liveIn(t, s, acmeAPI); len(got) != 0 {
		t.Fatalf("acme/warehouse-api live = %v, want none", got)
	}
}

func TestProposingCostsTheSameInASmallAndALargeNamespace(t *testing.T) {
	allocs := func(n int) float64 {
		s, _ := openStore(t)
		fill(t, s, acmeMig, n)
		// Built before measuring; each run proposes a new claim. 100 runs
		// spread a map's growth, which falls at random sizes, below the margin.
		var claims []Claim
		for i := range 101 {
			claims = append(claims, Claim{Text: fmt.Sprintf("ACME plans OrderApp review %d next quarter.", i), Trust: Unverified, AsOf: "2026-10-21", Author: bot})
		}
		next := 0
		return testing.AllocsPerRun(100, func() {
			if _, err := s.Append(acmeMig, claims[next]); err != nil {
				t.Fatal(err)
			}
			next++
		})
	}
	small, large := allocs(20), allocs(1000)
	if large > small+2 {
		t.Fatalf("allocations per proposal: %v with 20 claims, %v with 1000", small, large)
	}
}

func TestAppendingInsideALiveLoopNeitherBlocksNorChangesTheLoop(t *testing.T) {
	s, _ := openStore(t)
	for i := range 3 {
		put(t, s, acmeMig, sourced(fmt.Sprintf("ACME migration wave %d is planned", i), jane))
	}
	done := make(chan int)
	go func() {
		live, err := s.Live(acmeMig)
		if err != nil {
			t.Error(err)
		}
		seen := 0
		for c := range live {
			seen++
			p, err := s.Append(acmeMig, sourced("A follow-up to "+c.Text, bot))
			if err == nil {
				err = s.Confirm(acmeMig, p.ID, jane.ID)
			}
			if err != nil {
				t.Error(err)
			}
		}
		done <- seen
	}()
	select {
	case seen := <-done:
		if seen != 3 {
			t.Fatalf("loop saw %d claims, want the 3 from when it began", seen)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("appending inside a Live loop blocked")
	}
	if got := liveIn(t, s, acmeMig); len(got) != 6 {
		t.Fatalf("live after the loop = %d, want 6", len(got))
	}
}

func TestReadersRunWhileOneWriterAppends(t *testing.T) {
	s, _ := openStore(t)
	put(t, s, acme, sourced("ACME projects are sponsored by the COO", jane))
	stop := make(chan struct{})
	var readers sync.WaitGroup
	for range 4 {
		readers.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				live, err := s.Live(acmeMig)
				if err != nil {
					t.Error(err)
					return
				}
				for c := range live {
					if c.Text == "" {
						t.Error("read a claim without text")
					}
				}
				props, err := s.Proposals(acmeMig)
				if err != nil {
					t.Error(err)
					return
				}
				for range props {
				}
			}
		})
	}
	fillHistory(t, s, acmeMig, 200)
	close(stop)
	readers.Wait()
	if got := liveIn(t, s, acmeMig); len(got) != 101 {
		t.Fatalf("live = %d, want 100 of the namespace and 1 of the org", len(got))
	}
}

func TestHistoryLeavesMemory(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	onlyLive := Namespace{Org: "acme", Engagement: "only-live"}
	for i := range 1000 {
		put(t, s, onlyLive, sourced(fmt.Sprintf("The ACME review %d states that OrderApp release %d moves the checkout to the new payment provider before the freeze.", i, i%40), jane))
	}
	fillHistory(t, s, acmeMig, 2000) // 1000 live, 1000 superseded or rejected
	s.Close()
	base, withHistory := heldAfterOpen(t, dir, onlyLive), heldAfterOpen(t, dir, acmeMig)
	// History keeps an id and a status, about 100 B each; held bodies would
	// double the heap. 1.5 tells the two apart.
	if float64(withHistory) > 1.5*float64(base) {
		t.Fatalf("held after open: %d B with history, %d B for the same live claims without", withHistory, base)
	}
}

func TestIDIsTheHashOfNamespaceAndContent(t *testing.T) {
	a, _ := openStore(t)
	b, _ := openStore(t)
	c := sourced("ACME moves OrderApp in March", bot)
	first, second := put(t, a, acmeMig, c), put(t, b, acmeMig, c)
	if first.ID != second.ID || len(first.ID) != 64 {
		t.Fatalf("ids %q and %q, want one 64-character hash", first.ID, second.ID)
	}
	if other := put(t, a, acmeAPI, c); other.ID == first.ID {
		t.Fatal("another namespace gave the same id")
	}
	changes := map[string]func(*Claim){
		"text":       func(c *Claim) { c.Text += "." },
		"trust":      func(c *Claim) { c.Trust = Unverified },
		"asof":       func(c *Claim) { c.AsOf = "2026-01-16" },
		"source":     func(c *Claim) { c.Source = &Source{System: "sidenote", Ref: "blk-2", Version: "v1"} },
		"party":      func(c *Claim) { c.Party = "Jane Roe" },
		"quote":      func(c *Claim) { c.Quote = "moves in March" },
		"attributed": func(c *Claim) { c.Attributed = true },
		"supersedes": func(c *Claim) { c.Supersedes = []string{"x"} },
		"challenges": func(c *Claim) { c.Challenges = []string{"x"} },
		"retracted":  func(c *Claim) { c.Retracted = "x" },
		"author":     func(c *Claim) { c.Author = jane },
	}
	for name, change := range changes {
		d := c
		change(&d)
		if contentID(acmeMig, d) == first.ID {
			t.Errorf("changing %s kept the id", name)
		}
	}
	d := c
	d.RecordedAt = d.RecordedAt.AddDate(1, 0, 0)
	d.Supersedes = []string{}
	if contentID(acmeMig, d) != first.ID {
		t.Error("recorded_at or an empty list changed the id")
	}
}

func TestAppendingTheSameClaimAgainWritesNothing(t *testing.T) {
	s, dir := openStore(t)
	c := sourced("ACME moves OrderApp in March", bot)
	first := put(t, s, acmeMig, c)
	again := put(t, s, acmeMig, c)
	if again.ID != first.ID || !again.RecordedAt.Equal(first.RecordedAt) {
		t.Fatalf("retry gave %+v, want the claim as first recorded", again)
	}
	if err := s.Confirm(acmeMig, first.ID, jane.ID); err != nil {
		t.Fatal(err)
	}
	if live := put(t, s, acmeMig, c); live.ID != first.ID {
		t.Fatalf("retry after confirm gave %q", live.ID)
	}
	data, err := os.ReadFile(filepath.Join(dir, "acme", "migration.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), `"type":"claim"`); n != 1 {
		t.Fatalf("log holds %d claim lines, want 1", n)
	}
}

func TestRepeatingAClaimThatIsHistorySaysWhatBecameOfIt(t *testing.T) {
	s, dir := openStore(t)
	rejected := put(t, s, acmeMig, sourced("ACME moves OrderApp in March", bot))
	if err := s.Reject(acmeMig, rejected.ID, jane.ID); err != nil {
		t.Fatal(err)
	}
	old := put(t, s, acmeMig, sourced("ACME runs OrderApp version 3", jane))
	fix := sourced("ACME runs OrderApp version 4", jane)
	fix.Supersedes = []string{old.ID}
	fix = put(t, s, acmeMig, fix)
	wrong := put(t, s, acmeMig, sourced("ACME runs OrderApp in Berlin", jane))
	back := put(t, s, acmeMig, Claim{Text: "Retracting: OrderApp never ran in Berlin", Trust: Unverified, AsOf: "2026-03-01", Retracted: wrong.ID, Author: jane})

	want := []HistoryError{
		{ID: rejected.ID, State: Rejected},
		{ID: old.ID, State: Superseded, By: fix.ID},
		{ID: wrong.ID, State: Retracted, By: back.ID},
		{ID: back.ID, State: Retraction},
	}
	repeat := []Claim{rejected, old, wrong, back}
	check := func(s *Store) {
		t.Helper()
		for i, c := range repeat {
			c.RecordedAt = time.Time{}
			_, err := s.Append(acmeMig, c)
			var h *HistoryError
			if !errors.As(err, &h) || *h != want[i] {
				t.Errorf("repeat %s: got %v, want %+v", want[i].State, err, want[i])
			}
			if !errors.Is(err, ErrHistory) || errors.Is(err, ErrInvalid) {
				t.Errorf("repeat %s: %v is not ErrHistory alone", want[i].State, err)
			}
		}
	}
	check(s)
	s.Close()
	again, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	check(again)
}

func TestAWrongCallerIDIsRefused(t *testing.T) {
	s, _ := openStore(t)
	c := sourced("ACME moves OrderApp in March", jane)
	c.ID = "c-1"
	if _, err := s.Append(acmeMig, c); !errors.Is(err, ErrInvalid) {
		t.Fatalf("caller id: got %v", err)
	}
	c.ID = contentID(acmeMig, c)
	if got := put(t, s, acmeMig, c); got.ID != c.ID {
		t.Fatalf("matching caller id: got %q", got.ID)
	}
}

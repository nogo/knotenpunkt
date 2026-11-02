package claim

import (
	"fmt"
	"testing"
)

// fill writes n claims to ns as an embedding system would over months: most
// claims are proposed and confirmed, some are rejected, and every tenth one
// corrects the claim before it.
func fill(tb testing.TB, s *Store, ns Namespace, n int) {
	tb.Helper()
	prev := ""
	for i := range n {
		c := Claim{
			Text:   fmt.Sprintf("The ACME review %d states that OrderApp release %d moves the checkout to the new payment provider before the freeze.", i, i%40),
			Trust:  Sourced,
			AsOf:   "2026-10-20",
			Source: &Source{System: "sidenote", Ref: fmt.Sprintf("blk-%d", i), Version: "v1"},
			Author: Author{Kind: Proposer, ID: "intake"},
		}
		if i%10 == 9 && prev != "" {
			c.Supersedes = []string{prev}
		}
		got, err := s.Append(ns, c)
		if err != nil {
			tb.Fatal(err)
		}
		if i%7 == 6 {
			err = s.Reject(ns, got.ID, "jane")
		} else {
			err = s.Confirm(ns, got.ID, "jane")
			prev = got.ID
		}
		if err != nil {
			tb.Fatal(err)
		}
	}
}

var sizes = []int{200, 2000}

func benchStore(b *testing.B, n int) (*Store, string) {
	b.Helper()
	dir := b.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		b.Fatal(err)
	}
	fill(b, s, acmeMig, n)
	return s, dir
}

func BenchmarkOpenNamespace(b *testing.B) {
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			s, dir := benchStore(b, n)
			s.Close()
			b.ReportAllocs()
			for b.Loop() {
				re, err := OpenStore(dir)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := re.Live(acmeMig); err != nil {
					b.Fatal(err)
				}
				re.Close()
			}
		})
	}
}

func BenchmarkLive(b *testing.B) {
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			s, _ := benchStore(b, n)
			defer s.Close()
			b.ReportAllocs()
			for b.Loop() {
				if _, err := s.Live(acmeMig); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkRecall(b *testing.B) {
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			s, _ := benchStore(b, n)
			defer s.Close()
			b.ReportAllocs()
			for b.Loop() {
				if _, err := s.Recall(acmeMig, "Does the checkout move to the new payment provider?", 5); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkPropose(b *testing.B) {
	for _, n := range sizes {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			s, _ := benchStore(b, n)
			defer s.Close()
			c := Claim{Text: "ACME plans a second OrderApp review next quarter.", Trust: Unverified, AsOf: "2026-10-21", Author: Author{Kind: Proposer, ID: "intake"}}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := s.Append(acmeMig, c); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

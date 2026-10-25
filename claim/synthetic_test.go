package claim

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const syntheticVault = "../testdata/synthetic-vault"

// loadSynthetic replays the synthetic vault's sidenote log into a Store through
// the public API: sessions propose, the user confirms.
func loadSynthetic(t *testing.T) *Store {
	t.Helper()
	s, _ := openStore(t)
	files, err := filepath.Glob(filepath.Join(syntheticVault, "log", "*", "????-??-??.jsonl"))
	if err != nil || len(files) == 0 {
		t.Fatalf("synthetic log: %v, %d files", err, len(files))
	}
	where := map[string]Namespace{}
	for _, f := range files {
		r, err := os.Open(f)
		if err != nil {
			t.Fatal(err)
		}
		sc := bufio.NewScanner(r)
		sc.Buffer(nil, 1<<20)
		for sc.Scan() {
			var rec struct {
				Type       string   `json:"type"`
				ID         string   `json:"id"`
				Namespace  string   `json:"namespace"`
				Text       string   `json:"text"`
				Trust      Trust    `json:"trust"`
				AsOf       string   `json:"asof"`
				Party      string   `json:"party"`
				Attributed bool     `json:"attributed"`
				Supersedes []string `json:"supersedes"`
				Source     *struct {
					Ref, Version string
				} `json:"source"`
				Claim string `json:"claim"`
			}
			if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
				t.Fatal(err)
			}
			switch rec.Type {
			case "claim":
				ns, err := ParseNamespace(rec.Namespace)
				if err != nil {
					t.Fatalf("%s: %v", rec.ID, err)
				}
				c := Claim{ID: rec.ID, Text: rec.Text, Trust: rec.Trust, AsOf: rec.AsOf, Party: rec.Party,
					Attributed: rec.Attributed, Supersedes: rec.Supersedes, Author: bot}
				if rec.Source != nil {
					c.Source = &Source{System: "sidenote", Ref: rec.Source.Ref, Version: rec.Source.Version}
				}
				put(t, s, ns, c)
				where[rec.ID] = ns
			case "claim_decision":
				if err := s.Decide(where[rec.Claim], Decision{ClaimID: rec.Claim, Outcome: Confirm, Author: jane}); err != nil {
					t.Fatalf("%s: %v", rec.Claim, err)
				}
			}
		}
		r.Close()
	}
	return s
}

type label struct {
	Proposal        string `json:"proposal"`
	Namespace       string `json:"namespace"`
	Target          string `json:"target"`
	TargetNamespace string `json:"target_namespace"`
	Label           string `json:"label"`
}

func syntheticLabels(t *testing.T) []label {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(syntheticVault, "labels.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var out []label
	dec := json.NewDecoder(bytes.NewReader(data))
	for dec.More() {
		var l label
		if err := dec.Decode(&l); err != nil {
			t.Fatal(err)
		}
		out = append(out, l)
	}
	return out
}

// TestSyntheticVaultKeepsNamespacesApart checks the namespace rules on the
// synthetic vault: a labelled target is readable from its proposal's namespace,
// and recall for a lookalike from another org never returns that org's claim.
func TestSyntheticVaultKeepsNamespacesApart(t *testing.T) {
	s := loadSynthetic(t)
	labels := syntheticLabels(t)
	if len(labels) != 42 {
		t.Fatalf("labels = %d, want 42", len(labels))
	}
	for _, l := range labels {
		ns, _ := ParseNamespace(l.Namespace)
		live, err := s.Live(ns)
		if err != nil {
			t.Fatal(err)
		}
		readable := false
		for _, c := range live {
			readable = readable || c.ID == l.Target
		}
		if l.Label == "other-namespace" {
			if readable {
				t.Errorf("%s: %s reads %s from %s", l.Proposal, l.Namespace, l.Target, l.TargetNamespace)
			}
			props, _ := s.Proposals(ns)
			for _, p := range props {
				if p.ID != l.Proposal {
					continue
				}
				matches, err := s.RecallClaim(ns, p, 20)
				if err != nil {
					t.Fatal(err)
				}
				for _, m := range matches {
					if m.Claim.Namespace.Org != ns.Org {
						t.Errorf("%s: recall in %s returned %s from %s", l.Proposal, ns, m.Claim.ID, m.Claim.Namespace)
					}
				}
			}
			continue
		}
		if !readable {
			t.Errorf("%s: target %s in %s is not readable from %s", l.Proposal, l.Target, l.TargetNamespace, l.Namespace)
		}
	}
}

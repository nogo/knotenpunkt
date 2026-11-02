// Command intake is a throwaway spike: it judges every test proposal against
// every live claim its namespace reads, and records each claim's word-overlap
// rank, so recall strategies can be compared from one run. Not product code.
//
//	go run ./spikes/intake -url http://localhost:9481 -model qwen-moe -out results.jsonl
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/nogo/knotenpunkt/claim"
	"github.com/nogo/knotenpunkt/decide"
)

var vault = "testdata/synthetic-vault"

var options = []decide.Option{
	{ID: "conflicts", Description: "both cannot be true at once: the new claim changes or contradicts a fact of the old one"},
	{ID: "agrees", Description: "the new claim states the same fact as the old one, possibly with more detail"},
	{ID: "unrelated", Description: "the claims are about different facts, and both can be true"},
}

type proposal struct {
	ID, Namespace, Text, Target, Label, Set string
}

type row struct {
	Proposal  string             `json:"proposal"`
	Set       string             `json:"set"`
	Label     string             `json:"label"`
	Target    string             `json:"target"`
	Candidate string             `json:"candidate"`
	IsTarget  bool               `json:"is_target"`
	Rank      int                `json:"rank"` // word-overlap rank, 1 is best; 0 = no shared word
	Pred      string             `json:"pred"`
	Probs     map[string]float64 `json:"probs"`
	MS        int64              `json:"ms"`
}

func main() {
	url := flag.String("url", "http://localhost:9481", "llama-server base URL")
	model := flag.String("model", "qwen-moe", "model")
	out := flag.String("out", "", "results JSONL")
	flag.StringVar(&vault, "vault", vault, "synthetic vault directory")
	pad := flag.String("pad", "", "optional JSONL of extra claims {id,text} added to -pad-ns; then only that namespace's proposals run")
	padNS := flag.String("pad-ns", "acme/migration", "namespace that -pad fills")
	order := flag.String("order", "old-first", "old-first or new-first: which claim opens the prompt")
	limit := flag.Int("limit", 0, "run only the first N proposals (0 = all)")
	flag.Parse()
	dir, err := os.MkdirTemp("", "intake-store")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	store, err := claim.OpenStore(dir)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	if err := load(store); err != nil {
		log.Fatal(err)
	}
	props, err := proposals()
	if err != nil {
		log.Fatal(err)
	}
	if *pad != "" {
		ns, err := claim.ParseNamespace(*padNS)
		if err != nil {
			log.Fatal(err)
		}
		n := 0
		each(*pad, func(m map[string]any) {
			c := claim.Claim{ID: "c-" + m["id"].(string), Text: m["text"].(string), Trust: claim.Unverified, AsOf: "2026-09-20",
				Author: claim.Author{Kind: claim.User, ID: "owner"}}
			if _, err := store.Append(ns, c); err != nil {
				log.Fatalf("pad %s: %v", c.ID, err)
			}
			n++
		})
		var only []proposal
		for _, p := range props {
			if p.Namespace == *padNS {
				only = append(only, p)
			}
		}
		props = only
		fmt.Fprintf(os.Stderr, "padded %s with %d claims; %d proposals\n", *padNS, n, len(props))
	}
	if *limit > 0 && *limit < len(props) {
		props = props[:*limit]
	}
	f, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	client := &decide.Client{BaseURL: *url, Model: *model}
	start := time.Now()
	for i, p := range props {
		ns, err := claim.ParseNamespace(p.Namespace)
		if err != nil {
			log.Fatal(err)
		}
		live, err := store.Live(ns)
		if err != nil {
			log.Fatal(err)
		}
		matches, err := store.Recall(ns, p.Text, len(live))
		if err != nil {
			log.Fatal(err)
		}
		rank := map[string]int{}
		for r, m := range matches {
			rank[m.Claim.ID] = r + 1
		}
		for _, c := range live {
			t0 := time.Now()
			res, err := decideRetry(client, decide.Request{
				Question: "Does the new claim conflict with the old one?",
				Options:  options,
				Context:  pairContext(*order, c.Text, p.Text),
			})
			if err != nil && res.Probs == nil {
				log.Fatalf("%s vs %s: %v", p.ID, c.ID, err)
			}
			enc.Encode(row{p.ID, p.Set, p.Label, p.Target, c.ID, c.ID == p.Target, rank[c.ID], res.Choice, res.Probs, time.Since(t0).Milliseconds()})
		}
		fmt.Fprintf(os.Stderr, "%d/%d %s %d claims %s\n", i+1, len(props), p.ID, len(live), time.Since(start).Round(time.Second))
	}
}

// pairContext puts the old or the new claim first. New-first lets every call
// for one proposal share the same prompt beginning.
func pairContext(order, old, new string) []string {
	if order == "new-first" {
		return []string{"New claim: " + new, "Old claim: " + old}
	}
	return []string{"Old claim: " + old, "New claim: " + new}
}

// load replays the synthetic sidenote log into the store, as the claim tests do.
func load(s *claim.Store) error {
	files, _ := filepath.Glob(filepath.Join(vault, "log", "*", "????-??-??.jsonl"))
	where := map[string]claim.Namespace{}
	user := claim.Author{Kind: claim.User, ID: "owner"}
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(nil, 1<<20)
		for sc.Scan() {
			var r struct {
				Type, ID, Namespace, Text, AsOf, Party, Claim string
				Trust                                         claim.Trust
				Attributed                                    bool
				Supersedes                                    []string
				Source                                        *struct{ Ref, Version string }
				Author                                        struct{ Kind string }
			}
			if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
				return err
			}
			switch r.Type {
			case "claim":
				if r.ID[2] == 'p' || r.ID[2] == 'x' { // open proposals are the test input, not the store
					continue
				}
				ns, err := claim.ParseNamespace(r.Namespace)
				if err != nil {
					return err
				}
				var asof struct {
					AsOf string `json:"asof"`
				}
				json.Unmarshal(sc.Bytes(), &asof)
				c := claim.Claim{ID: r.ID, Text: r.Text, Trust: r.Trust, AsOf: asof.AsOf, Party: r.Party, Attributed: r.Attributed,
					Supersedes: r.Supersedes, Author: claim.Author{Kind: claim.Proposer, ID: "session"}}
				if r.Source != nil {
					c.Source = &claim.Source{System: "sidenote", Ref: r.Source.Ref, Version: r.Source.Version}
				}
				if _, err := s.Append(ns, c); err != nil {
					return fmt.Errorf("%s: %w", r.ID, err)
				}
				where[r.ID] = ns
			case "claim_decision":
				if err := s.Confirm(where[r.Claim], r.Claim, user.ID); err != nil {
					return fmt.Errorf("%s: %w", r.Claim, err)
				}
			}
		}
		f.Close()
	}
	return nil
}

// proposals are the 36 labelled ones from the vault and the 60 hard ones.
func proposals() ([]proposal, error) {
	var out []proposal
	texts := map[string]string{}
	files, _ := filepath.Glob(filepath.Join(vault, "log", "*", "????-??-??.jsonl"))
	for _, path := range files {
		each(path, func(m map[string]any) {
			if m["type"] == "claim" {
				texts[m["id"].(string)] = m["text"].(string)
			}
		})
	}
	each(filepath.Join(vault, "labels.jsonl"), func(m map[string]any) {
		if m["label"] == "other-namespace" {
			return
		}
		id := m["proposal"].(string)
		out = append(out, proposal{id, m["namespace"].(string), texts[id], m["target"].(string), m["label"].(string), "easy"})
	})
	label := map[string]string{"conflicts": "conflicts", "agrees": "agrees", "unrelated": "unrelated"}
	each(filepath.Join(vault, "hard", "candidates.jsonl"), func(m map[string]any) {
		out = append(out, proposal{m["id"].(string), m["namespace"].(string), m["new"].(string), m["seed"].(string), label[m["intended"].(string)], "hard"})
	})
	return out, nil
}

func each(path string, fn func(map[string]any)) {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		var m map[string]any
		if json.Unmarshal(sc.Bytes(), &m) == nil {
			fn(m)
		}
	}
}

// decideRetry retries a request the router dropped, such as when another
// client made it swap models. A model answer without an option is not retried.
func decideRetry(client *decide.Client, r decide.Request) (decide.Result, error) {
	var res decide.Result
	var err error
	for attempt := 0; attempt < 8; attempt++ {
		res, err = client.Decide(context.Background(), r)
		if err == nil || errors.Is(err, decide.ErrNoOption) {
			return res, err
		}
		fmt.Fprintf(os.Stderr, "retry %d after: %v\n", attempt+1, err)
		time.Sleep(15 * time.Second)
	}
	return res, err
}

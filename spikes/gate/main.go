// Command gate is a throwaway spike for the question gate and the slow path:
// System One asks "do these claims answer the question?", System Two reads the
// namespace's source document and proposes a claim, and the gate is asked
// again with that claim confirmed. Not product code.
//
//	go run ./spikes/gate -phase gate   -model qwen-moe -questions q.jsonl -docs docs -out gate.jsonl
//	go run ./spikes/gate -phase slow   -model qwen-27b -questions q.jsonl -docs docs -out slow.jsonl
//	go run ./spikes/gate -phase regate -model qwen-moe -questions q.jsonl -docs docs -in slow.jsonl -out regate.jsonl
//	go run ./spikes/gate -phase grade  -model qwen-moe -questions q.jsonl -docs docs -in slow.jsonl -out grade.jsonl
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nogo/knotenpunkt/claim"
	"github.com/nogo/knotenpunkt/decide"
)

type question struct {
	ID         string   `json:"id"`
	Namespace  string   `json:"namespace"`
	Lang       string   `json:"lang"`
	Question   string   `json:"question"`
	Answerable string   `json:"answerable"`
	ClaimIDs   []string `json:"claim_ids"`
	Expected   string   `json:"expected"`
}

var gateOptions = []decide.Option{
	{ID: "yes", Description: "the claims state the answer"},
	{ID: "no", Description: "the answer is not in these claims"},
}

var gradeOptions = []decide.Option{
	{ID: "match", Description: "the given answer says the same as the expected answer"},
	{ID: "differ", Description: "the given answer is wrong, missing or says something else"},
}

type gateRow struct {
	Question   string             `json:"question"`
	Namespace  string             `json:"namespace"`
	Lang       string             `json:"lang"`
	Answerable string             `json:"answerable"`
	Model      string             `json:"model"`
	Variant    string             `json:"variant"`
	Claims     int                `json:"claims"`
	ClaimIDs   []string           `json:"claim_ids"`
	Choice     string             `json:"choice"`
	Probs      map[string]float64 `json:"probs"`
	MS         int64              `json:"ms"`
	Err        string             `json:"err,omitempty"`
}

type slowRow struct {
	Question         string `json:"question"`
	Namespace        string `json:"namespace"`
	Lang             string `json:"lang"`
	Answerable       string `json:"answerable"`
	Model            string `json:"model"`
	Doc              string `json:"doc"`
	Raw              string `json:"raw"`
	Found            bool   `json:"found"`
	Answer           string `json:"answer"`
	Claim            string `json:"claim"`
	Quote            string `json:"quote"`
	ParseErr         string `json:"parse_err,omitempty"`
	QuoteVerbatim    bool   `json:"quote_verbatim"`
	MS               int64  `json:"ms"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	Err              string `json:"err,omitempty"`
}

type regateRow struct {
	Question   string             `json:"question"`
	Answerable string             `json:"answerable"`
	Lang       string             `json:"lang"`
	Model      string             `json:"model"`
	Accepted   bool               `json:"accepted"`
	Refusal    string             `json:"refusal,omitempty"`
	Before     string             `json:"before"`
	Choice     string             `json:"choice"`
	Probs      map[string]float64 `json:"probs"`
	MS         int64              `json:"ms"`
}

type gradeRow struct {
	Question   string             `json:"question"`
	Answerable string             `json:"answerable"`
	Model      string             `json:"model"`
	Found      bool               `json:"found"`
	Choice     string             `json:"choice"`
	Probs      map[string]float64 `json:"probs"`
	Substring  bool               `json:"substring"`
	Correct    bool               `json:"correct"` // decide's grade, or found=false for "none"
}

func main() {
	vault := flag.String("vault", "testdata/synthetic-vault", "synthetic vault")
	qpath := flag.String("questions", "/tmp/kp-night/gate/questions.jsonl", "questions JSONL")
	docs := flag.String("docs", "/tmp/kp-night/gate/docs", "source documents")
	url := flag.String("url", "http://localhost:9481", "llama-server base URL")
	model := flag.String("model", "qwen-moe", "model")
	phase := flag.String("phase", "", "gate, slow, regate or grade")
	in := flag.String("in", "", "previous phase output (slow output for regate and grade)")
	out := flag.String("out", "", "results JSONL")
	limit := flag.Int("limit", 0, "only the first n questions (smoke tests)")
	flag.Parse()
	root, err := os.MkdirTemp("", "gate-run")
	if err != nil {
		log.Fatal(err)
	}
	tmpRoot = root
	defer os.RemoveAll(root)
	if *out == "" || *phase == "" {
		log.Fatal("need -phase and -out")
	}
	var qs []question
	readJSONL(*qpath, func(b []byte) {
		var q question
		if err := json.Unmarshal(b, &q); err != nil {
			log.Fatal(err)
		}
		qs = append(qs, q)
	})
	if *limit > 0 && *limit < len(qs) {
		qs = qs[:*limit]
	}
	f, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	client := &decide.Client{BaseURL: *url, Model: *model}
	start := time.Now()
	progress := func(i int, id string) {
		fmt.Fprintf(os.Stderr, "%s %d/%d %s %s\n", *phase, i+1, len(qs), id, time.Since(start).Round(time.Second))
	}
	switch *phase {
	case "gate":
		store := openVault(*vault)
		defer store.Close()
		for i, q := range qs {
			ns := mustNS(q.Namespace)
			live, err := store.Live(ns)
			if err != nil {
				log.Fatal(err)
			}
			matches, err := store.Recall(ns, q.Question, 5)
			if err != nil {
				log.Fatal(err)
			}
			var top []claim.Claim
			for _, m := range matches {
				top = append(top, m.Claim)
			}
			for _, v := range []struct {
				name   string
				claims []claim.Claim
			}{{"all", live}, {"top5", top}} {
				enc.Encode(askGate(client, *model, q, v.name, v.claims))
			}
			progress(i, q.ID)
		}
	case "slow":
		for i, q := range qs {
			enc.Encode(slow(*url, *model, *docs, q))
			progress(i, q.ID)
		}
	case "regate":
		byID := map[string]question{}
		for _, q := range qs {
			byID[q.ID] = q
		}
		before := map[string]string{}
		var rows []slowRow
		readJSONL(*in, func(b []byte) {
			var r slowRow
			json.Unmarshal(b, &r)
			rows = append(rows, r)
		})
		for i, r := range rows {
			q, ok := byID[r.Question]
			if !ok || !r.Found || !r.QuoteVerbatim {
				continue
			}
			store := openVault(*vault)
			ns := mustNS(q.Namespace)
			live, _ := store.Live(ns)
			pre := askGate(client, *model, q, "all", live)
			before[q.ID] = pre.Choice
			row := regateRow{Question: q.ID, Answerable: q.Answerable, Lang: q.Lang, Model: *model, Before: pre.Choice}
			c := claim.Claim{Text: r.Claim, Trust: claim.Sourced, AsOf: "2026-10-25", Attributed: true, Quote: r.Quote,
				Source: &claim.Source{System: "doc", Ref: r.Doc, Version: "v1"},
				Author: claim.Author{Kind: claim.Proposer, ID: "slow-path"}}
			got, err := store.Append(ns, c)
			if err != nil {
				row.Refusal = err.Error()
			} else if err := store.Confirm(ns, got.ID, "owner"); err != nil {
				row.Refusal = err.Error()
			} else {
				row.Accepted = true
				live, _ = store.Live(ns)
				g := askGate(client, *model, q, "all", live)
				row.Choice, row.Probs, row.MS = g.Choice, g.Probs, g.MS
			}
			store.Close()
			enc.Encode(row)
			progress(i, q.ID)
		}
	case "grade":
		byID := map[string]question{}
		for _, q := range qs {
			byID[q.ID] = q
		}
		var rows []slowRow
		readJSONL(*in, func(b []byte) {
			var r slowRow
			json.Unmarshal(b, &r)
			rows = append(rows, r)
		})
		for i, r := range rows {
			q, ok := byID[r.Question]
			if !ok {
				continue
			}
			row := gradeRow{Question: q.ID, Answerable: q.Answerable, Model: *model, Found: r.Found,
				Substring: strings.Contains(norm(r.Answer), norm(q.Expected))}
			if r.Found {
				res, err := decideRetry(client, decide.Request{
					Question: "Does the given answer match the expected answer?",
					Options:  gradeOptions,
					Context:  []string{"Question: " + q.Question, "Expected answer: " + q.Expected, "Given answer: " + r.Answer},
				})
				if err != nil && res.Probs == nil {
					log.Fatal(err)
				}
				row.Choice, row.Probs = res.Choice, res.Probs
			}
			if q.Answerable == "none" {
				row.Correct = !r.Found
			} else {
				row.Correct = r.Found && row.Choice == "match"
			}
			enc.Encode(row)
			progress(i, q.ID)
		}
	default:
		log.Fatalf("unknown phase %q", *phase)
	}
}

func askGate(client *decide.Client, model string, q question, variant string, claims []claim.Claim) gateRow {
	ctx := make([]string, len(claims))
	ids := make([]string, len(claims))
	for i, c := range claims {
		ctx[i] = fmt.Sprintf("%d. %s", i+1, c.Text)
		ids[i] = c.ID
	}
	if len(ctx) == 0 {
		ctx = []string{"(no claims)"}
	}
	t0 := time.Now()
	res, err := decideRetry(client, decide.Request{
		Question: "Above are the claims recorded so far. Do these claims answer the question: " + q.Question,
		Options:  gateOptions,
		Context:  ctx,
	})
	row := gateRow{Question: q.ID, Namespace: q.Namespace, Lang: q.Lang, Answerable: q.Answerable, Model: model,
		Variant: variant, Claims: len(claims), ClaimIDs: ids, Choice: res.Choice, Probs: res.Probs, MS: time.Since(t0).Milliseconds()}
	if err != nil {
		if res.Probs == nil {
			log.Fatalf("%s: %v", q.ID, err)
		}
		row.Err = err.Error()
	}
	return row
}

func docFile(ns string) string { return strings.ReplaceAll(ns, "/", "-") + ".md" }

const slowSystem = `You answer questions from one source document and nothing else.
Reply with one JSON object: {"found": true|false, "answer": "...", "claim": "...", "quote": "..."}.
- found: true only if the document states the answer.
- answer: a short answer, in the language of the question; empty if not found.
- claim: one self-contained sentence stating the fact, attributed to the document (for example "The project notes state that ..."), at least 10 characters; empty if not found.
- quote: the exact span of the document that supports it, copied character for character; empty if not found.`

func slow(url, model, docs string, q question) slowRow {
	name := docFile(q.Namespace)
	row := slowRow{Question: q.ID, Namespace: q.Namespace, Lang: q.Lang, Answerable: q.Answerable, Model: model, Doc: name}
	doc, err := os.ReadFile(filepath.Join(docs, name))
	if err != nil {
		row.Err = err.Error()
		return row
	}
	body, _ := json.Marshal(map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": slowSystem},
			{"role": "user", "content": "Document " + name + ":\n\n" + string(doc) + "\n\n---\nQuestion: " + q.Question},
		},
		"max_tokens":           2000,
		"temperature":          0,
		"response_format":      map[string]string{"type": "json_object"},
		"chat_template_kwargs": map[string]bool{"enable_thinking": true},
	})
	var raw []byte
	var t0 time.Time
	for attempt := 0; ; attempt++ {
		t0 = time.Now()
		resp, err := http.Post(strings.TrimRight(url, "/")+"/v1/chat/completions", "application/json", bytes.NewReader(body))
		if err == nil {
			raw, err = io.ReadAll(resp.Body)
			resp.Body.Close()
			if err == nil && resp.StatusCode != http.StatusOK {
				err = fmt.Errorf("%s: %.300s", resp.Status, raw)
			}
		}
		row.MS = time.Since(t0).Milliseconds()
		if err == nil {
			break
		}
		if attempt == 7 {
			row.Err = err.Error()
			return row
		}
		fmt.Fprintf(os.Stderr, "retry %d after: %v\n", attempt+1, err)
		time.Sleep(15 * time.Second)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Choices) == 0 {
		row.Err = "bad response"
		return row
	}
	row.PromptTokens, row.CompletionTokens = out.Usage.PromptTokens, out.Usage.CompletionTokens
	row.Raw = out.Choices[0].Message.Content
	var ans struct {
		Found  bool   `json:"found"`
		Answer string `json:"answer"`
		Claim  string `json:"claim"`
		Quote  string `json:"quote"`
	}
	if err := json.Unmarshal([]byte(jsonPart(row.Raw)), &ans); err != nil {
		row.ParseErr = err.Error()
		return row
	}
	row.Found, row.Answer, row.Claim, row.Quote = ans.Found, ans.Answer, ans.Claim, ans.Quote
	row.QuoteVerbatim = ans.Quote != "" && strings.Contains(spaces(string(doc)), spaces(ans.Quote))
	return row
}

// jsonPart drops anything around the outermost JSON object, such as a fence.
func jsonPart(s string) string {
	i, j := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if i < 0 || j < i {
		return s
	}
	return s[i : j+1]
}

func spaces(s string) string { return strings.Join(strings.Fields(s), " ") }

func norm(s string) string {
	return strings.ToLower(spaces(strings.Map(func(r rune) rune {
		if strings.ContainsRune(".,;:!?\"'()", r) {
			return ' '
		}
		return r
	}, s)))
}

func mustNS(s string) claim.Namespace {
	ns, err := claim.ParseNamespace(s)
	if err != nil {
		log.Fatal(err)
	}
	return ns
}

func readJSONL(path string, fn func([]byte)) {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<22)
	for sc.Scan() {
		if len(bytes.TrimSpace(sc.Bytes())) > 0 {
			fn(append([]byte(nil), sc.Bytes()...))
		}
	}
}

// tmpRoot holds every store this run opens; main removes it at the end.
var tmpRoot string

// openVault loads the synthetic vault into a fresh store, as spikes/intake does.
func openVault(vault string) *claim.Store {
	dir, err := os.MkdirTemp(tmpRoot, "gate-store")
	if err != nil {
		log.Fatal(err)
	}
	store, err := claim.OpenStore(dir)
	if err != nil {
		log.Fatal(err)
	}
	if err := load(store, vault); err != nil {
		log.Fatal(err)
	}
	return store
}

// load replays the synthetic sidenote log into the store, skipping the open
// proposals (c-p*, c-x*), which are test input.
func load(s *claim.Store, vault string) error {
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
				Type       string      `json:"type"`
				ID         string      `json:"id"`
				Namespace  string      `json:"namespace"`
				Text       string      `json:"text"`
				AsOf       string      `json:"asof"`
				Party      string      `json:"party"`
				Claim      string      `json:"claim"`
				Trust      claim.Trust `json:"trust"`
				Attributed bool        `json:"attributed"`
				Supersedes []string    `json:"supersedes"`
				Source     *struct {
					Ref     string `json:"ref"`
					Version string `json:"version"`
				} `json:"source"`
			}
			if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
				return err
			}
			switch r.Type {
			case "claim":
				if strings.HasPrefix(r.ID, "c-p") || strings.HasPrefix(r.ID, "c-x") {
					continue
				}
				ns, err := claim.ParseNamespace(r.Namespace)
				if err != nil {
					return err
				}
				c := claim.Claim{ID: r.ID, Text: r.Text, Trust: r.Trust, AsOf: r.AsOf, Party: r.Party, Attributed: r.Attributed,
					Supersedes: r.Supersedes, Author: claim.Author{Kind: claim.Proposer, ID: "session"}}
				if r.Source != nil {
					c.Source = &claim.Source{System: "sidenote", Ref: r.Source.Ref, Version: r.Source.Version}
				}
				if _, err := s.Append(ns, c); err != nil {
					return fmt.Errorf("%s: %w", r.ID, err)
				}
				where[r.ID] = ns
			case "claim_decision":
				if _, ok := where[r.Claim]; !ok {
					continue
				}
				if err := s.Confirm(where[r.Claim], r.Claim, user.ID); err != nil {
					return fmt.Errorf("%s: %w", r.Claim, err)
				}
			}
		}
		f.Close()
	}
	return nil
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

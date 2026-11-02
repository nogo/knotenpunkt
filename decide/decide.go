// Package decide is System One: it asks a local model to pick one of a fixed
// list of options and reads the answer's probabilities from one token's
// log-probabilities. It writes nothing and keeps no state. Reach for it when a
// judgement over text has a closed set of answers; escalate when it is Unsure.
package decide

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
)

// ErrNoOption means the model's first token was none of the option keys.
var ErrNoOption = errors.New("decide: no option in the model's answer")

// keys are the one-character labels the model answers with, in option order.
const keys = "ABCDEFGHI"

// Option is one answer the model may give.
type Option struct {
	ID          string
	Description string
}

// Request is one decision. Context holds the texts the question is about, such
// as claims; they are listed before the question.
type Request struct {
	Question  string
	Options   []Option
	Context   []string
	Threshold float64 // below this top probability the result is Unsure
}

// Result holds a probability per option id, renormalised over the options.
// Mass is how much of the model's probability fell on the options at all.
type Result struct {
	Probs  map[string]float64
	Choice string
	Unsure bool
	Mass   float64
}

// Client talks to an OpenAI-compatible chat endpoint, such as llama-server.
type Client struct {
	BaseURL string // without /v1
	Model   string
	HTTP    *http.Client // nil uses http.DefaultClient
}

// Decide asks the model once and returns its probabilities.
func (c *Client) Decide(ctx context.Context, r Request) (Result, error) {
	if len(r.Options) < 2 || len(r.Options) > len(keys) {
		return Result{}, fmt.Errorf("decide: need 2 to %d options, got %d", len(keys), len(r.Options))
	}
	body, err := json.Marshal(map[string]any{
		"model": c.Model,
		"messages": []map[string]string{
			{"role": "system", "content": "You make one decision and answer with a single letter."},
			{"role": "user", "content": prompt(r)},
		},
		"max_tokens":           1,
		"temperature":          0,
		"logprobs":             true,
		"top_logprobs":         20,
		"chat_template_kwargs": map[string]bool{"enable_thinking": false},
	})
	if err != nil {
		return Result{}, fmt.Errorf("decide: encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Result{}, fmt.Errorf("decide: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	httpc := c.HTTP
	if httpc == nil {
		httpc = http.DefaultClient
	}
	resp, err := httpc.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("decide: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return Result{}, fmt.Errorf("decide: server answered %s: %s", resp.Status, bytes.TrimSpace(msg))
	}
	var out struct {
		Choices []struct {
			Logprobs struct {
				Content []struct {
					TopLogprobs []token `json:"top_logprobs"`
				} `json:"content"`
			} `json:"logprobs"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Result{}, fmt.Errorf("decide: decode answer: %w", err)
	}
	if len(out.Choices) == 0 || len(out.Choices[0].Logprobs.Content) == 0 {
		return Result{}, fmt.Errorf("decide: answer has no log-probabilities")
	}
	res := read(r, out.Choices[0].Logprobs.Content[0].TopLogprobs)
	if res.Mass == 0 {
		return res, ErrNoOption
	}
	return res, nil
}

// token is one candidate for the answer's first token.
type token struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
}

// prompt lists the context, the question and the options with their keys.
func prompt(r Request) string {
	var b strings.Builder
	for _, c := range r.Context {
		b.WriteString(c)
		b.WriteString("\n")
	}
	b.WriteString("\n---\n")
	b.WriteString(r.Question)
	b.WriteString("\n")
	for i, o := range r.Options {
		fmt.Fprintf(&b, "%c) %s: %s\n", keys[i], o.ID, o.Description)
	}
	b.WriteString("Answer with the letter only.")
	return b.String()
}

// read turns the top tokens into option probabilities. A key may appear as
// several tokens (" A", "A)"); the likeliest one counts, whatever the order.
func read(r Request, top []token) Result {
	p := map[string]float64{}
	for _, t := range top {
		k := strings.TrimRight(strings.TrimSpace(t.Token), ")")
		i := strings.Index(keys[:len(r.Options)], k)
		if len(k) != 1 || i < 0 {
			continue
		}
		p[r.Options[i].ID] = max(p[r.Options[i].ID], math.Exp(t.Logprob))
	}
	res := Result{Probs: map[string]float64{}}
	for _, v := range p {
		res.Mass += v
	}
	best := -1.0
	for _, o := range r.Options {
		v := 0.0
		if res.Mass > 0 {
			v = p[o.ID] / res.Mass
		}
		res.Probs[o.ID] = v
		if v > best {
			best, res.Choice = v, o.ID
		}
	}
	if res.Mass == 0 {
		res.Choice = ""
	}
	res.Unsure = res.Mass == 0 || best < r.Threshold
	return res
}

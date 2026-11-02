package main

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// answering returns a server whose first token is always key with p.
func answering(t *testing.T, key string, p float64) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"logprobs": map[string]any{
			"content": []any{map[string]any{"top_logprobs": []any{map[string]any{"token": key, "logprob": math.Log(p)}}}}}}}})
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func pairsFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pairs.jsonl")
	data := `{"id":"1","old":"PRIVATE ACME ships on Monday","new":"PRIVATE ACME ships on Friday","label":"conflicts"}
{"id":"2","old":"PRIVATE Globex has 2,300 employees","new":"PRIVATE OrderApp uses Redis","label":"unrelated"}
`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestScorePrintsThreeJudgesWithoutText(t *testing.T) {
	var out bytes.Buffer
	results := filepath.Join(t.TempDir(), "results.jsonl")
	err := run([]string{"score", "-pairs", pairsFile(t), "-url", answering(t, "A", 0.9), "-model", "m", "-out", results}, &out)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "model n=2 errors=0 accuracy=0.50") ||
		!strings.HasPrefix(lines[1], "constant ") || !strings.HasPrefix(lines[2], "overlap(cut=") {
		t.Fatalf("output:\n%s", out.String())
	}
	if !strings.Contains(lines[0], "confident.precision=0.50 confident.coverage=1.00") {
		t.Fatalf("model line: %s", lines[0])
	}
	saved, err := os.ReadFile(results)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{out.String(), string(saved)} {
		if strings.Contains(text, "PRIVATE") {
			t.Fatalf("claim text leaked: %s", text)
		}
	}
}

func TestScoreRefusesResultsInsideTheRepository(t *testing.T) {
	inside := filepath.Join(".", "results-must-not-exist.jsonl")
	err := run([]string{"score", "-pairs", pairsFile(t), "-url", answering(t, "A", 0.9), "-model", "m", "-out", inside}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "inside this repository") {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(inside); !os.IsNotExist(statErr) {
		os.Remove(inside)
		t.Fatal("the file was created")
	}
}

func TestScoreNeedsItsFlags(t *testing.T) {
	if err := run([]string{"score", "-pairs", "x"}, &bytes.Buffer{}); err == nil {
		t.Fatal("missing -url and -model were accepted")
	}
}

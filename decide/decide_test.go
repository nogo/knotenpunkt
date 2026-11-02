package decide

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

var verdict = []Option{
	{ID: "conflicts", Description: "both cannot be true"},
	{ID: "agrees", Description: "same fact"},
	{ID: "unrelated", Description: "different facts"},
}

// fakeServer answers every request with the given top tokens and keeps the
// last request body.
func fakeServer(t *testing.T, top map[string]float64, status int) (*Client, *map[string]any) {
	t.Helper()
	var last map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		json.NewDecoder(r.Body).Decode(&last)
		if status != http.StatusOK {
			http.Error(w, "model not loaded", status)
			return
		}
		var tokens []map[string]any
		for tok, p := range top {
			tokens = append(tokens, map[string]any{"token": tok, "logprob": math.Log(p)})
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{
			"logprobs": map[string]any{"content": []any{map[string]any{"top_logprobs": tokens}}}}}})
	}))
	t.Cleanup(srv.Close)
	return &Client{BaseURL: srv.URL + "/", Model: "qwen-moe"}, &last
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestProbabilitiesMapToOptionsAndRenormalise(t *testing.T) {
	c, last := fakeServer(t, map[string]float64{"A": 0.6, " B": 0.2, "C)": 0.1, "Z": 0.05}, http.StatusOK)
	res, err := c.Decide(context.Background(), Request{Question: "Does the new claim conflict?", Options: verdict,
		Context: []string{"Old: ACME ships on Monday", "New: ACME ships on Friday"}, Threshold: 0.5})
	if err != nil {
		t.Fatal(err)
	}
	if !near(res.Mass, 0.9) || !near(res.Probs["conflicts"], 0.6/0.9) || !near(res.Probs["agrees"], 0.2/0.9) || !near(res.Probs["unrelated"], 0.1/0.9) {
		t.Fatalf("result = %+v", res)
	}
	if res.Choice != "conflicts" || res.Unsure {
		t.Fatalf("choice = %q unsure = %v, want conflicts and sure", res.Choice, res.Unsure)
	}
	msgs := (*last)["messages"].([]any)
	user := msgs[1].(map[string]any)["content"].(string)
	for _, want := range []string{"Old: ACME ships on Monday", "A) conflicts: both cannot be true", "C) unrelated", "Answer with the letter only."} {
		if !strings.Contains(user, want) {
			t.Errorf("prompt lacks %q:\n%s", want, user)
		}
	}
	if (*last)["max_tokens"].(float64) != 1 || (*last)["model"] != "qwen-moe" || (*last)["logprobs"] != true {
		t.Errorf("request = %v", *last)
	}
}

func TestTheLikeliestTokenOfAKeyCounts(t *testing.T) {
	c, _ := fakeServer(t, map[string]float64{"B": 0.5, " B": 0.3, "A": 0.2}, http.StatusOK)
	res, err := c.Decide(context.Background(), Request{Options: verdict})
	if err != nil {
		t.Fatal(err)
	}
	if !near(res.Probs["agrees"], 0.5/0.7) {
		t.Fatalf("agrees = %v, want the likelier B token only", res.Probs["agrees"])
	}
}

func TestBelowTheThresholdIsUnsure(t *testing.T) {
	c, _ := fakeServer(t, map[string]float64{"A": 0.4, "B": 0.35, "C": 0.25}, http.StatusOK)
	res, err := c.Decide(context.Background(), Request{Options: verdict, Threshold: 0.7})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Unsure || res.Choice != "conflicts" {
		t.Fatalf("result = %+v, want conflicts but unsure", res)
	}
}

func TestAMissingKeyGetsZero(t *testing.T) {
	c, _ := fakeServer(t, map[string]float64{"A": 0.7, "C": 0.1}, http.StatusOK)
	res, err := c.Decide(context.Background(), Request{Options: verdict})
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := res.Probs["agrees"]; !ok || p != 0 {
		t.Fatalf("agrees = %v, %v, want 0 and present", p, ok)
	}
}

func TestNoOptionTokenIsAnError(t *testing.T) {
	c, _ := fakeServer(t, map[string]float64{"The": 0.9, "Yes": 0.1}, http.StatusOK)
	res, err := c.Decide(context.Background(), Request{Options: verdict})
	if !errors.Is(err, ErrNoOption) || !res.Unsure || res.Choice != "" {
		t.Fatalf("res = %+v, err = %v, want ErrNoOption", res, err)
	}
}

func TestHTTPErrorIsReturnedNotScored(t *testing.T) {
	c, _ := fakeServer(t, nil, http.StatusServiceUnavailable)
	if _, err := c.Decide(context.Background(), Request{Options: verdict}); err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("err = %v, want the 503", err)
	}
}

func TestOptionCountIsChecked(t *testing.T) {
	c, _ := fakeServer(t, map[string]float64{"A": 1}, http.StatusOK)
	if _, err := c.Decide(context.Background(), Request{Options: verdict[:1]}); err == nil {
		t.Fatal("one option was accepted")
	}
}

// TestLiveDecide scores three invented pairs on a real server.
// KNOTENPUNKT_LIVE=1 KNOTENPUNKT_URL=http://localhost:9481 KNOTENPUNKT_MODEL=qwen-moe
func TestLiveDecide(t *testing.T) {
	if os.Getenv("KNOTENPUNKT_LIVE") != "1" {
		t.Skip("set KNOTENPUNKT_LIVE=1, KNOTENPUNKT_URL and KNOTENPUNKT_MODEL")
	}
	c := &Client{BaseURL: os.Getenv("KNOTENPUNKT_URL"), Model: os.Getenv("KNOTENPUNKT_MODEL")}
	for _, p := range []struct{ old, new string }{
		{"ACME ships every Monday from Hamburg.", "ACME ships every Friday from Hamburg."},
		{"OrderApp keeps guest carts for 24 hours.", "Guest carts in OrderApp are kept for one day."},
		{"Globex has 2,300 employees.", "The OrderApp checkout uses Redis."},
	} {
		res, err := c.Decide(context.Background(), Request{Question: "Does the new claim conflict with the old one?",
			Options: verdict, Context: []string{"Old: " + p.old, "New: " + p.new}, Threshold: 0.7})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%-10s mass=%.2f probs=%v", res.Choice, res.Mass, res.Probs)
	}
}

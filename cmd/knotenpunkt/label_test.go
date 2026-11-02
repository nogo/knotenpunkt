package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nogo/knotenpunkt/internal/score"
)

func unlabelled(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "unlabelled.jsonl")
	var b strings.Builder
	for _, id := range []string{"h1", "h2", "h3", "h4"} {
		b.WriteString(`{"id":"` + id + `","old":"ACME ships on Monday ` + id + `","new":"ACME ships on Friday ` + id + `","why":"small-flip"}` + "\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func labelWith(t *testing.T, in, out, answers string) string {
	t.Helper()
	stdin = strings.NewReader(answers)
	t.Cleanup(func() { stdin = os.Stdin })
	var screen bytes.Buffer
	if err := run([]string{"label", "-in", in, "-out", out}, &screen); err != nil {
		t.Fatal(err)
	}
	return screen.String()
}

func TestLabelWritesEachAnswerAndResumes(t *testing.T) {
	in := unlabelled(t)
	out := filepath.Join(t.TempDir(), "pairs.jsonl")
	screen := labelWith(t, in, out, "c\ns\nx\na\nq\n")
	if !strings.Contains(screen, "c, a, u, s or q") || strings.Contains(screen, "small-flip") {
		t.Fatalf("screen:\n%s", screen)
	}
	first, err := score.ReadPairs(out)
	if err != nil || len(first) != 2 || first[0].Label != "conflicts" || first[1].Label != "agrees" {
		t.Fatalf("after quitting: %+v, %v", first, err)
	}
	labelWith(t, in, out, "u\nu\nu\n")
	all, err := score.ReadPairs(out)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, p := range all {
		seen[p.ID]++
	}
	if len(all) != 4 || len(seen) != 4 {
		t.Fatalf("after resuming: %d pairs, ids %v; want the skipped one and the two unseen, each once", len(all), seen)
	}
}

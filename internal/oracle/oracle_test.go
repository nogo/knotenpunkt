package oracle

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// inventedVault makes a log containing only invented claims and pages.
func inventedVault(t *testing.T) string {
	t.Helper()
	vault := t.TempDir()
	dir := filepath.Join(vault, "log", "2026-10")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	log := `{"type":"claim","id":"a","text":"ACME uses red","author":{"kind":"user","id":"u1"},"source":{"page":"ACME"}}
{"type":"claim","id":"b","author":{"kind":"session","id":"s1"},"text":"ACME uses blue","supersedes":["a"],"source":{"page":"ACME"}}
{"type":"claim_decision","id":"z1","claim":"b","confirm":true,"author":{"kind":"user","id":"u1"}}
{"type":"claim","id":"c","text":" acme USES   blue ","author":{"kind":"user","id":"u1"},"supersedes":["b"],"source":{"page":"ACME"}}
{"type":"claim","id":"d","text":"OrderApp is open","author":{"kind":"user","id":"u1"},"source":{"page":"OrderApp"}}
{"type":"claim","id":"e","text":"OrderApp is closed","author":{"kind":"user","id":"u1"},"challenges":["d"],"source":{"page":"OrderApp"}}
{"type":"claim","id":"f","text":"Jane Roe can approve","author":{"kind":"user","id":"u1"},"source":{"page":"Approvals"}}
{"type":"claim","id":"g","text":"Retraction marker","author":{"kind":"user","id":"u1"},"supersedes":["f"],"retracted":true}
{"type":"claim","id":"h","author":{"kind":"session","id":"s1"},"text":"SECRET PROPOSAL","supersedes":["c"],"source":{"page":"Draft"}}
{"type":"claim","id":"i","text":"ACME ships weekly","author":{"kind":"user","id":"u1"},"source":{"page":"ACME"}}
{"type":"claim","id":"j","text":"OrderApp uses queues","author":{"kind":"user","id":"u1"},"source":{"page":"Queues"}}
{"type":"claim","id":"k","author":{"kind":"session","id":"s1"},"text":"SECRET REJECTED","supersedes":["d"]}
{"type":"claim_decision","id":"z2","claim":"k","confirm":false,"author":{"kind":"user","id":"u1"}}
{"type":"claim_decision","id":"z3","claim":"k","confirm":true,"author":{"kind":"user","id":"u1"}}
{"type":"page","text":{"ignored":true}}
`
	if err := os.WriteFile(filepath.Join(dir, "01.jsonl"), []byte(log), 0600); err != nil {
		t.Fatal(err)
	}
	return vault
}

// TestBuildPairs proves folding, labels, exclusions, privacy, and reproducibility together.
func TestBuildPairs(t *testing.T) {
	vault := inventedVault(t)
	before := vaultFiles(t, vault)
	out := filepath.Join(t.TempDir(), "oracle.jsonl")
	counts, err := Build(vault, out, 7)
	if err != nil {
		t.Fatal(err)
	}
	if counts.Claims != 11 || counts.Confirmed != 9 || counts.Live != 5 || counts.Conflicts != 2 || counts.Unrelated != 2 || counts.TrustOnly != 1 || counts.Retractions != 1 || counts.Unconfirmed != 2 {
		t.Fatalf("unexpected counts: %+v", counts)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var pairs []Pair
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		var p Pair
		if err := json.Unmarshal(line, &p); err != nil {
			t.Fatal(err)
		}
		pairs = append(pairs, p)
	}
	if pairs[0].Old != "ACME uses red" || pairs[0].New != "ACME uses blue" || pairs[0].Label != "conflicts" || pairs[0].Why != "supersedes" {
		t.Fatalf("wrong superseding pair: %+v", pairs[0])
	}
	if pairs[1].Why != "challenges" {
		t.Fatalf("missing challenge: %+v", pairs[1])
	}
	claims, confirmed, err := readVault(vault, new(Counts))
	if err != nil {
		t.Fatal(err)
	}
	byText := map[string]claim{}
	for _, c := range claims {
		byText[c.Text] = c
	}
	for _, p := range pairs {
		if strings.Contains(p.Old+p.New, "SECRET") || strings.Contains(p.Old+p.New, "Retraction") {
			t.Fatal("excluded claim in pair")
		}
		if p.Label != "unrelated" {
			continue
		}
		a, b := byText[p.Old], byText[p.New]
		if !confirmed[a.ID] || !confirmed[b.ID] || linked(a, b) || a.Source.Page == b.Source.Page {
			t.Fatalf("invalid unrelated pair: %+v", p)
		}
		if a.ID == "a" || a.ID == "b" || a.ID == "f" || a.ID == "g" || b.ID == "a" || b.ID == "b" || b.ID == "f" || b.ID == "g" {
			t.Fatal("dead claim sampled")
		}
	}
	second := filepath.Join(t.TempDir(), "oracle.jsonl")
	if _, err := Build(vault, second, 7); err != nil {
		t.Fatal(err)
	}
	again, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, again) {
		t.Fatal("output changed for same snapshot and seed")
	}
	after := vaultFiles(t, vault)
	if len(before) != len(after) {
		t.Fatal("vault files changed")
	}
	for path, content := range before {
		if !bytes.Equal(content, after[path]) {
			t.Fatal("vault contents changed")
		}
	}
	if counts.Linked == 0 || counts.SharedPage == 0 || counts.NotSampled == 0 {
		t.Fatal("unrelated exclusions not exercised")
	}
	var printed bytes.Buffer
	if err := counts.Print(&printed); err != nil {
		t.Fatal(err)
	}
	for _, c := range claims {
		if strings.Contains(printed.String(), c.Text) || (c.Source.Page != "" && strings.Contains(printed.String(), c.Source.Page)) {
			t.Fatal("private data printed")
		}
	}
	info, err := os.Stat(out)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("output is not private")
	}
}

// vaultFiles captures all file bytes to detect any writes inside the vault.
func vaultFiles(t *testing.T, vault string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	err := filepath.WalkDir(vault, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[path] = data
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// TestUnrelatedShortfall reports scarcity without making invalid pairs.
func TestUnrelatedShortfall(t *testing.T) {
	claims := map[string]claim{"a": {ID: "a", Text: "ACME uses red"}, "b": {ID: "b", Text: "ACME uses blue", Supersedes: []string{"a"}}}
	var counts Counts
	pairs := makePairs(claims, map[string]bool{"a": true, "b": true}, 1, &counts)
	if len(pairs) != 1 || counts.Conflicts != 1 || counts.Unrelated != 0 || counts.Shortfall != 1 {
		t.Fatalf("wrong shortfall: %+v", counts)
	}
}

// TestProtectedPaths rejects vault and repository paths, including symlink aliases.
func TestProtectedPaths(t *testing.T) {
	vault, repo := inventedVault(t), t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(vault, alias); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{vault, repo, alias} {
		out := filepath.Join(dir, "pairs.jsonl")
		if _, err := Build(vault, out, 1, repo); err == nil {
			t.Fatal("protected output accepted")
		}
		if _, err := os.Stat(out); !os.IsNotExist(err) {
			t.Fatal("protected output created")
		}
	}
}

// TestPartialAppend omits an unfinished append and leaves every vault byte unchanged.
func TestPartialAppend(t *testing.T) {
	vault := inventedVault(t)
	path := filepath.Join(vault, "log", "2026-10", "02.jsonl")
	data := []byte(`{"type":"claim","id":"unfinished","text":"PRIVATE`)
	if err := os.WriteFile(path, data, 0400); err != nil {
		t.Fatal(err)
	}
	counts, err := Build(vault, filepath.Join(t.TempDir(), "pairs.jsonl"), 1)
	if err != nil {
		t.Fatal(err)
	}
	if counts.Partial != 1 {
		t.Fatal("partial append not counted")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, after) {
		t.Fatal("log changed")
	}
}

// TestMalformedRecordDoesNotLeak checks that malformed private data stays out of errors.
func TestMalformedRecordDoesNotLeak(t *testing.T) {
	vault := inventedVault(t)
	path := filepath.Join(vault, "log", "2026-10", "02.jsonl")
	if err := os.WriteFile(path, []byte("PRIVATE INVALID RECORD\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Build(vault, filepath.Join(t.TempDir(), "pairs.jsonl"), 1)
	if err == nil || strings.Contains(err.Error(), "PRIVATE") || strings.Contains(err.Error(), vault) {
		t.Fatal("missing or unsafe error")
	}
}

// TestConcurrentAppend proves the reader finishes while another writer keeps the log open.
func TestConcurrentAppend(t *testing.T) {
	vault := inventedVault(t)
	path := filepath.Join(vault, "log", "2026-10", "02.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	started, stop, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		defer f.Close()
		for i := 0; ; i++ {
			if _, err := f.WriteString("{\"type\":\"page\"}\n"); err != nil {
				done <- err
				return
			}
			if i == 0 {
				close(started)
			}
			select {
			case <-stop:
				done <- nil
				return
			default:
			}
		}
	}()
	<-started
	counts, buildErr := Build(vault, filepath.Join(t.TempDir(), "pairs.jsonl"), 1)
	close(stop)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	if counts.Claims != 11 {
		t.Fatalf("lost claims: %+v", counts)
	}
}

// TestExistingOutputPreserved prevents accidental replacement of a frozen oracle.
func TestExistingOutputPreserved(t *testing.T) {
	out := filepath.Join(t.TempDir(), "pairs.jsonl")
	if err := os.WriteFile(out, []byte("frozen"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(inventedVault(t), out, 1); err == nil {
		t.Fatal("existing output accepted")
	}
	data, err := os.ReadFile(out)
	if err != nil || string(data) != "frozen" {
		t.Fatal("existing oracle changed")
	}
}

// TestSidenoteDecisionRules follows sidenote's fold: only the user decides,
// the first decision in id order wins, and one claim may supersede several.
func TestSidenoteDecisionRules(t *testing.T) {
	vault := t.TempDir()
	dir := filepath.Join(vault, "log", "m1")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	log := `{"type":"claim","id":"a","text":"ACME ships on Monday","author":{"kind":"user","id":"u1"},"source":{"page":"ACME"}}
{"type":"claim","id":"b","text":"ACME ships on Friday","author":{"kind":"user","id":"u1"},"source":{"page":"ACME"}}
{"type":"claim","id":"c","text":"ACME ships on Sunday","author":{"kind":"session","id":"s1"},"supersedes":["a","b"]}
{"type":"claim","id":"d","text":"OrderApp runs nightly","author":{"kind":"session","id":"s1"},"supersedes":["a"]}
{"type":"claim_decision","id":"y2","claim":"c","confirm":false,"author":{"kind":"user","id":"u1"}}
{"type":"claim_decision","id":"y1","claim":"c","confirm":true,"author":{"kind":"user","id":"u1"}}
{"type":"claim_decision","id":"y3","claim":"d","confirm":true,"author":{"kind":"session","id":"s1"}}
`
	if err := os.WriteFile(filepath.Join(dir, "2026-10-01.jsonl"), []byte(log), 0600); err != nil {
		t.Fatal(err)
	}
	claims, confirmed, err := readVault(vault, new(Counts))
	if err != nil {
		t.Fatal(err)
	}
	if !confirmed["c"] {
		t.Fatal("the first decision in id order (y1, confirm) must win over y2")
	}
	if confirmed["d"] {
		t.Fatal("a decision not made by the user must not confirm")
	}
	var counts Counts
	pairs := makePairs(claims, confirmed, 1, &counts)
	if counts.Conflicts != 2 || pairs[0].Old != "ACME ships on Monday" || pairs[1].Old != "ACME ships on Friday" {
		t.Fatalf("a claim superseding two claims must give two conflicts: %+v %+v", counts, pairs)
	}
}

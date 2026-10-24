// Package oracle builds frozen claim pairs from sidenote logs without changing the vault.
package oracle

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type claim struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	Author     string `json:"author"`
	Supersedes string `json:"supersedes"`
	Challenges string `json:"challenges"`
	Retracted  bool   `json:"retracted"`
	Source     struct {
		Page string `json:"page"`
	} `json:"source"`
}

// Pair is one labelled example; its identifier reveals neither claim identifier.
type Pair struct {
	ID    string `json:"id"`
	Old   string `json:"old"`
	New   string `json:"new"`
	Label string `json:"label"`
	Why   string `json:"why"`
}

// Counts contains only quantities safe to report outside the vault.
type Counts struct {
	Claims, Confirmed, Live                                                                          int
	Conflicts, Unrelated                                                                             int
	TrustOnly, Retractions, Unconfirmed, Missing, Linked, SharedPage, NotSampled, Shortfall, Partial int
}

// Print reports quantities without exposing paths or record contents.
func (c Counts) Print(w io.Writer) error {
	_, err := fmt.Fprintf(w, "claims_read=%d confirmed=%d live=%d conflicts=%d unrelated=%d left_out_trust_only=%d left_out_retractions=%d left_out_unconfirmed=%d left_out_missing=%d left_out_linked=%d left_out_shared_page=%d left_out_not_sampled=%d left_out_shortfall=%d left_out_partial_records=%d\n", c.Claims, c.Confirmed, c.Live, c.Conflicts, c.Unrelated, c.TrustOnly, c.Retractions, c.Unconfirmed, c.Missing, c.Linked, c.SharedPage, c.NotSampled, c.Shortfall, c.Partial)
	return err
}

type snapshot struct {
	file *os.File
	size int64
}

// Build reads a bounded log snapshot and creates a private, new output file.
// Repository roots come from the command so this package need not locate its caller.
func Build(vault, out string, seed int64, repositories ...string) (Counts, error) {
	var counts Counts
	if err := validateOutput(vault, out, repositories); err != nil {
		return counts, err
	}
	claims, confirmed, err := readVault(vault, &counts)
	if err != nil {
		return counts, err
	}
	pairs := makePairs(claims, confirmed, seed, &counts)
	f, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return counts, fmt.Errorf("oracle: cannot create output")
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(out)
		}
	}()
	encoder := json.NewEncoder(f)
	for _, pair := range pairs {
		if err := encoder.Encode(pair); err != nil {
			_ = f.Close()
			return counts, fmt.Errorf("oracle: cannot write output")
		}
	}
	if err := f.Close(); err != nil {
		return counts, fmt.Errorf("oracle: cannot close output")
	}
	ok = true
	return counts, nil
}

// validateOutput resolves symlinks before checking containment, including new files.
func validateOutput(vault, out string, repositories []string) error {
	root, err := filepath.EvalSymlinks(vault)
	if err != nil {
		return fmt.Errorf("oracle: cannot resolve vault")
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("oracle: cannot resolve vault")
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return fmt.Errorf("oracle: vault is not a directory")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(out))
	if err != nil {
		return fmt.Errorf("oracle: cannot resolve output directory")
	}
	target, err := filepath.Abs(filepath.Join(parent, filepath.Base(out)))
	if err != nil {
		return fmt.Errorf("oracle: cannot resolve output")
	}
	for _, forbidden := range append([]string{root}, repositories...) {
		resolved, err := filepath.EvalSymlinks(forbidden)
		if err != nil {
			return fmt.Errorf("oracle: cannot resolve protected directory")
		}
		resolved, err = filepath.Abs(resolved)
		if err != nil {
			return fmt.Errorf("oracle: cannot resolve protected directory")
		}
		rel, err := filepath.Rel(resolved, target)
		if err != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return fmt.Errorf("oracle: output is inside a protected directory")
		}
	}
	return nil
}

// readVault opens all files and records their lengths before consuming the snapshot.
func readVault(vault string, counts *Counts) (map[string]claim, map[string]bool, error) {
	paths, err := filepath.Glob(filepath.Join(vault, "log", "*", "*.jsonl"))
	if err != nil {
		return nil, nil, fmt.Errorf("oracle: cannot list log")
	}
	var files []snapshot
	defer func() {
		for _, s := range files {
			_ = s.file.Close()
		}
	}()
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			return nil, nil, fmt.Errorf("oracle: cannot open log")
		}
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() {
			_ = f.Close()
			return nil, nil, fmt.Errorf("oracle: cannot inspect log")
		}
		files = append(files, snapshot{f, info.Size()})
	}
	claims := make(map[string]claim)
	decisions := make(map[string]bool)
	for _, s := range files {
		r := bufio.NewReader(io.NewSectionReader(s.file, 0, s.size))
		for {
			line, err := r.ReadBytes('\n')
			if err == io.EOF {
				if len(line) > 0 {
					counts.Partial++
				}
				break
			}
			if err != nil {
				return nil, nil, fmt.Errorf("oracle: cannot read log")
			}
			if len(strings.TrimSpace(string(line))) == 0 {
				continue
			}
			var header struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(line, &header) != nil {
				return nil, nil, fmt.Errorf("oracle: malformed log record")
			}
			switch header.Type {
			case "claim":
				var c claim
				if json.Unmarshal(line, &c) != nil || c.ID == "" {
					return nil, nil, fmt.Errorf("oracle: malformed claim")
				}
				counts.Claims++
				claims[c.ID] = c
			case "claim_decision":
				var d struct {
					Claim   string `json:"claim"`
					Confirm bool   `json:"confirm"`
				}
				if json.Unmarshal(line, &d) != nil || d.Claim == "" {
					return nil, nil, fmt.Errorf("oracle: malformed decision")
				}
				decisions[d.Claim] = d.Confirm
			}
		}
	}
	confirmed := make(map[string]bool)
	for id, c := range claims {
		confirmed[id] = c.Author == "user" || decisions[id]
	}
	return claims, confirmed, nil
}

// makePairs keeps historical conflicts and samples eligible live pairs with a fixed seed.
func makePairs(claims map[string]claim, confirmed map[string]bool, seed int64, counts *Counts) []Pair {
	ids := make([]string, 0, len(claims))
	dead := make(map[string]bool)
	for id, c := range claims {
		ids = append(ids, id)
		if confirmed[id] {
			counts.Confirmed++
			if c.Retracted {
				dead[id] = true
			}
			if c.Supersedes != "" {
				dead[c.Supersedes] = true
			}
		}
	}
	sort.Strings(ids)
	var pairs []Pair
	seen := make(map[string]bool)
	for _, id := range ids {
		c := claims[id]
		for _, link := range []struct{ target, rule string }{{c.Supersedes, "supersedes"}, {c.Challenges, "challenges"}} {
			if link.target == "" {
				continue
			}
			old, exists := claims[link.target]
			switch {
			case !exists:
				counts.Missing++
			case !confirmed[id] || !confirmed[old.ID]:
				counts.Unconfirmed++
			case c.Retracted || old.Retracted:
				counts.Retractions++
			case link.rule == "supersedes" && normalize(old.Text) == normalize(c.Text):
				counts.TrustOnly++
			case old.ID != id:
				key := pairKey(old.ID, id)
				if !seen[key] {
					pairs = append(pairs, newPair(old, c, "conflicts", link.rule))
					seen[key] = true
				}
			}
		}
	}
	counts.Conflicts = len(pairs)
	var live []claim
	for _, id := range ids {
		if confirmed[id] && !dead[id] {
			live = append(live, claims[id])
		}
	}
	counts.Live = len(live)
	rng := rand.New(rand.NewSource(seed))
	var unrelated []Pair
	eligible := 0
	for i, a := range live {
		for _, b := range live[i+1:] {
			if linked(a, b) {
				counts.Linked++
				continue
			}
			if a.Source.Page == b.Source.Page {
				counts.SharedPage++
				continue
			}
			eligible++
			if len(unrelated) < counts.Conflicts {
				unrelated = append(unrelated, newPair(a, b, "unrelated", "live_no_link_different_page"))
				continue
			}
			if j := rng.Intn(eligible); j < len(unrelated) {
				unrelated[j] = newPair(a, b, "unrelated", "live_no_link_different_page")
			}
		}
	}
	counts.Unrelated = len(unrelated)
	counts.NotSampled = eligible - counts.Unrelated
	counts.Shortfall = counts.Conflicts - counts.Unrelated
	return append(pairs, unrelated...)
}

// normalize ignores case and all whitespace differences for trust-only revisions.
func normalize(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), "")) }

// linked checks both directions because a log link has only one author.
func linked(a, b claim) bool {
	return a.Supersedes == b.ID || a.Challenges == b.ID || b.Supersedes == a.ID || b.Challenges == a.ID
}

// pairKey identifies an unordered pair without delimiter ambiguity.
func pairKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return fmt.Sprintf("%d:%s%s", len(a), a, b)
}

// newPair hashes identifiers so output identifiers do not expose vault metadata.
func newPair(a, b claim, label, why string) Pair {
	id := sha256.Sum256([]byte(label + ":" + pairKey(a.ID, b.ID)))
	return Pair{fmt.Sprintf("%x", id), a.Text, b.Text, label, why}
}

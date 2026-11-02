// Package score measures a judge on labelled claim pairs against two free
// baselines: always the most common label, and word overlap. It prints
// counts and scores only, never a claim's text.
package score

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode"
)

// Positive is the label whose precision matters most: a false "conflicts"
// alarm costs the owner's trust in the check.
const Positive = "conflicts"

// Labels are the options, in the order the judge is asked.
var Labels = []string{"conflicts", "agrees", "unrelated"}

// Pair is one line of the oracle's output.
type Pair struct {
	ID    string `json:"id"`
	Old   string `json:"old"`
	New   string `json:"new"`
	Label string `json:"label"`
	Why   string `json:"why"`
}

// Prediction is one answer. Confident means the top probability reached the
// threshold. Err is set when the judge gave no answer; such a pair counts as
// wrong and is never confident.
type Prediction struct {
	ID        string             `json:"id"`
	Label     string             `json:"label"`
	Pred      string             `json:"pred"`
	Probs     map[string]float64 `json:"probs,omitempty"`
	Confident bool               `json:"confident"`
	MS        float64            `json:"ms,omitempty"`
	Mass      float64            `json:"mass,omitempty"`
	Err       string             `json:"err,omitempty"`
}

// ReadPairs reads a pairs file. Unknown labels are refused.
func ReadPairs(path string) ([]Pair, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("score: %w", err)
	}
	defer f.Close()
	var out []Pair
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	for n := 1; sc.Scan(); n++ {
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		var p Pair
		if err := json.Unmarshal(sc.Bytes(), &p); err != nil {
			return nil, fmt.Errorf("score: pairs line %d is not JSON", n)
		}
		if !known(p.Label) {
			return nil, fmt.Errorf("score: pairs line %d has an unknown label", n)
		}
		out = append(out, p)
	}
	return out, sc.Err()
}

func known(label string) bool {
	for _, l := range Labels {
		if l == label {
			return true
		}
	}
	return false
}

// Constant always answers the most common label; ties go to the earlier label.
func Constant(pairs []Pair) []Prediction {
	count := map[string]int{}
	for _, p := range pairs {
		count[p.Label]++
	}
	best := Labels[0]
	for _, l := range Labels {
		if count[l] > count[best] {
			best = l
		}
	}
	out := make([]Prediction, len(pairs))
	for i, p := range pairs {
		out[i] = Prediction{ID: p.ID, Label: p.Label, Pred: best, Confident: true}
	}
	return out
}

// Overlap answers Positive when the Jaccard overlap of the two claims' words
// reaches a cut-off, and "unrelated" below it. The cut-off is the one that
// scores best on these same pairs, which flatters the baseline on purpose.
func Overlap(pairs []Pair) ([]Prediction, float64) {
	j := make([]float64, len(pairs))
	cuts := []float64{1.01}
	for i, p := range pairs {
		j[i] = jaccard(p.Old, p.New)
		cuts = append(cuts, j[i])
	}
	sort.Float64s(cuts)
	predict := func(cut float64) []Prediction {
		out := make([]Prediction, len(pairs))
		for i, p := range pairs {
			pred := "unrelated"
			if j[i] >= cut {
				pred = Positive
			}
			out[i] = Prediction{ID: p.ID, Label: p.Label, Pred: pred, Confident: true}
		}
		return out
	}
	bestCut, bestRight := cuts[0], -1
	for _, cut := range cuts {
		right := 0
		for _, pr := range predict(cut) {
			if pr.Pred == pr.Label {
				right++
			}
		}
		if right > bestRight {
			bestCut, bestRight = cut, right
		}
	}
	return predict(bestCut), bestCut
}

func jaccard(a, b string) float64 {
	wa, wb := words(a), words(b)
	inter := 0
	for w := range wa {
		if wb[w] {
			inter++
		}
	}
	union := len(wa) + len(wb) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func words(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }) {
		out[w] = true
	}
	return out
}

// Metrics is what Report prints for one judge.
type Metrics struct {
	N, Errors                 int
	Accuracy                  float64
	Precision, Recall         map[string]float64
	ConfidentPrecision        float64 // of Positive, among confident answers
	ConfidentCoverage         float64 // share of true Positive pairs flagged confidently
	MedianMS, P95MS, MeanMass float64
}

// Measure computes Metrics. A precision with nothing predicted is 0.
func Measure(preds []Prediction) Metrics {
	m := Metrics{N: len(preds), Precision: map[string]float64{}, Recall: map[string]float64{}}
	predicted, actual, right := map[string]int{}, map[string]int{}, map[string]int{}
	confFlag, confRight, ok := 0, 0, 0
	var ms, mass []float64
	for _, p := range preds {
		actual[p.Label]++
		if p.Err != "" {
			m.Errors++
			continue
		}
		predicted[p.Pred]++
		if p.Pred == p.Label {
			right[p.Label]++
			ok++
		}
		if p.Confident && p.Pred == Positive {
			confFlag++
			if p.Label == Positive {
				confRight++
			}
		}
		if p.MS > 0 {
			ms = append(ms, p.MS)
		}
		if p.Mass > 0 {
			mass = append(mass, p.Mass)
		}
	}
	m.Accuracy = ratio(ok, m.N)
	for _, l := range Labels {
		m.Precision[l] = ratio(right[l], predicted[l])
		m.Recall[l] = ratio(right[l], actual[l])
	}
	m.ConfidentPrecision = ratio(confRight, confFlag)
	m.ConfidentCoverage = ratio(confRight, actual[Positive])
	m.MedianMS, m.P95MS = quantile(ms, 0.5), quantile(ms, 0.95)
	for _, v := range mass {
		m.MeanMass += v / float64(len(mass))
	}
	return m
}

func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func quantile(v []float64, q float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	return s[int(q*float64(len(s)-1)+0.5)]
}

// Print writes one line of key=value pairs for a judge.
func Print(w io.Writer, name string, m Metrics) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s n=%d errors=%d accuracy=%.2f", name, m.N, m.Errors, m.Accuracy)
	for _, l := range Labels {
		fmt.Fprintf(&b, " %s.precision=%.2f %s.recall=%.2f", l, m.Precision[l], l, m.Recall[l])
	}
	fmt.Fprintf(&b, " confident.precision=%.2f confident.coverage=%.2f", m.ConfidentPrecision, m.ConfidentCoverage)
	if m.MedianMS > 0 {
		fmt.Fprintf(&b, " ms.median=%.0f ms.p95=%.0f mass.mean=%.2f", m.MedianMS, m.P95MS, m.MeanMass)
	}
	_, err := fmt.Fprintln(w, b.String())
	return err
}

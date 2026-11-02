package score

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var invented = []Pair{
	{ID: "1", Old: "ACME ships on Monday from Hamburg", New: "ACME ships on Friday from Hamburg", Label: "conflicts"},
	{ID: "2", Old: "OrderApp keeps carts for 24 hours", New: "OrderApp keeps carts for 48 hours", Label: "conflicts"},
	{ID: "3", Old: "Globex has 2,300 employees", New: "Globex employs 2,300 people", Label: "agrees"},
	{ID: "4", Old: "ACME uses Kafka for events", New: "Jane Roe approves refunds", Label: "unrelated"},
	{ID: "5", Old: "OrderApp uses Redis", New: "Globex sends files by SFTP", Label: "unrelated"},
	{ID: "6", Old: "ACME has three warehouses", New: "Payroll closes on the 20th", Label: "unrelated"},
}

func TestConstantAnswersTheMostCommonLabel(t *testing.T) {
	for _, p := range Constant(invented) {
		if p.Pred != "unrelated" {
			t.Fatalf("pred = %q, want unrelated (3 of 6)", p.Pred)
		}
	}
	m := Measure(Constant(invented))
	if m.Accuracy != 0.5 || m.Recall["unrelated"] != 1 || m.Precision["conflicts"] != 0 || m.ConfidentCoverage != 0 {
		t.Fatalf("metrics = %+v", m)
	}
}

func TestOverlapLearnsACutOff(t *testing.T) {
	preds, cut := Overlap(invented)
	if cut <= 0 || cut > 1 {
		t.Fatalf("cut = %v", cut)
	}
	m := Measure(preds)
	// The agrees pair shares as many words as a conflict: overlap cannot tell
	// them apart, which is why a judge must beat it.
	if m.Precision["conflicts"] != 2.0/3 || m.Recall["conflicts"] != 1 || m.Accuracy != 5.0/6 {
		t.Fatalf("overlap metrics = %+v (cut %v)", m, cut)
	}
}

func TestConfidentPrecisionCountsOnlyConfidentAlarms(t *testing.T) {
	preds := []Prediction{
		{Label: "conflicts", Pred: "conflicts", Confident: true, MS: 100, Mass: 0.9},
		{Label: "agrees", Pred: "conflicts", Confident: false, MS: 300, Mass: 0.7},
		{Label: "unrelated", Pred: "conflicts", Confident: true, MS: 200, Mass: 0.8},
		{Label: "conflicts", Err: "decide: no option"},
	}
	m := Measure(preds)
	if m.ConfidentPrecision != 0.5 || m.ConfidentCoverage != 0.5 || m.Errors != 1 || m.Accuracy != 0.25 {
		t.Fatalf("metrics = %+v", m)
	}
	if m.MedianMS != 200 || m.P95MS != 300 {
		t.Fatalf("latency = %v / %v", m.MedianMS, m.P95MS)
	}
}

func TestPrintHoldsNoClaimText(t *testing.T) {
	var b bytes.Buffer
	if err := Print(&b, "constant", Measure(Constant(invented))); err != nil {
		t.Fatal(err)
	}
	for _, p := range invented {
		if strings.Contains(b.String(), p.Old) || strings.Contains(b.String(), "ACME") {
			t.Fatalf("output holds claim text: %s", b.String())
		}
	}
	if !strings.HasPrefix(b.String(), "constant n=6 errors=0 accuracy=0.50") {
		t.Fatalf("output = %s", b.String())
	}
}

func TestReadPairsRefusesUnknownLabels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pairs.jsonl")
	os.WriteFile(path, []byte(`{"id":"1","old":"a","new":"b","label":"other-namespace"}`+"\n"), 0o600)
	if _, err := ReadPairs(path); err == nil {
		t.Fatal("an unknown label was accepted")
	}
}

func TestSyntheticPairsAreBalanced(t *testing.T) {
	pairs, err := ReadPairs("../../testdata/synthetic-vault/pairs.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if m := Measure(Constant(pairs)); len(pairs) != 36 || m.Accuracy < 0.33 || m.Accuracy > 0.34 {
		t.Fatalf("%d pairs, constant accuracy %.2f, want 36 and a third", len(pairs), m.Accuracy)
	}
}

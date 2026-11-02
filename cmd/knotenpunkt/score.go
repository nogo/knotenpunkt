package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nogo/knotenpunkt/decide"
	"github.com/nogo/knotenpunkt/internal/score"
)

// conflictQuestion is the intake check's question; its options are score.Labels.
const conflictQuestion = "Does the new claim conflict with the old one?"

var conflictOptions = []decide.Option{
	{ID: "conflicts", Description: "both cannot be true at once: the new claim changes or contradicts a fact of the old one"},
	{ID: "agrees", Description: "the new claim states the same fact as the old one, possibly with more detail"},
	{ID: "unrelated", Description: "the claims are about different facts, and both can be true"},
}

// runScore asks the model about every pair and prints the model's metrics next
// to the two baselines. Pair text never reaches the output.
func runScore(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("score", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	pairsPath := flags.String("pairs", "", "pairs JSONL from the oracle")
	url := flags.String("url", "", "OpenAI-compatible base URL, such as http://localhost:9481")
	model := flags.String("model", "", "model name on that server")
	threshold := flags.Float64("threshold", 0.7, "top probability for a confident answer")
	perPair := flags.String("out", "", "optional new JSONL file for per-pair results, outside this repository")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *pairsPath == "" || *url == "" || *model == "" {
		return fmt.Errorf("knotenpunkt: score requires -pairs, -url and -model; optional -threshold and -out")
	}
	var results io.WriteCloser
	if *perPair != "" {
		f, err := createOutside(*perPair)
		if err != nil {
			return err
		}
		results = f
		defer results.Close()
	}
	pairs, err := score.ReadPairs(*pairsPath)
	if err != nil {
		return err
	}
	client := &decide.Client{BaseURL: *url, Model: *model}
	var preds []score.Prediction
	for _, p := range pairs {
		start := time.Now()
		res, err := client.Decide(context.Background(), decide.Request{
			Question:  conflictQuestion,
			Options:   conflictOptions,
			Context:   []string{"Old claim: " + p.Old, "New claim: " + p.New},
			Threshold: *threshold,
		})
		pr := score.Prediction{ID: p.ID, Label: p.Label, Pred: res.Choice, Probs: res.Probs,
			Confident: !res.Unsure, MS: float64(time.Since(start).Milliseconds()), Mass: res.Mass}
		if err != nil {
			pr = score.Prediction{ID: p.ID, Label: p.Label, Err: errorKind(err)}
			if !errors.Is(err, decide.ErrNoOption) {
				return fmt.Errorf("knotenpunkt: pair %s: %w", p.ID, err)
			}
		}
		preds = append(preds, pr)
		if results != nil {
			if err := json.NewEncoder(results).Encode(pr); err != nil {
				return fmt.Errorf("knotenpunkt: write results: %w", err)
			}
		}
	}
	overlap, cut := score.Overlap(pairs)
	if err := score.Print(out, "model", score.Measure(preds)); err != nil {
		return err
	}
	if err := score.Print(out, "constant", score.Measure(score.Constant(pairs))); err != nil {
		return err
	}
	return score.Print(out, fmt.Sprintf("overlap(cut=%.2f)", cut), score.Measure(overlap))
}

// errorKind keeps a judge error free of anything the model said.
func errorKind(err error) string {
	if errors.Is(err, decide.ErrNoOption) {
		return "no option"
	}
	return "request failed"
}

// createOutside creates a new file that is not inside this repository, so
// results cannot be committed by accident.
func createOutside(path string) (*os.File, error) {
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("knotenpunkt: cannot resolve output directory")
	}
	target, err := filepath.Abs(filepath.Join(parent, filepath.Base(path)))
	if err != nil {
		return nil, fmt.Errorf("knotenpunkt: cannot resolve output")
	}
	for _, root := range repositoryRoots(path) {
		if rel, err := filepath.Rel(root, target); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("knotenpunkt: output is inside this repository")
		}
	}
	f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("knotenpunkt: cannot create output")
	}
	return f, nil
}

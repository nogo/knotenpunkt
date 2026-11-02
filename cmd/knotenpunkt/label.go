package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"

	"github.com/nogo/knotenpunkt/internal/score"
)

// stdin is where label reads answers; tests replace it.
var stdin io.Reader = os.Stdin

var answerKeys = map[string]string{"c": "conflicts", "a": "agrees", "u": "unrelated"}

// runLabel shows unlabelled pairs one at a time in a fixed shuffled order and
// appends each answer to -out at once, so a stopped session resumes where it
// ended. Pairs already in -out are skipped; a skipped pair is not written.
func runLabel(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("label", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	in := flags.String("in", "", "pairs JSONL without labels")
	dst := flags.String("out", "", "labelled pairs JSONL, appended to")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *in == "" || *dst == "" {
		return fmt.Errorf("knotenpunkt: label requires -in and -out")
	}
	pairs, err := readUnlabelled(*in)
	if err != nil {
		return err
	}
	done := map[string]bool{}
	if old, err := score.ReadPairs(*dst); err == nil {
		for _, p := range old {
			done[p.ID] = true
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := os.OpenFile(*dst, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("knotenpunkt: open %s: %w", *dst, err)
	}
	defer f.Close()
	rand.New(rand.NewSource(1)).Shuffle(len(pairs), func(i, j int) { pairs[i], pairs[j] = pairs[j], pairs[i] })
	answers := bufio.NewScanner(stdin)
	for i, p := range pairs {
		if done[p.ID] {
			continue
		}
		fmt.Fprintf(out, "\n[%d/%d]\nOLD  %s\nNEW  %s\n(c)onflicts (a)grees (u)nrelated (s)kip (q)uit > ", i+1, len(pairs), p.Old, p.New)
		label := ""
		for label == "" && answers.Scan() {
			switch key := strings.TrimSpace(strings.ToLower(answers.Text())); key {
			case "q":
				return nil
			case "s":
				label = "-"
			default:
				if l, ok := answerKeys[key]; ok {
					label = l
				} else {
					fmt.Fprint(out, "c, a, u, s or q > ")
				}
			}
		}
		if label == "" {
			return answers.Err()
		}
		if label == "-" {
			continue
		}
		p.Label = label
		if err := json.NewEncoder(f).Encode(p); err != nil {
			return fmt.Errorf("knotenpunkt: write %s: %w", *dst, err)
		}
	}
	fmt.Fprintln(out, "\nall pairs answered")
	return nil
}

func readUnlabelled(path string) ([]score.Pair, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("knotenpunkt: %w", err)
	}
	var out []score.Pair
	dec := json.NewDecoder(strings.NewReader(string(data)))
	for dec.More() {
		var p score.Pair
		if err := dec.Decode(&p); err != nil {
			return nil, fmt.Errorf("knotenpunkt: %s is not pairs JSONL", path)
		}
		out = append(out, p)
	}
	return out, nil
}

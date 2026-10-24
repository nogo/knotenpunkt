// Command knotenpunkt builds a private oracle from a sidenote vault.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/nogo/knotenpunkt/internal/oracle"
)

// main reports sanitized errors and exits unsuccessfully when the oracle cannot be built.
func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run parses flags silently so invalid arguments cannot echo private data.
func run(args []string, out io.Writer) error {
	if len(args) == 0 || args[0] != "oracle" {
		return fmt.Errorf("knotenpunkt: expected oracle command")
	}
	flags := flag.NewFlagSet("oracle", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	vault := flags.String("vault", "", "sidenote vault directory")
	output := flags.String("out", "", "new JSONL output file")
	seed := flags.Int64("seed", 1, "sampling seed")
	if flags.Parse(args[1:]) != nil || flags.NArg() != 0 || *vault == "" || *output == "" {
		return fmt.Errorf("knotenpunkt: oracle requires -vault and -out; optional -seed")
	}
	counts, err := oracle.Build(*vault, *output, *seed, repositoryRoots(*output)...)
	if err != nil {
		return err
	}
	return counts.Print(out)
}

// repositoryRoots locates this checkout from build sources and the working directory.
func repositoryRoots(outputs ...string) []string {
	var roots []string
	_, source, _, ok := runtime.Caller(0)
	starts := []string{}
	for _, output := range outputs {
		if parent, err := filepath.EvalSymlinks(filepath.Dir(output)); err == nil {
			if absolute, err := filepath.Abs(parent); err == nil {
				starts = append(starts, absolute)
			}
		}
	}
	if ok && filepath.IsAbs(source) {
		starts = append(starts, filepath.Dir(source))
	}
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	for _, start := range starts {
		for dir := start; ; dir = filepath.Dir(dir) {
			data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
			if err == nil && strings.HasPrefix(string(data), "module github.com/nogo/knotenpunkt\n") {
				roots = append(roots, dir)
				break
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	return roots
}

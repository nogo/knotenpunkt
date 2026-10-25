package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOracleCommand checks command wiring and the actual repository boundary.
func TestOracleCommand(t *testing.T) {
	vault := t.TempDir()
	dir := filepath.Join(vault, "log", "2026-10")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	data := []byte("{\"type\":\"claim\",\"id\":\"a\",\"text\":\"PRIVATE ACME\",\"author\":{\"kind\":\"user\",\"id\":\"u1\"}}\n")
	if err := os.WriteFile(filepath.Join(dir, "01.jsonl"), data, 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "pairs.jsonl")
	var printed bytes.Buffer
	if err := run([]string{"oracle", "-vault", vault, "-out", out, "-seed", "42"}, &printed); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(printed.String(), "claims_read=1 confirmed=1 live=1") || strings.Contains(printed.String(), "ACME") || strings.Contains(printed.String(), vault) {
		t.Fatalf("unsafe or wrong counts: %s", printed.String())
	}
	roots := repositoryRoots()
	if len(roots) == 0 {
		t.Fatal("repository not found")
	}
	if err := run([]string{"oracle", "-vault", vault, "-out", filepath.Join(roots[0], "forbidden.jsonl")}, &printed); err == nil {
		t.Fatal("repository output accepted")
	}
}

// TestInvalidFlagsStayPrivate ensures flag errors cannot repeat private arguments.
func TestInvalidFlagsStayPrivate(t *testing.T) {
	var printed bytes.Buffer
	err := run([]string{"oracle", "-seed", "PRIVATE ACME"}, &printed)
	if err == nil || strings.Contains(err.Error(), "PRIVATE") || printed.Len() != 0 {
		t.Fatal("unsafe flag error")
	}
}

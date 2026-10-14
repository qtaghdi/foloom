package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseOptionsAcceptsPositionalWorkbookAndLegacyFileFlag(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "positional", args: []string{"book.xlsx"}},
		{name: "legacy flag", args: []string{"-file", "book.xlsx"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			options, err := parseOptions(test.args, &bytes.Buffer{})
			if err != nil {
				t.Fatal(err)
			}
			if options.file != "book.xlsx" {
				t.Fatalf("file = %q, want book.xlsx", options.file)
			}
		})
	}
}

func TestParseOptionsRejectsAmbiguousOrMissingWorkbook(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"one.xlsx", "two.xlsx"},
		{"-file", "one.xlsx", "two.xlsx"},
	} {
		if _, err := parseOptions(args, &bytes.Buffer{}); err == nil {
			t.Fatalf("parseOptions(%q) succeeded, want error", args)
		}
	}
}

func TestVersionDoesNotRequireWorkbook(t *testing.T) {
	previousVersion, previousCommit, previousBuildDate := version, commit, buildDate
	version, commit, buildDate = "0.1.0-alpha.1", "abc123", "2026-10-14T00:00:00Z"
	t.Cleanup(func() {
		version, commit, buildDate = previousVersion, previousCommit, previousBuildDate
	})

	var stdout, stderr bytes.Buffer
	if exitCode := run([]string{"-version"}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "0.1.0-alpha.1") || !strings.Contains(got, "abc123") {
		t.Fatalf("version output = %q", got)
	}
}

func TestCheckValidatesWorkbookWithoutStartingTransport(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test path")
	}
	fixtureSource := filepath.Join(filepath.Dir(currentFile), "..", "..", "testdata", "xlsx", "compatibility", "libreoffice.xlsx")
	content, err := os.ReadFile(fixtureSource)
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(t.TempDir(), "libreoffice.xlsx")
	if err := os.WriteFile(fixture, content, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if exitCode := run([]string{"-check", fixture}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "Workbook check passed") || !strings.Contains(got, "Sheets:") {
		t.Fatalf("check output = %q", got)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(fixture), ".xlsx-viewer-history")); !os.IsNotExist(err) {
		t.Fatalf("check created workbook history: %v", err)
	}
}

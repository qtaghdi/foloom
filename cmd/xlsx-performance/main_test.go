package main

import (
	"strings"
	"testing"
	"time"
)

func TestProfilesDefineIncreasingWorkloadsAndExplicitBudgets(t *testing.T) {
	ci, err := profileFor("ci")
	if err != nil {
		t.Fatal(err)
	}
	release, err := profileFor("release")
	if err != nil {
		t.Fatal(err)
	}
	ciCells := ci.sheets * ci.rowsPerSheet * ci.columns
	releaseCells := release.sheets * release.rowsPerSheet * release.columns
	if ciCells != 25_000 {
		t.Fatalf("CI workload cells = %d, want 25000", ciCells)
	}
	if releaseCells != 400_000 {
		t.Fatalf("release workload cells = %d, want 400000", releaseCells)
	}
	if releaseCells <= ciCells {
		t.Fatal("release workload must be larger than CI workload")
	}
	if ci.performanceBudget.open <= 0 || ci.performanceBudget.read <= 0 || ci.performanceBudget.peakHeapBytes == 0 {
		t.Fatal("CI profile must define positive latency and heap budgets")
	}
	if _, err := profileFor("unknown"); err == nil {
		t.Fatal("unknown profile must be rejected")
	}
}

func TestReportStatesEvidenceAndMemoryBoundaries(t *testing.T) {
	profile, err := profileFor("ci")
	if err != nil {
		t.Fatal(err)
	}
	result := evidence{
		commit:          "abc123",
		recordedAt:      time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC),
		workload:        profile,
		excelizeVersion: "v2.11.0",
		measurements: []measurement{{
			name: "Open session", duration: time.Second, limit: profile.performanceBudget.open,
			allocatedBytes: 2 << 20, peakHeapBytes: 3 << 20,
		}},
	}
	report := renderMarkdown(result)
	for _, expected := range []string{
		"regression guardrails", "25000", "Overall status: **Pass**",
		"not operating-system peak RSS", "Generated workbooks are temporary",
	} {
		if !strings.Contains(report, expected) {
			t.Fatalf("report does not contain %q", expected)
		}
	}
}

func TestMeasurementResultExplainsBudgetFailure(t *testing.T) {
	status, details := measurementResult(measurement{
		duration:      2 * time.Second,
		limit:         time.Second,
		peakHeapBytes: 3 << 20,
	}, budget{peakHeapBytes: 2 << 20})
	if status != "Fail" {
		t.Fatalf("status = %q, want Fail", status)
	}
	if !strings.Contains(details, "Latency budget exceeded") || !strings.Contains(details, "Heap budget exceeded") {
		t.Fatalf("details = %q", details)
	}
}

package main

import (
	"fmt"
	"runtime"
	"strings"
	"time"
)

func renderMarkdown(result evidence) string {
	var output strings.Builder
	status := "Pass"
	if result.failed {
		status = "Fail"
	}
	fmt.Fprintln(&output, "# XLSX Performance Evidence")
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "These budgets are regression guardrails, not end-user latency promises.")
	fmt.Fprintln(&output, "Generated workbooks are temporary and contain no user data.")
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "## Environment")
	fmt.Fprintln(&output)
	fmt.Fprintf(&output, "- Candidate commit: `%s`\n", result.commit)
	fmt.Fprintf(&output, "- Working tree dirty at start: `%t`\n", result.dirty)
	fmt.Fprintf(&output, "- Recorded at: `%s`\n", result.recordedAt.Format(time.RFC3339))
	fmt.Fprintf(&output, "- Operating system and architecture: `%s/%s`\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&output, "- Logical CPUs: `%d`\n", runtime.NumCPU())
	fmt.Fprintf(&output, "- Go: `%s`\n", runtime.Version())
	fmt.Fprintf(&output, "- Excelize: `%s`\n", result.excelizeVersion)
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "## Workload")
	fmt.Fprintln(&output)
	fmt.Fprintf(&output, "- Profile: `%s`\n", result.workload.name)
	fmt.Fprintf(&output, "- Populated cells: `%d` across `%d` sheets\n", result.workload.sheets*result.workload.rowsPerSheet*result.workload.columns, result.workload.sheets)
	fmt.Fprintf(&output, "- Shape per sheet: `%d` data rows by `%d` columns, plus one header row\n", result.workload.rowsPerSheet, result.workload.columns)
	fmt.Fprintf(&output, "- Formula interval: every `%d` rows in the final column\n", result.workload.formulaEveryRows)
	fmt.Fprintf(&output, "- Bounded read: `%d` cells\n", result.workload.boundedReadCells)
	fmt.Fprintf(&output, "- Batch commit: `%d` cells\n", result.workload.batchCommitCells)
	fmt.Fprintf(&output, "- Generated workbook size: `%d` bytes\n", result.workbookBytes)
	fmt.Fprintf(&output, "- Generated workbook SHA-256: `%s`\n", result.workbookSHA256)
	fmt.Fprintf(&output, "- Generation time, excluded from latency budgets: `%s`\n", formatDuration(result.generation.duration))
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "## Budgets")
	fmt.Fprintln(&output)
	fmt.Fprintf(&output, "- Open: `%s`\n", result.workload.performanceBudget.open)
	fmt.Fprintf(&output, "- Read 10,000 cells: `%s`\n", result.workload.performanceBudget.read)
	fmt.Fprintf(&output, "- Commit one cell: `%s`\n", result.workload.performanceBudget.singleCommit)
	fmt.Fprintf(&output, "- Commit 1,000 cells: `%s`\n", result.workload.performanceBudget.batchCommit)
	fmt.Fprintf(&output, "- Peak observed Go heap for each measured action: `%s`\n", formatBytes(result.workload.performanceBudget.peakHeapBytes))
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "## Results")
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "| Action | Duration | Limit | Allocated during action | Peak observed Go heap | Status | Details |")
	fmt.Fprintln(&output, "| --- | ---: | ---: | ---: | ---: | --- | --- |")
	for _, item := range result.measurements {
		itemStatus, details := measurementResult(item, result.workload.performanceBudget)
		fmt.Fprintf(&output, "| %s | %s | %s | %s | %s | %s | %s |\n",
			item.name, formatDuration(item.duration), item.limit, formatBytes(item.allocatedBytes), formatBytes(item.peakHeapBytes), itemStatus, details)
	}
	fmt.Fprintln(&output)
	fmt.Fprintf(&output, "Overall status: **%s**\n", status)
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "## Interpretation Boundaries")
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "- The heap sampler observes Go `HeapAlloc` at one-millisecond intervals. It is not operating-system peak RSS.")
	fmt.Fprintln(&output, "- The workbook is synthetic and does not model every chart, image, style, formula, or shared-string distribution.")
	fmt.Fprintln(&output, "- Generation time is reported separately and is not part of open or edit latency.")
	fmt.Fprintln(&output, "- Shared CI runner variance is expected. A failure blocks the gate and requires investigation, not a budget increase without evidence.")
	fmt.Fprintln(&output, "- Run the release profile on the release machine and retain its report before making performance claims.")
	return output.String()
}

func measurementResult(item measurement, limits budget) (string, string) {
	details := make([]string, 0, 3)
	if item.err != nil {
		details = append(details, "Error: "+item.err.Error())
	}
	if item.duration > item.limit {
		details = append(details, "Latency budget exceeded")
	}
	if item.peakHeapBytes > limits.peakHeapBytes {
		details = append(details, "Heap budget exceeded")
	}
	if len(details) == 0 {
		return "Pass", "Within budget"
	}
	return "Fail", strings.ReplaceAll(strings.Join(details, "; "), "|", "\\|")
}

func formatDuration(value time.Duration) string {
	return fmt.Sprintf("%.2f ms", float64(value)/float64(time.Millisecond))
}

func formatBytes(value uint64) string {
	const mebibyte = 1 << 20
	return fmt.Sprintf("%.2f MiB", float64(value)/mebibyte)
}

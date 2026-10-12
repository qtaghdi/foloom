package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"github.com/qtaghdi/document-workspace/internal/formats/xlsx"
	"github.com/xuri/excelize/v2"
)

type measurement struct {
	name           string
	duration       time.Duration
	allocatedBytes uint64
	peakHeapBytes  uint64
	limit          time.Duration
	err            error
}

type evidence struct {
	commit          string
	dirty           bool
	recordedAt      time.Time
	workload        workload
	generation      measurement
	measurements    []measurement
	workbookBytes   int64
	workbookSHA256  string
	excelizeVersion string
	failed          bool
}

func run(ctx context.Context, profile workload) (evidence, error) {
	tempDir, err := os.MkdirTemp("", "document-workspace-xlsx-performance-")
	if err != nil {
		return evidence{}, fmt.Errorf("create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	result := evidence{
		commit:          gitOutput(ctx, "rev-parse", "HEAD"),
		dirty:           gitOutput(ctx, "status", "--porcelain") != "",
		recordedAt:      time.Now().UTC(),
		workload:        profile,
		excelizeVersion: dependencyVersion("github.com/xuri/excelize/v2"),
	}
	workbookPath := filepath.Join(tempDir, profile.name+"-performance.xlsx")
	result.generation = measure("Generate synthetic workbook", 0, func() error {
		return generateWorkbook(workbookPath, profile)
	})
	if result.generation.err != nil {
		return evidence{}, result.generation.err
	}
	info, err := os.Stat(workbookPath)
	if err != nil {
		return evidence{}, fmt.Errorf("inspect generated workbook: %w", err)
	}
	contents, err := os.ReadFile(workbookPath)
	if err != nil {
		return evidence{}, fmt.Errorf("hash generated workbook: %w", err)
	}
	result.workbookBytes = info.Size()
	result.workbookSHA256 = fmt.Sprintf("%x", sha256.Sum256(contents))

	var session *xlsx.Session
	open := measure("Open session", profile.performanceBudget.open, func() error {
		var openErr error
		session, openErr = xlsx.Open(workbookPath)
		return openErr
	})
	result.measurements = append(result.measurements, open)
	if open.err != nil {
		return result, fmt.Errorf("open generated workbook: %w", open.err)
	}
	defer session.Close()

	read := measure("Read 10,000 cells", profile.performanceBudget.read, func() error {
		_, readErr := session.ReadRange("Data1", "A1:CV100")
		return readErr
	})
	result.measurements = append(result.measurements, read)

	single := measure("Commit one cell", profile.performanceBudget.singleCommit, func() error {
		snapshot := session.Snapshot()
		_, applyErr := session.Apply(snapshot.Revision, "benchmark", []xlsx.Operation{{
			Type: "set_cell", Sheet: "Data1", Cell: "A1002", Value: "single-commit",
		}})
		return applyErr
	})
	result.measurements = append(result.measurements, single)

	batch := measure("Commit 1,000 cells", profile.performanceBudget.batchCommit, func() error {
		cells := make([][]xlsx.CellInput, 100)
		for row := range cells {
			cells[row] = make([]xlsx.CellInput, 10)
			for column := range cells[row] {
				cells[row][column].Value = row*10 + column
			}
		}
		snapshot := session.Snapshot()
		_, applyErr := session.Apply(snapshot.Revision, "benchmark", []xlsx.Operation{{
			Type: "paste_range", Sheet: "Data1", Range: "A1003:J1102", Cells: cells,
		}})
		return applyErr
	})
	result.measurements = append(result.measurements, batch)

	for _, item := range result.measurements {
		if item.err != nil || item.duration > item.limit || item.peakHeapBytes > profile.performanceBudget.peakHeapBytes {
			result.failed = true
		}
	}
	return result, nil
}

func generateWorkbook(path string, profile workload) error {
	book := excelize.NewFile()
	defer book.Close()

	headerStyle, err := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return fmt.Errorf("create header style: %w", err)
	}
	for sheetIndex := 1; sheetIndex <= profile.sheets; sheetIndex++ {
		sheet := fmt.Sprintf("Data%d", sheetIndex)
		if sheetIndex == 1 {
			if err := book.SetSheetName("Sheet1", sheet); err != nil {
				return fmt.Errorf("rename first sheet: %w", err)
			}
		} else if _, err := book.NewSheet(sheet); err != nil {
			return fmt.Errorf("create sheet %s: %w", sheet, err)
		}
		writer, err := book.NewStreamWriter(sheet)
		if err != nil {
			return fmt.Errorf("create stream writer for %s: %w", sheet, err)
		}
		header := make([]any, profile.columns)
		for column := range header {
			header[column] = excelize.Cell{StyleID: headerStyle, Value: fmt.Sprintf("Column %d", column+1)}
		}
		if err := writer.SetRow("A1", header); err != nil {
			return fmt.Errorf("write header for %s: %w", sheet, err)
		}
		for row := 1; row <= profile.rowsPerSheet; row++ {
			values := make([]any, profile.columns)
			for column := range values {
				if column%10 == 0 {
					values[column] = fmt.Sprintf("S%d-R%d-C%d", sheetIndex, row, column+1)
				} else {
					values[column] = sheetIndex*1_000_000 + row*1_000 + column
				}
			}
			cell, _ := excelize.CoordinatesToCellName(1, row+1)
			if err := writer.SetRow(cell, values); err != nil {
				return fmt.Errorf("write %s row %d: %w", sheet, row, err)
			}
		}
		if err := writer.Flush(); err != nil {
			return fmt.Errorf("flush %s: %w", sheet, err)
		}
		for row := profile.formulaEveryRows; row <= profile.rowsPerSheet; row += profile.formulaEveryRows {
			cell, _ := excelize.CoordinatesToCellName(profile.columns, row+1)
			formula := fmt.Sprintf("SUM(B%d:D%d)", row+1, row+1)
			if err := book.SetCellFormula(sheet, cell, formula); err != nil {
				return fmt.Errorf("write formula %s!%s: %w", sheet, cell, err)
			}
		}
	}
	if err := book.SaveAs(path); err != nil {
		return fmt.Errorf("save generated workbook: %w", err)
	}
	return nil
}

func measure(name string, limit time.Duration, operation func() error) measurement {
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	var peak atomic.Uint64
	peak.Store(before.HeapAlloc)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				var current runtime.MemStats
				runtime.ReadMemStats(&current)
				updatePeak(&peak, current.HeapAlloc)
			case <-stop:
				return
			}
		}
	}()
	started := time.Now()
	err := operation()
	duration := time.Since(started)
	close(stop)
	<-done
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	updatePeak(&peak, after.HeapAlloc)
	return measurement{
		name:           name,
		duration:       duration,
		allocatedBytes: after.TotalAlloc - before.TotalAlloc,
		peakHeapBytes:  peak.Load(),
		limit:          limit,
		err:            err,
	}
}

func updatePeak(peak *atomic.Uint64, candidate uint64) {
	for current := peak.Load(); candidate > current; current = peak.Load() {
		if peak.CompareAndSwap(current, candidate) {
			return
		}
	}
}

func dependencyVersion(path string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dependency := range info.Deps {
		if dependency.Path == path {
			return dependency.Version
		}
	}
	return "unknown"
}

func gitOutput(ctx context.Context, args ...string) string {
	command := exec.CommandContext(ctx, "git", args...)
	output, err := command.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

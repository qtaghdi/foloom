package main

import (
	"fmt"
	"time"
)

type budget struct {
	open          time.Duration
	read          time.Duration
	singleCommit  time.Duration
	batchCommit   time.Duration
	peakHeapBytes uint64
}

type workload struct {
	name              string
	sheets            int
	rowsPerSheet      int
	columns           int
	formulaEveryRows  int
	boundedReadCells  int
	batchCommitCells  int
	performanceBudget budget
}

func profileFor(name string) (workload, error) {
	const mebibyte = 1 << 20
	switch name {
	case "ci":
		return workload{
			name:             "ci",
			sheets:           1,
			rowsPerSheet:     250,
			columns:          100,
			formulaEveryRows: 25,
			boundedReadCells: 10_000,
			batchCommitCells: 1_000,
			performanceBudget: budget{
				open:          3 * time.Second,
				read:          time.Second,
				singleCommit:  5 * time.Second,
				batchCommit:   8 * time.Second,
				peakHeapBytes: 512 * mebibyte,
			},
		}, nil
	case "release":
		return workload{
			name:             "release",
			sheets:           4,
			rowsPerSheet:     1_000,
			columns:          100,
			formulaEveryRows: 25,
			boundedReadCells: 10_000,
			batchCommitCells: 1_000,
			performanceBudget: budget{
				open:          10 * time.Second,
				read:          2 * time.Second,
				singleCommit:  15 * time.Second,
				batchCommit:   20 * time.Second,
				peakHeapBytes: 1024 * mebibyte,
			},
		}, nil
	default:
		return workload{}, fmt.Errorf("unknown profile %q: use ci or release", name)
	}
}

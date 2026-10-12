package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

func main() {
	profileName := flag.String("profile", "ci", "performance profile: ci or release")
	reportPath := flag.String("report", os.Getenv("REPORT"), "optional Markdown report path")
	flag.Parse()

	profile, err := profileFor(*profileName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	result, err := run(context.Background(), profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run XLSX performance profile: %v\n", err)
		os.Exit(1)
	}
	report := renderMarkdown(result)
	fmt.Print(report)
	if *reportPath != "" {
		if err := os.WriteFile(*reportPath, []byte(report), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write performance report: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\nPerformance report written to %s\n", *reportPath)
	}
	if result.failed {
		os.Exit(1)
	}
}

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/qtaghdi/document-workspace/internal/formats/xlsx"
	"github.com/qtaghdi/document-workspace/internal/transport/httpapi"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

type options struct {
	file             string
	transport        string
	addr             string
	stdioBrowser     bool
	stdioBrowserAddr string
	browserToken     string
	mcpToken         string
	check            bool
	showVersion      bool
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	options, err := parseOptions(args, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}
	if options.showVersion {
		fmt.Fprintf(stdout, "document-workspace %s (commit %s, built %s)\n", version, commit, buildDate)
		return 0
	}

	if options.check {
		inspection, err := xlsx.Inspect(options.file)
		if err != nil {
			fmt.Fprintf(stderr, "check workbook: %v\n", err)
			return 1
		}
		printWorkbookCheck(stdout, inspection)
		return 0
	}

	session, err := xlsx.Open(options.file)
	if err != nil {
		fmt.Fprintf(stderr, "open workbook: %v\n", err)
		return 1
	}
	defer session.Close()

	if options.browserToken == "" {
		options.browserToken, err = randomToken()
		if err != nil {
			fmt.Fprintf(stderr, "generate browser token: %v\n", err)
			return 1
		}
	}
	if options.mcpToken == "" {
		options.mcpToken, err = randomToken()
		if err != nil {
			fmt.Fprintf(stderr, "generate MCP token: %v\n", err)
			return 1
		}
	}

	api := httpapi.NewWithTokens(session, options.browserToken, options.mcpToken)
	if options.transport == "stdio" {
		if options.stdioBrowser {
			err = api.RunStdioWithBrowser(context.Background(), options.stdioBrowserAddr)
		} else {
			err = api.RunStdio(context.Background())
		}
		if err != nil {
			fmt.Fprintf(stderr, "run stdio transport: %v\n", err)
			return 1
		}
		return 0
	}

	logger := log.New(stderr, "", log.LstdFlags)
	server := &http.Server{
		Addr:              options.addr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	logger.Printf("viewer: http://%s/?token=%s", options.addr, options.browserToken)
	logger.Printf("mcp: http://%s/mcp (Authorization: Bearer %s)", options.addr, options.mcpToken)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(stderr, "run HTTP transport: %v\n", err)
		return 1
	}
	return 0
}

func parseOptions(args []string, stderr io.Writer) (options, error) {
	var result options
	flags := flag.NewFlagSet("document-workspace", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&result.file, "file", "", "path to the XLSX workbook to open")
	flags.StringVar(&result.transport, "transport", "http", "MCP transport: http or stdio")
	flags.StringVar(&result.addr, "addr", "127.0.0.1:8765", "HTTP listen address")
	flags.BoolVar(&result.stdioBrowser, "stdio-browser", true, "serve a loopback browser fallback in stdio mode")
	flags.StringVar(&result.stdioBrowserAddr, "stdio-browser-addr", "127.0.0.1:0", "loopback address for the stdio browser fallback")
	flags.StringVar(&result.browserToken, "browser-token", "", "browser session token; generated when omitted")
	flags.StringVar(&result.mcpToken, "mcp-token", "", "MCP bearer token; generated when omitted")
	flags.BoolVar(&result.check, "check", false, "validate the workbook and exit without starting a server")
	flags.BoolVar(&result.showVersion, "version", false, "print build version and exit")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: document-workspace [options] <workbook.xlsx>")
		fmt.Fprintln(stderr, "       document-workspace -file <workbook.xlsx> [options]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Options:")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	if result.showVersion {
		return result, nil
	}
	if flags.NArg() > 1 {
		return options{}, errors.New("provide exactly one workbook path")
	}
	if flags.NArg() == 1 {
		if result.file != "" {
			return options{}, errors.New("provide the workbook as either a positional path or -file, not both")
		}
		result.file = flags.Arg(0)
	}
	if strings.TrimSpace(result.file) == "" {
		flags.Usage()
		return options{}, errors.New("workbook path is required")
	}
	if result.transport != "http" && result.transport != "stdio" {
		return options{}, fmt.Errorf("unsupported transport %q: use http or stdio", result.transport)
	}
	return result, nil
}

func printWorkbookCheck(output io.Writer, snapshot xlsx.Inspection) {
	fmt.Fprintln(output, "Workbook check passed")
	fmt.Fprintf(output, "Name: %s\n", snapshot.Name)
	fmt.Fprintf(output, "Sheets: %d\n", len(snapshot.Sheets))
	fmt.Fprintf(output, "Compatibility warnings: %d\n", len(snapshot.Warnings))
	for _, warning := range snapshot.Warnings {
		fmt.Fprintf(output, "- %s: %s\n", warning.Feature, warning.Message)
	}
	fmt.Fprintf(output, "Formula storage: %s\n", snapshot.FormulaPolicy.Storage)
	fmt.Fprintf(output, "Native recalculation: %s\n", snapshot.FormulaPolicy.NativeRecalculation)
}

func randomToken() (string, error) {
	var value [24]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

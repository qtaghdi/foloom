# Packaging Verification

The release executable embeds two separately built interfaces:

- `index.html` plus hashed assets for the authenticated browser editor;
- `app.html` as a self-contained MCP App resource.

Run the complete packaging gate from a clean checkout:

```bash
make package-verify
```

The target installs the exact pnpm lockfile, builds both interfaces, validates
their generated files, compiles the Go executable, opens a disposable XLSX copy
through an absolute path containing spaces, and exercises both embedded bundle
delivery paths. GitHub Actions runs the same target on pull requests and pushes
to `main` from a clean checkout.

`make build` rebuilds and validates both interfaces before compiling
`bin/document-workspace`. `make go-build` remains a low-level Go-only command
for development; it does not create missing frontend output.

## Failure Behavior

`make package-check` fails with an actionable message when either entry file is
missing, when the browser entry references a missing or empty hashed asset, or
when the MCP App entry is not self-contained. Its regression tests cover
missing generated output, stale browser references, and a complete fixture.

Generated hashed assets and `app.html` remain ignored because they are build
outputs. The browser `index.html` remains tracked so source changes that alter
its hashed references are visible in review. A package must always run
`web-build` before the Go compiler snapshots the embedded directory.

## Alpha Release Archives

Run the current-platform release test after the embedded interfaces are built:

```bash
make release-package-test VERSION=0.1.0-alpha.local
```

The command creates one archive, verifies its SHA-256 checksum and required
contents, extracts it, checks its embedded version, and validates a disposable
XLSX fixture without starting a transport.

Run the complete cross-platform archive build with:

```bash
make release-verify VERSION=0.1.0-alpha.local
```

The output in `dist/` contains unsigned archives for macOS, Linux, and Windows
on AMD64 and ARM64, plus `SHA256SUMS`. Each archive contains the executable,
README, format support matrix, host compatibility guide, and local security
guide. The Go binary embeds the previously built browser and MCP App bundles.

The `Private alpha package` workflow runs only through an authorized manual
dispatch. It uploads repository-scoped workflow artifacts and has read-only
repository contents permission. Tags do not trigger it, and it cannot create a
GitHub Release. Reintroducing public publication requires a separate issue,
owner decision, distribution terms, and policy review.

The alpha artifacts are not signed or notarized. They do not include macOS or
Windows installers, automatic updates, a file picker, or a hosted service. The
[getting-started guide](getting-started.md) explains the supported command-line
and MCP-host flow without recommending global operating system security bypasses.

## Build Metadata and Preflight

Release builds embed the requested version, source commit, and commit date.
Users can record them with `document-workspace -version`. Existing integrations
can continue using `-file`, while interactive users can pass one workbook path
positionally.

`document-workspace -check workbook.xlsx` opens the workbook through the same
XLSX session boundary used by the service, reports structural compatibility and
formula policy metadata, then exits. It does not start HTTP or stdio transport
and does not print cell contents.

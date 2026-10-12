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

## Supported Local Artifact

The current artifact is a platform-specific executable built by Go for the
host operating system and architecture. It accepts absolute workbook paths,
including paths containing spaces. This gate does not yet produce signed macOS
or Windows installers, cross-compiled archives, container images, checksums, or
a public release manifest.

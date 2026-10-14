# XLSX Alpha Getting Started

document-workspace currently runs as a local process for one existing XLSX
workbook. It can connect to Claude Desktop or Codex over stdio and can provide
the spreadsheet UI through an MCP App or an authenticated loopback browser
fallback.

The alpha is not a Microsoft Excel replacement or a hosted collaboration
service. Use a disposable copy of an important workbook until the native Excel
release gates are complete.

## Choose an Archive

Release artifacts use these names:

| Operating system | Processor | Archive suffix |
| --- | --- | --- |
| macOS | Apple silicon | `darwin_arm64.tar.gz` |
| macOS | Intel | `darwin_amd64.tar.gz` |
| Linux | x86-64 | `linux_amd64.tar.gz` |
| Linux | ARM64 | `linux_arm64.tar.gz` |
| Windows | x86-64 | `windows_amd64.zip` |
| Windows | ARM64 | `windows_arm64.zip` |

Every artifact set includes `SHA256SUMS`. Verify the selected archive before
extracting it:

```bash
shasum -a 256 -c SHA256SUMS
```

The current alpha archives are unsigned. macOS or Windows can therefore show an
unverified developer warning. Review the downloaded file and checksum, then use
the operating system's one-time approval flow if you trust the artifact. Do not
disable operating system protection globally.

## Check a Workbook

Run the preflight before configuring an MCP host:

```bash
./document-workspace -check /absolute/path/to/workbook.xlsx
```

The command validates the OOXML package, supported size and archive boundaries,
sheet metadata, compatibility warnings, and formula policy. It does not start a
network listener and does not print cell contents.

## Run the Browser Editor

Pass the workbook as a positional path:

```bash
./document-workspace /absolute/path/to/workbook.xlsx
```

Open the exact viewer URL printed to the terminal. The URL contains a local
session credential. Do not share terminal output or the URL. The service binds
to `127.0.0.1:8765` by default and does not expose the workbook to the network.

Use `./document-workspace -help` for address, transport, and fallback options.
Use `./document-workspace -version` when recording a bug report.

## Connect Claude Desktop

Use the extracted executable and an absolute workbook path in the Claude
Desktop MCP configuration:

```json
{
  "mcpServers": {
    "document-workspace": {
      "command": "/absolute/path/to/document-workspace",
      "args": [
        "-transport",
        "stdio",
        "-file",
        "/absolute/path/to/workbook.xlsx"
      ]
    }
  }
}
```

Fully quit and reopen Claude Desktop after changing the configuration. Ask the
host to call `open_workbook`. If it does not render the MCP App, open the
single-use `browserUrl` returned by that tool.

## Connect Codex

Add the extracted executable and workbook to the applicable Codex
`config.toml`:

```toml
[mcp_servers.document_workspace]
command = "/absolute/path/to/document-workspace"
args = ["-transport", "stdio", "-file", "/absolute/path/to/workbook.xlsx"]
```

Start a new Codex session so it discovers the tools. Ask Codex to call
`open_workbook` and use the returned single-use URL in the in-app browser when
native MCP App rendering is unavailable.

## Approval and Safety

Read tools are marked read-only. Workbook mutations are marked destructive so
the MCP host can apply its own approval policy. Prefer one-time approval until
the operation batch, workbook, and server configuration are familiar. The
server still rejects stale revisions and invalid or oversized batches after a
host approval.

The process writes changes directly to the selected workbook with an atomic
same-directory replacement. It also creates bounded local undo history beside
the workbook. Keep another editor closed while writing, and retain an external
backup for important files.

See [host compatibility](host-compatibility.md), [local security](security.md),
[format support](format-support.md), and [XLSX recovery](xlsx-recovery.md) for
the tested boundaries.

# 변경 이력

이 프로젝트의 중요한 변경 사항을 이 파일에 기록합니다.

형식은 [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)를 따르며,
릴리스를 시작하면 Semantic Versioning을 적용할 예정입니다.

## 미출시

### 추가

- Added unsigned alpha archive generation for macOS, Linux, and Windows on
  AMD64 and ARM64, with SHA-256 manifests, archive content checks, and a
  current-platform executable smoke test.
- Added a manually dispatched private-alpha workflow that uploads verified build
  artifacts inside the repository without permission to publish a GitHub Release.
- Added CLI build metadata, positional workbook paths, and a workbook preflight
  that validates compatibility without starting a transport or printing cells.
- Added a private XLSX alpha guide covering checksums, unsigned application
  warnings, local browser use, Claude Desktop, Codex, approvals, and backups.
- Added a single pre-Excel XLSX candidate verification command that records
  safety, compatibility, full-suite, packaging, and release-performance results
  while keeping native Excel and visible-host claims explicitly manual.
- Added a required full-regression CI job for frontend tests, Go tests, race
  detection, static analysis, and the preserved Python regression suite.
- Added CI and release XLSX performance profiles with explicit latency and Go
  heap guardrails, temporary generated workloads, and Markdown evidence reports.
- Added a native Microsoft Excel fixture, edit, formula, and reopen procedure
  with a reusable evidence template for the remaining X-07 and X-08 gates.
- Added a reproducible X-01 through X-06 XLSX safety verification command and
  required CI check with environment, lockfile, fixture, and focused test evidence.
- Added deterministic XLSX transaction tests for mixed-batch rejection and
  concurrent same-revision writes across memory, disk, history, and events.
- Added a structured XLSX formula policy that identifies cached or unavailable
  formula values, treats browser calculations as previews, and requests full
  native recalculation after formula-affecting edits.
- Added source hash checks and cooperating writer locks for XLSX edits and
  history restoration, with conflict responses for external changes.
- Added event sequence checkpoints, replay-gap resync, and slow-subscriber
  disconnects, plus browser recovery that pauses writes without blind retries.
- Added failure-injection, HTTP/SSE, and frontend recovery regression tests.
- Added regression tests for the ten-entry and 32 MiB history retention limits,
  restart boundaries, monotonic undo and redo revisions, and disabled browser
  controls when history actions are unavailable.
- Added a negative browser and MCP authorization matrix, rejected-origin
  no-mutation checks, credential and document-content non-disclosure tests, and
  a documented local credential-delivery boundary.
- Added clean-checkout package CI, generated bundle validation with failure-path
  tests, and an executable smoke test for absolute workbook paths with spaces.
- Added keyboard edit, rectangular paste, and server-owned history UI tests,
  accessible live save and conflict states, reduced-motion and safe-area rules,
  and responsive shell evidence from desktop, laptop, and mobile viewports.

- Isolated XLSX mutations and history restoration in candidates so failed
  persistence retains authoritative memory, revisions, and undo/redo state.
- Added an evidence-based XLSX stable release checklist with recovery, native
  fidelity, formula, host, and packaging gates and an explicit initial subset.
- Added a DOCX preservation experiment specification covering bounded edits,
  revision-scoped targets, fixtures, structural comparisons, and Word reopen gates.
- Added a shared product specification covering document lifecycle, revisioned
  collaboration, host permissions, proposed review, persistence, and recovery.
- Added a format support matrix separating current XLSX capabilities from
  planned DOCX, PPTX, HWPX, PDF, and binary HWP research, with conversion scope,
  compatibility evidence, and release gates.
- Added a Univer-powered spreadsheet UI behind a replaceable engine interface.
- Added Vite and Tailwind CSS build tooling without adding a React application
  dependency.
- Added human single-cell persistence and committed AI edit synchronization in
  the Univer adapter.
- Added an integration test for authenticated delivery of the embedded UI and
  its JavaScript assets.
- Added an architecture decision comparing Univer, ONLYOFFICE, and a custom
  spreadsheet engine.
- Added repository ignore rules for local workbooks, secrets, dependencies, and
  build artifacts.
- Added root monorepo commands for frontend, Go, build, test, audit, and local
  development workflows.
- Added an OSS-only dependency guard that rejects Univer Pro packages.
- Added contribution rules and GitHub templates for issues and pull requests.
- Ignored generated Vite assets while retaining the embedded Go build flow.
- Added a Go service foundation using the official MCP Go SDK and Excelize.
- Added `get_workbook`, `read_range`, and `apply_operations` MCP tools.
- Added revision-based conflict detection for workbook writes.
- Added same-directory temporary writes and atomic workbook replacement.
- Added an authenticated browser session and a token-protected MCP endpoint.
- Added a browser spreadsheet prototype with direct cell editing.
- Added an SSE stream for AI cursor, typing, and commit events.
- Added bidirectional human and AI range presence with OSS selection rendering.
- Added bounded SSE event replay using `Last-Event-ID`.
- Added atomic rectangular paste, basic cell formatting, and merge operations.
- Added initial XLSX style and merged-cell rendering in the Univer adapter.
- Added separate browser and MCP credentials for local role separation.
- Added workbook and operation count limits.
- Added workbook persistence and formula round-trip tests.
- Added repository architecture, product planning, and engineering guidance.
- Added an MCP App resource, `open_workbook` tool metadata, and app-only
  collaboration tools using the official MCP Apps bridge.
- Added separate browser and self-contained MCP App production bundles.
- Added an in-memory MCP protocol test for the interactive tool and resource.
- Added stdio MCP transport for local desktop hosts and a subprocess protocol
  test that exercises workbook reads through the real transport boundary.
- Added a host compatibility matrix and local Claude Desktop and Codex launch
  configuration examples.
- Added a repository-local architecture skill for coordinated Go, MCP, realtime,
  TypeScript, and Univer changes.
- Added runtime validation for workbook snapshots, ranges, operations responses,
  and realtime events received by the browser.
- Added a recorded Codex Desktop and CLI stdio certification covering workbook
  reads, approved writes, revision conflicts, and XLSX reopening.
- Added a recorded Claude Desktop and Code stdio certification covering tool
  reads, one-time approvals, writes, revision conflicts, and XLSX reopening.
- Added a stdio browser fallback that lets desktop hosts without MCP App
  rendering open the same workbook session in a local browser panel.
- Added a recorded Codex Browser panel certification for stdio fallback launch,
  human editing, AI presence and commits, and XLSX reopening.
- Added an integration test proving that browser and MCP edits share one
  revision sequence, ordered event log, and persisted XLSX file.
- Added the first XLSX compatibility corpus fixture and an automated unrelated
  edit round-trip test for formulas, styles, merges, validation, conditional
  formatting, drawings, links, names, and comments.
- Added a repository-local `xlsx-compatibility-qa` skill for safe corpus-based
  workbook compatibility verification.
- Added row and column insertion and deletion across MCP, HTTP, realtime events,
  XLSX persistence, and the Univer UI.
- Added bounded server-authoritative undo and redo with atomic package restore
  and revision reload events.
- Added viewport-triggered range loading beyond the initial browser data budget.
- Added compatibility detection and browser notices for charts, images,
  conditional formatting, data validation, external links, and macros.
- Added OSS Univer rendering for XLSX list validation and numeric conditional
  formatting rules loaded from the Go workbook adapter.
- Added a LibreOffice-produced workbook fixture to the compatibility corpus.
- Added restart-safe local undo and redo history with hash-addressed snapshots,
  atomic manifest recovery, and external workbook replacement detection.
- Added a bounded sheet object API for browser and MCP App image and chart
  previews.
- Added OSS Univer image rendering and custom read-only previews for bar, line,
  area, pie, and doughnut charts without adding Univer Pro dependencies.
- Added OSS Univer mappings for range-backed lists, literal whole-number,
  decimal, and date validation, custom-formula validation, prompts, and errors.
- Added OSS Univer rendering for common text, formula, rank, average,
  date-period, color-scale, data-bar, unique-value, and duplicate-value
  conditional formatting rules.
- Expanded the generated XLSX compatibility fixture and adapter assertions for
  validation metadata and conditional formatting parameters.
- Added revisioned image and chart operations for placement, resize, and
  deletion through the shared browser, HTTP, and MCP contracts.
- Added existing chart title updates for AI operations while preserving chart
  type, source references, series data, and unrelated workbook features.
- Added direct Univer drawing synchronization so human image and chart preview
  transforms persist to XLSX and remote object edits trigger an authoritative
  reload.

### 변경

- 저장소 소유자가 읽는 문서, GitHub 이슈 및 Pull Request 양식, 제품 문구의 기본
  언어를 한국어로 변경했습니다. AI 전용 지침, 코드 주석, 커밋 메시지, 명령어와
  API 식별자는 영어를 유지합니다.

- Restricted the project to a private local alpha until native Excel evidence,
  repeated invited-tester use, distribution terms, and an explicit owner
  decision support public source or binary distribution.
- Added a CI policy check that rejects automatic release triggers, repository
  contents write permission, and known GitHub Release publishers while the
  private-alpha policy is active.
- Required every repository change to use an issue-numbered branch and linked
  pull request, with templates and automated metadata policy checks.
- Renamed the product and repository architecture to `document-workspace` and
  repositioned XLSX as the first format adapter for a multi-format document
  collaboration platform.
- Reorganized the monorepo into `apps`, `cmd`, `internal/formats`,
  `internal/transport`, `testdata/xlsx`, and `legacy` boundaries while
  preserving existing workbook tools, revisions, events, and XLSX persistence.
- Versioned the XLSX MCP App resource under the document-workspace namespace
  and renamed the Go module, executable, frontend package, and local
  architecture skill.
- Defined the repository as a polyglot monorepo with Go as the authoritative
  workbook and collaboration service.
- Replaced the planned Univer Pro evaluation with an OSS-only presence and XLSX
  integration strategy owned by this repository.
- Replaced the Univer sheets preset with explicit OSS plugin registration and
  reduced the self-contained MCP App bundle from 12.7 MB to about 7.8 MB.
- Migrated JavaScript dependency management from npm to a pnpm workspace with a
  shared exact-version catalog for Univer packages.
- Versioned the MCP App resource URI, added server usage instructions, declared
  complete tool safety annotations, and added the OpenAI compatibility alias.
- Split browser orchestration, HTTP and MCP App transports, UI presentation,
  workbook mapping, and Univer command mapping into focused TypeScript modules.
- Split the Go HTTP adapter and workbook domain into focused files while
  preserving routes, tool contracts, event ordering, revision semantics, and
  atomic XLSX persistence.
- Replaced loose frontend operation and event interfaces with discriminated
  unions that encode required fields for each command and event type.
- Documented host-owned manual, automatic, one-time, and persistent approval
  behavior for workbook MCP tools.
- Updated compatibility notices to describe supported image and chart
  write-back controls and the remaining advanced chart editing limits.

### 수정

- Pinned and installed the legacy Python regression dependencies in CI so the
  complete verification suite works on a clean runner.
- Kept browser undo and redo availability synchronized after local edits,
  remote AI commits, history restoration, reload, and recovery transitions.
- Updated the browser revision after AI formatting and merge operations so the
  next human edit does not submit a stale base revision.
- Loaded worksheet data in bounded range chunks using server-reported sheet
  dimensions instead of limiting every sheet to the first 200 rows.
- Kept large or unsupported sheet objects from blocking workbook startup by
  omitting them with a visible compatibility notice.

### 보안

- Compared credentials through fixed-length hashes and added framing, resource,
  capability, base URI, object, and form restrictions to browser responses.
- Added XLSX package preflight limits for expanded bytes, individual parts, and
  entry count, with rejection of encrypted, duplicate, and unsafe ZIP entries.
- Updated Univer to 1.0.2 after dependency audit and verified that the installed
  dependency tree reports no known vulnerabilities.
- Allowed only the data and blob image, font, and worker sources required by
  Univer while keeping scripts restricted to same-origin assets.
- Added random session tokens, strict session cookies, browser Origin checks,
  request body limits, range size limits, and baseline security headers.
- Protected stdio browser fallback links with single-use two-minute launch
  tokens, loopback-only binding, and a bounded outstanding-token set.
- Validated embedded image bytes before rendering and allowed only decoded PNG,
  JPEG, and GIF data within the preview byte and object limits.
- Bounded durable history manifests, image dimensions, chart series, points,
  and labels before loading them into application memory.

## 0.0.1 - 2026-09-28

### Added

- Added the initial Python XLSX viewer and editor proof of concept.
- Added support for worksheet tabs, direct cell edits, list validation controls,
  and a limited subset of expression-based conditional formatting.

# Native Microsoft Excel Verification

## Purpose and Release Boundary

Use this procedure when a supported desktop version of Microsoft Excel is
available. Until the resulting evidence is recorded, X-07 native fidelity and
X-08 formula expectations remain blocked. Excelize or LibreOffice reopening is
useful automated evidence, but it is not a substitute for Microsoft Excel.

Always work on disposable copies. Never run document-workspace and Excel as
simultaneous writers to the same file. The service's source hash check cannot
eliminate the final check-to-rename race with a non-cooperating application.

## Before Starting

Record these facts in a copy of the
[native Excel evidence template](release-evidence/xlsx-native-excel-template.md):

- candidate commit and whether the working tree is clean;
- operating system version and architecture;
- Microsoft Excel product version, build, license channel, and architecture;
- locale, workbook calculation mode, and whether external links are allowed;
- source fixture path, provenance, redistribution permission, size, and SHA-256;
- document-workspace binary hash and transport or UI mode used for edits.

Use a non-sensitive fixture whose origin and redistribution rights are known.
Do not commit a customer workbook, personal workbook, credential, external-link
token, or workbook containing identifying metadata.

## Create a Microsoft Excel Fixture

1. Run `make verify` and `make compatibility-test` on a clean candidate.
2. Copy `testdata/xlsx/compatibility/feature-rich.xlsx` to a temporary directory.
   Do not open or overwrite the committed source fixture.
3. Record the source file size and SHA-256 before opening it.
4. Open the copy in Microsoft Excel. If Excel asks to update external links,
   choose **Do Not Update** by default and record the prompt and choice. Do not
   authorize an external connection merely to complete this procedure.
5. Review the compatibility sheet before saving. Record visible formulas,
   styles, merged cells, validation, conditional formatting, chart, image,
   hyperlink, defined name, comment, and any Excel repair or compatibility
   warning.
6. Use **Save As** and select the normal `.xlsx` format. Do not enable macros,
   encryption, or legacy compatibility mode. Use a new filename.
7. Fully close Excel, reopen the saved file in Excel, and repeat the inventory.
   Record every normalization, removal, repair prompt, or visible difference.
8. Record the Excel-produced file size and SHA-256. A changed package hash is
   expected after Save As and is not by itself a failure.
9. If the fixture is non-sensitive and redistributable, add it under
   `testdata/xlsx/compatibility` with a name such as
   `excel-macos-<major-version>.xlsx`. Add its exact provenance, Excel build,
   operating system, date, source hash, output hash, and known differences to
   the corpus README.
10. Add the filename to the fixture table in `compatibility_test.go`. Do not
    weaken inventory assertions to accept an unexplained loss.

If Excel removes a source feature during the initial Save As, keep both hashes
and the observed difference. Decide whether the Excel-produced fixture should
represent the retained subset, just as the LibreOffice fixture does. Do not
describe it as equivalent to the generated source.

## Native Edit and Reopen Matrix

Create a fresh disposable copy for each row. Inventory the workbook before the
edit, perform only the named operation through the real product entry point,
stop document-workspace, then reopen the file in Excel.

| Case | Product operation | Native Excel checks |
| --- | --- | --- |
| Cell | Set one unrelated value | Value persisted once; surrounding formula and style unchanged |
| Formula | Set one formula | Expression preserved; calculation behavior recorded |
| Dependency | Change a formula input | Full recalculation request observed; dependent result recorded |
| Paste | Paste a rectangular range | Shape, values, formulas, and atomicity preserved |
| Format | Change a basic supported format | Requested style visible; unrelated custom format unchanged |
| Merge | Merge and unmerge a disposable range | Range state matches; unrelated merges unchanged |
| Rows | Insert and delete rows | References, validation, rules, drawings, and surrounding data inspected |
| Columns | Insert and delete columns | References, validation, rules, drawings, and surrounding data inspected |
| Image | Move, resize, then delete an existing image | Placement and size persist; unrelated media remains |
| Chart | Move, resize, and retitle an existing chart | Title and placement persist; type, series, axes, legend, and style remain |
| History | Commit, undo, and redo | Final revision reopens to the expected state once |
| Conflict | Modify the source externally between session operations | Product refuses the stale write; Excel's external edit remains |

For every case record the source hash, saved hash, base and final revisions,
exact operation, expected result, observed result, Excel warnings, screenshots
or logs where appropriate, and pass, fail, or blocked status. A package parser
reopen cannot fill the native Excel result column.

## Formula Verification

Use separate disposable copies for each formula case:

1. An existing supported formula whose expression is not edited.
2. A changed formula expression.
3. A changed dependency cell.
4. A function unsupported by the browser preview but supported by Excel.
5. An external workbook reference with the external source unavailable.
6. A formula with a deliberately stale cached value.

Before editing, record the formula text, cached source status returned by the
product, browser preview if present, and Excel's displayed value. After editing,
stop the service and open the saved workbook in Excel. Record whether Excel
recalculated automatically, showed a security or link prompt, retained the
formula text, and produced a value. If automatic recalculation is disabled,
record that state before using Excel's **Calculate Now** or full recalculation
command. Do not report the browser preview as an Excel result.

An unavailable external source is an expected limitation when Excel cannot
resolve it. The pass condition is preservation of the reference, an honest
warning, and no fabricated authoritative value.

## Completion Rules

X-07 passes only when all supported edit families preserve their intended
changes and no unrelated inventoried feature is lost without an accepted,
documented restriction. X-08 passes only when formula expressions, dependency
changes, unsupported functions, external references, stale caches, and native
recalculation behavior have observed Excel results.

Any repair prompt, silent feature loss, wrong formula result, duplicated edit,
or stale write accepted as current is a failed gate. Preserve the failing copy
outside Git, open an issue with sanitized evidence, and do not change the result
to pass because Excel can manually repair the file.

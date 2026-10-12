# Native Microsoft Excel Evidence

Status: Not run. Copy this template to a dated Markdown file before use.

## Candidate and Environment

- Candidate commit:
- Working tree clean:
- Recorded at:
- Reviewer:
- Operating system and architecture:
- Excel product version and build:
- Excel license channel and architecture:
- Locale:
- Calculation mode:
- External-link policy:
- document-workspace binary SHA-256:
- Product entry point and transport:

## Fixture Provenance

- Fixture filename:
- Source application and version:
- Original source:
- Creation or Save As date:
- Redistribution permission:
- Sensitive-data review:
- Source size and SHA-256:
- Excel-produced size and SHA-256:
- Pre-existing Excel warnings or normalizations:

## Feature Inventory

| Feature | Before Excel Save As | After Excel Save As and reopen | Notes |
| --- | --- | --- | --- |
| Formulas and external reference | Not run | Not run | |
| Styles and number formats | Not run | Not run | |
| Merged cells | Not run | Not run | |
| Data validation | Not run | Not run | |
| Conditional formatting | Not run | Not run | |
| Chart | Not run | Not run | |
| Image | Not run | Not run | |
| Hyperlink | Not run | Not run | |
| Defined name | Not run | Not run | |
| Comment | Not run | Not run | |
| Excel repair or compatibility prompt | Not run | Not run | |

## Edit and Reopen Results

| Case | Source hash | Operation and revisions | Expected | Observed in Excel | Status | Evidence |
| --- | --- | --- | --- | --- | --- | --- |
| Cell | | | | Not run | Blocked | |
| Formula | | | | Not run | Blocked | |
| Dependency | | | | Not run | Blocked | |
| Paste | | | | Not run | Blocked | |
| Format | | | | Not run | Blocked | |
| Merge and unmerge | | | | Not run | Blocked | |
| Rows | | | | Not run | Blocked | |
| Columns | | | | Not run | Blocked | |
| Image | | | | Not run | Blocked | |
| Chart | | | | Not run | Blocked | |
| Undo and redo | | | | Not run | Blocked | |
| External conflict | | | | Not run | Blocked | |

## Formula Results

| Case | Formula text | Cached status before | Browser preview | Excel prompt and recalculation | Excel result | Status |
| --- | --- | --- | --- | --- | --- | --- |
| Existing formula | | | | Not run | Not run | Blocked |
| Changed formula | | | | Not run | Not run | Blocked |
| Changed dependency | | | | Not run | Not run | Blocked |
| Browser-unsupported function | | | | Not run | Not run | Blocked |
| External reference unavailable | | | | Not run | Not run | Blocked |
| Stale cached value | | | | Not run | Not run | Blocked |

## Differences and Restrictions

- Repair prompts:
- Unrelated feature differences:
- Expected producer normalizations:
- Accepted restrictions and linked issue:
- Unresolved failures:

## Sign-Off

- X-07 native fidelity: Blocked
- X-08 formula expectations: Blocked
- Reviewer conclusion:
- Follow-up issues:

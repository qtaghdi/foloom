#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
binary_path="$repository_root/bin/document-workspace"
fixture_path="$repository_root/testdata/xlsx/compatibility/libreoffice.xlsx"

if [ ! -x "$binary_path" ]; then
  echo "Package smoke test failed: executable is missing at $binary_path" >&2
  echo "Run make go-build after make web-build." >&2
  exit 1
fi

temporary_root=$(mktemp -d "${TMPDIR:-/tmp}/document-workspace-package.XXXXXX")
cleanup() {
  rm -rf "$temporary_root"
}
trap cleanup EXIT HUP INT TERM

workbook_directory="$temporary_root/path with spaces"
workbook_path="$workbook_directory/package fixture.xlsx"
mkdir "$workbook_directory"
cp "$fixture_path" "$workbook_path"

"$binary_path" -transport stdio -stdio-browser=false -file "$workbook_path" </dev/null

echo "Package smoke test passed: executable opened an absolute workbook path containing spaces"

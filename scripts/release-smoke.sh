#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
version=${1:-}
host_os=$(go env GOOS)
host_arch=$(go env GOARCH)
archive="$repository_root/dist/document-workspace_${version}_${host_os}_${host_arch}.tar.gz"

if [ ! -f "$archive" ]; then
  echo "Release smoke test failed: current-platform archive is missing at $archive" >&2
  exit 1
fi

temporary_root=$(mktemp -d "${TMPDIR:-/tmp}/document-workspace-release.XXXXXX")
cleanup() {
  rm -rf "$temporary_root"
}
trap cleanup EXIT HUP INT TERM

tar -xzf "$archive" -C "$temporary_root"
package_directory="$temporary_root/document-workspace_${version}_${host_os}_${host_arch}"
binary="$package_directory/document-workspace"
workbook="$temporary_root/release fixture.xlsx"
cp "$repository_root/testdata/xlsx/compatibility/libreoffice.xlsx" "$workbook"

"$binary" -version | grep -F "document-workspace $version" >/dev/null
"$binary" -check "$workbook" | grep -F "Workbook check passed" >/dev/null

echo "Release smoke test passed for $host_os/$host_arch"

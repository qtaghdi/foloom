#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
version=${1:-}
host_os=$(go env GOOS)
host_arch=$(go env GOARCH)
archive="$repository_root/dist/foloom_${version}_${host_os}_${host_arch}.tar.gz"

if [ ! -f "$archive" ]; then
  echo "Release smoke test failed: current-platform archive is missing at $archive" >&2
  exit 1
fi

temporary_root=$(mktemp -d "${TMPDIR:-/tmp}/foloom-release.XXXXXX")
cleanup() {
  rm -rf "$temporary_root"
}
trap cleanup EXIT HUP INT TERM

tar -xzf "$archive" -C "$temporary_root"
package_directory="$temporary_root/foloom_${version}_${host_os}_${host_arch}"
binary="$package_directory/foloom"
workbook="$temporary_root/release fixture.xlsx"
cp "$repository_root/testdata/xlsx/compatibility/libreoffice.xlsx" "$workbook"

"$binary" -version | grep -F "foloom $version" >/dev/null
"$binary" -check "$workbook" | grep -F "Workbook check passed" >/dev/null

if [ "$host_os" = darwin ]; then
  installer_home="$temporary_root/installer home"
  mkdir -p "$installer_home"
  HOME="$installer_home" "$binary" mcp install | grep -F "MCP와 foloom 스킬을 설치했습니다." >/dev/null
  HOME="$installer_home" "$binary" mcp status | grep -F "Codex MCP: 설치됨" >/dev/null
  test -x "$installer_home/Library/Application Support/foloom/bin/foloom"
  test -f "$installer_home/.codex/skills/foloom/SKILL.md"
  test -f "$installer_home/.claude/skills/foloom/SKILL.md"
  HOME="$installer_home" "$binary" mcp uninstall | grep -F "Codex MCP: 설치되지 않음" >/dev/null
  test ! -e "$installer_home/Library/Application Support/foloom/bin/foloom"
  test ! -e "$installer_home/.codex/skills/foloom/SKILL.md"
  test ! -e "$installer_home/.claude/skills/foloom/SKILL.md"
fi

echo "Release smoke test passed for $host_os/$host_arch"

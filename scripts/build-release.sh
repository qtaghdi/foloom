#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
version=${1:-}
release_targets=${RELEASE_TARGETS:-"darwin_amd64 darwin_arm64 linux_amd64 linux_arm64 windows_amd64 windows_arm64"}
dist_directory="$repository_root/dist"

case "$version" in
  ""|*[!0-9A-Za-z.v+-]*)
    echo "Usage: scripts/build-release.sh <version>" >&2
    echo "Version may contain letters, numbers, dots, plus signs, and hyphens." >&2
    exit 2
    ;;
esac

commit=$(git -C "$repository_root" rev-parse --short=12 HEAD 2>/dev/null || printf unknown)
build_date=$(git -C "$repository_root" show -s --format=%cI HEAD 2>/dev/null || printf unknown)
ldflags="-s -w -X main.version=$version -X main.commit=$commit -X main.buildDate=$build_date"

rm -rf "$dist_directory"
mkdir -p "$dist_directory"

for target in $release_targets; do
  os=${target%%_*}
  arch=${target#*_}
  if [ "$os" = "$target" ] || [ -z "$arch" ]; then
    echo "Invalid release target $target. Use os_arch." >&2
    exit 2
  fi

  package_name="document-workspace_${version}_${os}_${arch}"
  stage_directory="$dist_directory/$package_name"
  binary_name=document-workspace
  if [ "$os" = windows ]; then
    binary_name=document-workspace.exe
  fi

  mkdir -p "$stage_directory/docs"
  cp "$repository_root/README.md" "$stage_directory/README.md"
  cp "$repository_root/docs/getting-started.md" "$stage_directory/docs/getting-started.md"
  cp "$repository_root/docs/docx-editing.md" "$stage_directory/docs/docx-editing.md"
  cp "$repository_root/docs/format-support.md" "$stage_directory/docs/format-support.md"
  cp "$repository_root/docs/host-compatibility.md" "$stage_directory/docs/host-compatibility.md"
  cp "$repository_root/docs/security.md" "$stage_directory/docs/security.md"

  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    GOTOOLCHAIN="${GO_TOOLCHAIN:-go1.25.0}" \
    GOCACHE="${GO_CACHE:-/tmp/document-workspace-go-cache}" \
    go build -trimpath -ldflags "$ldflags" \
    -o "$stage_directory/$binary_name" "$repository_root/cmd/document-workspace"

  if [ "$os" = windows ]; then
    (cd "$dist_directory" && zip -X -q -r "$package_name.zip" "$package_name")
  else
    (cd "$dist_directory" && tar -czf "$package_name.tar.gz" "$package_name")
  fi
  rm -rf "$stage_directory"
done

(cd "$dist_directory" && find . -maxdepth 1 -type f \( -name '*.tar.gz' -o -name '*.zip' \) -print | sort | xargs shasum -a 256 > SHA256SUMS)
echo "Release archives created in $dist_directory"

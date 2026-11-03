#!/bin/sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
dist_directory="$repository_root/dist"
release_targets=${RELEASE_TARGETS:-"darwin_amd64 darwin_arm64 linux_amd64 linux_arm64 windows_amd64 windows_arm64"}

if [ ! -f "$dist_directory/SHA256SUMS" ]; then
  echo "Release verification failed: dist/SHA256SUMS is missing." >&2
  exit 1
fi

(cd "$dist_directory" && shasum -a 256 -c SHA256SUMS)

archive_count=0
for target in $release_targets; do
  os=${target%%_*}
  arch=${target#*_}
  archive=$(find "$dist_directory" -maxdepth 1 -type f \( \
    -name "document-workspace_*_${os}_${arch}.tar.gz" -o \
    -name "document-workspace_*_${os}_${arch}.zip" \) | head -n 1)
  if [ -z "$archive" ]; then
    echo "Release verification failed: archive for $os/$arch is missing." >&2
    exit 1
  fi

  archive_count=$((archive_count + 1))
  filename=$(basename "$archive")
  package_name=${filename%.tar.gz}
  package_name=${package_name%.zip}
  binary_name=document-workspace
  if [ "$os" = windows ]; then
    binary_name=document-workspace.exe
    entries=$(unzip -Z1 "$archive")
  else
    entries=$(tar -tzf "$archive")
  fi

  for required in \
    "$package_name/$binary_name" \
    "$package_name/README.md" \
    "$package_name/docs/getting-started.md" \
    "$package_name/docs/format-support.md" \
    "$package_name/docs/docx-editing.md" \
    "$package_name/docs/host-compatibility.md" \
    "$package_name/docs/security.md"; do
    if ! printf '%s\n' "$entries" | grep -Fx "$required" >/dev/null; then
      echo "Release verification failed: $filename does not contain $required." >&2
      exit 1
    fi
  done
done

expected_count=$(printf '%s\n' $release_targets | wc -l | tr -d ' ')
actual_count=$(find "$dist_directory" -maxdepth 1 -type f \( -name '*.tar.gz' -o -name '*.zip' \) | wc -l | tr -d ' ')
if [ "$archive_count" -ne "$expected_count" ] || [ "$actual_count" -ne "$expected_count" ]; then
  echo "Release verification failed: expected $expected_count archives, found $actual_count." >&2
  exit 1
fi

echo "Release verification passed for $actual_count archives"

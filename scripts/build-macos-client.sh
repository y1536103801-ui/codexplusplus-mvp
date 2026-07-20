#!/bin/sh
set -eu

release=0
if [ "${1:-}" = "--release" ]; then
  release=1
elif [ "$#" -ne 0 ]; then
  echo "usage: $0 [--release]" >&2
  exit 2
fi

if [ "$(uname -s)" != "Darwin" ]; then
  echo "the macOS client must be built on macOS running on Apple hardware" >&2
  exit 3
fi

major="$(sw_vers -productVersion | cut -d. -f1)"
if [ -z "$major" ] || [ "$major" -lt 14 ]; then
  echo "macOS 14 or newer is required" >&2
  exit 4
fi

if ! cargo tauri --version >/dev/null 2>&1; then
  echo "cargo-tauri is required" >&2
  exit 5
fi

if [ "$release" -eq 1 ]; then
  if [ -z "${APPLE_SIGNING_IDENTITY:-}" ]; then
    echo "APPLE_SIGNING_IDENTITY is required for a public release" >&2
    exit 6
  fi
  if [ -z "${APPLE_API_KEY:-}" ] && { [ -z "${APPLE_ID:-}" ] || [ -z "${APPLE_PASSWORD:-}" ] || [ -z "${APPLE_TEAM_ID:-}" ]; }; then
    echo "Apple notarization credentials are required for a public release" >&2
    exit 7
  fi
else
  : "${APPLE_SIGNING_IDENTITY:=-}"
  export APPLE_SIGNING_IDENTITY
  echo "building an ad-hoc signed test package; do not publish this artifact"
fi

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$repo_root/desktop-client/src-tauri"
cargo test
cargo tauri build --bundles app,dmg

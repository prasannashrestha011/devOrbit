#!/usr/bin/env bash
set -euo pipefail

case "${1:-}" in
  linux) bundle=deb; expected=Linux ;;
  windows) bundle=nsis; expected=Windows ;;
  macos) bundle=dmg; expected=Darwin ;;
  *) echo "Usage: ./build.sh linux|windows|macos" >&2; exit 2 ;;
esac
if [[ $# -ne 1 ]]; then
  echo "Usage: ./build.sh linux|windows|macos" >&2
  exit 2
fi

case "$(uname -s)" in
  Linux) actual=Linux ;;
  MINGW*|MSYS*|CYGWIN*) actual=Windows ;;
  Darwin) actual=Darwin ;;
  *) actual=unknown ;;
esac
if [[ "$actual" != "$expected" ]]; then
  echo "Build ${1} installers on ${1} (current OS: $actual)." >&2
  exit 2
fi

cd "$(dirname "${BASH_SOURCE[0]}")/desktop"
if [[ ! -f node_modules/@tauri-apps/cli/tauri.js ]]; then npm ci; fi
npm run tauri -- icon src-tauri/icons/icon.png --output src-tauri/target/icons
if [[ "$1" == macos ]]; then export APPLE_SIGNING_IDENTITY="${APPLE_SIGNING_IDENTITY:--}"; fi
npm run tauri -- build --bundles "$bundle" --config '{"bundle":{"icon":["target/icons/icon.png","target/icons/icon.ico","target/icons/icon.icns"]}}' -- --locked

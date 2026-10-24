#!/usr/bin/env bash
set -euo pipefail

root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
for tool in go npm cargo; do
  if ! command -v "$tool" >/dev/null; then
    echo "Missing $tool; see README.md for desktop prerequisites" >&2
    exit 1
  fi
done
if [[ ! -x "$root/desktop/node_modules/.bin/tauri" ]]; then
  echo "Run 'cd desktop && npm ci' first" >&2
  exit 1
fi

cd "$root/desktop"
npm run tauri -- dev

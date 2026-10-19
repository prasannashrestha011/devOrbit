#!/usr/bin/env bash
set -euo pipefail

root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

for tool in go npm cargo curl; do
  if ! command -v "$tool" >/dev/null; then
    echo "Missing $tool; see README.md for desktop prerequisites" >&2
    exit 1
  fi
done
if [[ ! -x "$root/desktop/node_modules/.bin/tauri" ]]; then
  echo "Run 'cd desktop && npm ci' first" >&2
  exit 1
fi

build_dir="$(mktemp -d)"
server_pid=""
cleanup() {
  if [[ -n "$server_pid" ]]; then
    kill -TERM "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -f -- "$build_dir/devorbit"
  rmdir -- "$build_dir"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

(cd "$root/server" && go build -o "$build_dir/devorbit" ./cmd/devorbit)
"$build_dir/devorbit" &
server_pid=$!

host="${DEVORBIT_HOST:-127.0.0.1}"
if [[ "$host" == *:* ]]; then host="[$host]"; fi
url="http://$host:${DEVORBIT_PORT:-7456}/health"
ready=false
for ((attempt = 0; attempt < 50; attempt++)); do
  if curl -fsS --noproxy '*' --max-time 1 -o /dev/null "$url" 2>/dev/null; then
    ready=true
    break
  fi
  if ! kill -0 "$server_pid" 2>/dev/null; then
    echo "DevOrbit backend exited before it became ready" >&2
    exit 1
  fi
  sleep 0.2
done
if [[ "$ready" != true ]]; then
  echo "DevOrbit backend did not become ready at $url" >&2
  exit 1
fi

echo "Backend ready at $url; starting desktop"
cd "$root/desktop"
npm run tauri -- dev

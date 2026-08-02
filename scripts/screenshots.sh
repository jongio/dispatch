#!/usr/bin/env bash
# Generate or check website screenshots from macOS/Linux shells.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

out_dir="web/public/screenshots"
check=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --out)
      if [[ $# -lt 2 ]]; then
        echo "--out requires a directory" >&2
        exit 2
      fi
      out_dir="$2"
      shift 2
      ;;
    --out=*)
      out_dir="${1#--out=}"
      shift
      ;;
    --check)
      check=true
      if [[ "$out_dir" == "web/public/screenshots" ]]; then
        out_dir=".screenshots-check"
      fi
      shift
      ;;
    -h|--help)
      echo "Usage: scripts/screenshots.sh [--out DIR] [--check]"
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

if [[ "$check" == true ]]; then
  rm -rf "$out_dir"
  go run -tags screenshots ./cmd/screenshots --check --out "$out_dir"
  rm -rf "$out_dir"
  echo "Screenshot capture check passed."
  exit 0
fi

go run -tags screenshots ./cmd/screenshots --out "$out_dir"
node cmd/screenshots/render.mjs "$out_dir"

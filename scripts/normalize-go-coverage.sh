#!/usr/bin/env bash
set -euo pipefail

INPUT=${1:-coverage.out}
OUTPUT=${2:-coverage.sonar.out}

if [[ ! -s "$INPUT" ]]; then
  echo "coverage input is missing or empty: $INPUT" >&2
  exit 1
fi

MODULE_PATH=$(awk '$1 == "module" { print $2; exit }' go.mod)
if [[ -z "$MODULE_PATH" ]]; then
  echo "unable to resolve module path from go.mod" >&2
  exit 1
fi

{
  IFS= read -r header
  printf '%s\n' "$header"

  while IFS= read -r line; do
    case "$line" in
      "$MODULE_PATH"/*)
        printf '%s\n' "${line#"$MODULE_PATH"/}"
        ;;
      *)
        printf '%s\n' "$line"
        ;;
    esac
  done
} < "$INPUT" > "$OUTPUT"

if grep -Fq "$MODULE_PATH/" "$OUTPUT"; then
  echo "coverage normalization left module-prefixed paths in $OUTPUT" >&2
  exit 1
fi

missing=0
while IFS= read -r path; do
  [[ -z "$path" ]] && continue
  if [[ "$path" = /* ]]; then
    candidate="$path"
  else
    candidate="./$path"
  fi
  if [[ ! -f "$candidate" ]]; then
    echo "coverage references missing source file: $path" >&2
    missing=1
  fi
done < <(tail -n +2 "$OUTPUT" | cut -d: -f1 | sort -u)

if [[ "$missing" -ne 0 ]]; then
  exit 1
fi

printf 'normalized Go coverage for Sonar: %s -> %s\n' "$INPUT" "$OUTPUT"

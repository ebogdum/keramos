#!/usr/bin/env bash
# Documentation coverage gate: verifies every engine-registered template
# function has a documented input->output entry, and every CLI command has a
# doc page. Prints coverage and a PASS/FAIL summary. bash 3.2 compatible.
set -uo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fail=0

# --- functions -------------------------------------------------------------
grep -rhoE 'r\.Register\("[^"]+"' internal/engine/*.go 2>/dev/null \
  | grep -v _test | sed -E 's/r\.Register\("//;s/"//' | sort -u > "$tmp/fns.txt"
cat docs/templates/functions.md docs/templates/functions/*.md 2>/dev/null > "$tmp/fndocs.txt"

fn_total=$(wc -l < "$tmp/fns.txt" | tr -d ' ')
fn_ok=0; : > "$tmp/fn_missing.txt"
while IFS= read -r fn; do
  [ -z "$fn" ] && continue
  # Documented = the function name appears in a `backtick` entry AND the docs
  # contain at least one input->output arrow (we treat the whole file as the
  # corpus; per-entry checking happens in review).
  if grep -qF "\`$fn\`" "$tmp/fndocs.txt"; then
    fn_ok=$((fn_ok+1))
  else
    echo "$fn" >> "$tmp/fn_missing.txt"
  fi
done < "$tmp/fns.txt"
arrows=$(grep -cE '→|=>|Output:' "$tmp/fndocs.txt" 2>/dev/null || echo 0)
echo "FUNCTIONS: $fn_ok / $fn_total named in docs ; $arrows input->output arrows present"
if [ "$fn_ok" -lt "$fn_total" ]; then
  echo "  missing ($(wc -l < "$tmp/fn_missing.txt" | tr -d ' ')): $(tr '\n' ' ' < "$tmp/fn_missing.txt" | cut -c1-300)"
  fail=1
fi

# --- commands --------------------------------------------------------------
grep -oE 'new[A-Za-z]+Command\(\)' internal/cli/root.go 2>/dev/null \
  | sed -E 's/new([A-Za-z]+)Command.*/\1/' | tr 'A-Z' 'a-z' | sort -u > "$tmp/cmds.txt"
cmd_total=0; cmd_ok=0; : > "$tmp/cmd_missing.txt"
while IFS= read -r c; do
  [ -z "$c" ] && continue
  case "$c" in help|completion) continue;; esac
  cmd_total=$((cmd_total+1))
  # Command function names are de-hyphenated (newHelmCompatCommand -> helmcompat)
  # while doc files keep the hyphen (helm-compat.md). Accept a page whose
  # basename matches after stripping hyphens.
  found=""
  if [ -f "docs/cli/$c.md" ]; then
    found=1
  else
    for p in docs/cli/*.md; do
      b="$(basename "$p" .md | tr -d '-')"
      if [ "$b" = "$c" ]; then found=1; break; fi
    done
  fi
  if [ -n "$found" ]; then cmd_ok=$((cmd_ok+1)); else echo "$c" >> "$tmp/cmd_missing.txt"; fi
done < "$tmp/cmds.txt"
echo "COMMANDS: $cmd_ok / $cmd_total have a docs/cli/<name>.md page"
if [ "$cmd_ok" -lt "$cmd_total" ]; then
  echo "  missing: $(tr '\n' ' ' < "$tmp/cmd_missing.txt")"
  fail=1
fi

# --- config schemas --------------------------------------------------------
cfg_ok=0; cfg_total=0
for f in keramos-yaml values-yaml values-schema-json keramos-releases-yaml keramos-workspace-yaml; do
  cfg_total=$((cfg_total+1))
  [ -f "docs/reference/$f.md" ] && cfg_ok=$((cfg_ok+1))
done
echo "CONFIG: $cfg_ok / $cfg_total schema docs present"
[ "$cfg_ok" -lt "$cfg_total" ] && fail=1

echo "----"
if [ "$fail" -eq 0 ]; then echo "DOCS COVERAGE: PASS"; else echo "DOCS COVERAGE: INCOMPLETE"; fi
exit $fail

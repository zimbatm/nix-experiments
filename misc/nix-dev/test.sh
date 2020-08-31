#!/usr/bin/env bash
set -euo pipefail
banner() {
  echo
  echo --- "$*"
  echo
}

banner BATS
bats --recursive ./test

banner ShellCheck

while IFS= read -r -d '' file; do

  pushd "$(dirname "$file")" >/dev/null

  shellcheck -x -a "$(basename "$file")"

  popd >/dev/null

done < <(find . -path ./.git -prune -o -type f -executable -print0)

echo OK

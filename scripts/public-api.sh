#!/usr/bin/env bash
set -euo pipefail

modules=(
  "."
  "./cmd/servex"
  "./llm/adapter/eino"
  "./llm/adapter/adk"
  "./testx/container"
)

for module in "${modules[@]}"; do
  (
    cd "${module}"
    echo "# module ${module}"
    go list ./... | while read -r pkg; do
      echo
      echo "## ${pkg}"
      go doc "${pkg}" 2>/dev/null | sed -n '/^const /p;/^var /p;/^func [A-Z]/p;/^type [A-Z]/p'
    done
  )
  echo
done

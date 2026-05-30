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
  echo "== ${module}: govulncheck ./... =="
  (cd "${module}" && go run golang.org/x/vuln/cmd/govulncheck@latest ./...)
done

echo "vulnerability checks passed"

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
  echo "== ${module}: go test ./... =="
  (cd "${module}" && go test ./...)

  echo "== ${module}: go vet ./... =="
  (cd "${module}" && go vet ./...)

  echo "== ${module}: go build ./... =="
  (cd "${module}" && go build ./...)
done

echo "workspace checks passed"

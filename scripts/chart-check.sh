#!/usr/bin/env bash
# Lint, render every codec branch, and validate the manifests. A chart whose only tested path is the
# default one has an untested path for every value a user sets.
set -euo pipefail
cd "$(dirname "$0")/.."

HELM=${HELM:-helm}
KUBECONFORM=${KUBECONFORM:-kubeconform}
if ! command -v "$KUBECONFORM" >/dev/null 2>&1; then
  echo "  kubeconform not found; installing to .cache/bin"
  mkdir -p .cache/bin
  ver=v0.6.7
  curl -sSfL "https://github.com/yannh/kubeconform/releases/download/${ver}/kubeconform-linux-amd64.tar.gz" \
    | tar xz -C .cache/bin kubeconform
  KUBECONFORM="$PWD/.cache/bin/kubeconform"
fi

"$HELM" lint charts/wire-codec --strict

n=0
for codec in byte json proto flatbuf; do
  out=$("$HELM" template wire-codec charts/wire-codec --set codec="$codec")
  echo "$out" | "$KUBECONFORM" -strict -summary -kubernetes-version 1.29.0 - >/dev/null
  echo "  codec=$codec ✓"
  n=$((n + 1))
done
echo "  $n/$n branches render and validate"

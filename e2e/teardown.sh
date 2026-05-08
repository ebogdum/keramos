#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "Tearing down k3s cluster..."
docker compose -f "$SCRIPT_DIR/docker-compose.yml" down -v
rm -f "$SCRIPT_DIR/kubeconfig.yaml"
echo "Done."

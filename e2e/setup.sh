#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
KUBECONFIG_PATH="$SCRIPT_DIR/kubeconfig.yaml"

echo "Starting k3s cluster..."
docker compose -f "$SCRIPT_DIR/docker-compose.yml" up -d

echo "Waiting for k3s to be healthy..."
for i in $(seq 1 60); do
    if docker compose -f "$SCRIPT_DIR/docker-compose.yml" exec -T k3s kubectl get nodes 2>/dev/null | grep -q " Ready"; then
        echo "k3s is ready."
        break
    fi
    if [ "$i" -eq 60 ]; then
        echo "Timed out waiting for k3s"
        exit 1
    fi
    sleep 2
done

echo "Extracting kubeconfig..."
docker compose -f "$SCRIPT_DIR/docker-compose.yml" exec -T k3s cat /output/kubeconfig.yaml \
    | sed 's|https://127.0.0.1:6443|https://127.0.0.1:6443|' \
    > "$KUBECONFIG_PATH"

# Replace the internal hostname with localhost
sed -i.bak 's|server: .*|server: https://127.0.0.1:6443|' "$KUBECONFIG_PATH" && rm -f "$KUBECONFIG_PATH.bak"

echo "Kubeconfig written to $KUBECONFIG_PATH"
echo "Run tests with: KUBECONFIG=$KUBECONFIG_PATH go test -v ./e2e/ -timeout 5m"

#!/usr/bin/env bash
#
# Build all example images, verify they work, and print a size comparison table.
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PORT_BASE=18080
WAIT_SECONDS=3

# Image names
IMAGES=(
    "go-https-original"
    "go-https-chiseled"
    "go-https-distroless"
    "python-api-original"
    "python-api-chiseled"
    "python-api-distroless"
)

cleanup() {
    echo ""
    echo "Cleaning up containers..."
    for name in "${IMAGES[@]}"; do
        docker rm -f "$name" 2>/dev/null || true
    done
}
trap cleanup EXIT

# ── Build ────────────────────────────────────────────────────────────────────

echo "=== Building Go HTTPS images ==="
docker build -t go-https-original   -f "$SCRIPT_DIR/go-https/Dockerfile.original"   "$SCRIPT_DIR/go-https"
docker build -t go-https-chiseled   -f "$SCRIPT_DIR/go-https/Dockerfile.chiseled"   "$SCRIPT_DIR/go-https"
docker build -t go-https-distroless -f "$SCRIPT_DIR/go-https/Dockerfile.distroless" "$SCRIPT_DIR/go-https"

echo ""
echo "=== Building Python API images ==="
docker build -t python-api-original   -f "$SCRIPT_DIR/python-api/Dockerfile.original"   "$SCRIPT_DIR/python-api"
docker build -t python-api-chiseled   -f "$SCRIPT_DIR/python-api/Dockerfile.chiseled"   "$SCRIPT_DIR/python-api"
docker build -t python-api-distroless -f "$SCRIPT_DIR/python-api/Dockerfile.distroless" "$SCRIPT_DIR/python-api"

# ── Verify ───────────────────────────────────────────────────────────────────

echo ""
echo "=== Verifying images ==="

verify() {
    local name=$1
    local port=$2

    docker run -d --name "$name" -p "$port:8080" "$name" >/dev/null
    sleep "$WAIT_SECONDS"

    local healthz
    healthz=$(curl -sf "http://localhost:$port/healthz" 2>/dev/null || echo "FAIL")
    if [ "$healthz" = "ok" ]; then
        echo "  $name — healthz OK (port $port)"
    else
        echo "  $name — healthz FAILED (port $port)"
        return 1
    fi

    # Test TLS fetch
    local fetch
    fetch=$(curl -sf "http://localhost:$port/fetch" 2>/dev/null || echo "FAIL")
    if [ "$fetch" != "FAIL" ] && [ -n "$fetch" ]; then
        echo "  $name — /fetch OK (TLS works)"
    else
        echo "  $name — /fetch FAILED (TLS may be broken)"
    fi
}

port=$PORT_BASE
for name in "${IMAGES[@]}"; do
    verify "$name" "$port" || true
    port=$((port + 1))
done

# ── Size Table ───────────────────────────────────────────────────────────────

echo ""
echo "=== Image Size Comparison ==="
echo ""

# Helper: get image size in bytes
imgsize() {
    docker image inspect "$1" --format='{{.Size}}'
}

# Helper: format bytes to human-readable
human() {
    local bytes=$1
    if [ "$bytes" -ge 1073741824 ]; then
        printf "%.1f GB" "$(echo "scale=1; $bytes / 1073741824" | bc)"
    elif [ "$bytes" -ge 1048576 ]; then
        printf "%.1f MB" "$(echo "scale=1; $bytes / 1048576" | bc)"
    elif [ "$bytes" -ge 1024 ]; then
        printf "%.1f KB" "$(echo "scale=1; $bytes / 1024" | bc)"
    else
        printf "%d B" "$bytes"
    fi
}

# Helper: print row with optional % reduction vs a baseline
row() {
    local name=$1
    local size=$2
    local baseline=${3:-}
    local pct="—"
    if [ -n "$baseline" ] && [ "$baseline" -gt 0 ]; then
        pct=$(printf "%.0f%%" "$(echo "scale=2; (1 - $size / $baseline) * 100" | bc)")
    fi
    printf "%-24s %12s %12s\n" "$name" "$(human "$size")" "$pct"
}

printf "%-24s %12s %12s\n" "IMAGE" "SIZE" "REDUCTION"
printf "%-24s %12s %12s\n" "------------------------" "------------" "------------"

# Go section
go_orig=$(imgsize go-https-original)
go_chisel=$(imgsize go-https-chiseled)
go_distro=$(imgsize go-https-distroless)
row "go-https-original"   "$go_orig"
row "go-https-chiseled"   "$go_chisel"  "$go_orig"
row "go-https-distroless" "$go_distro"  "$go_orig"
echo ""

# Python section
py_orig=$(imgsize python-api-original)
py_chisel=$(imgsize python-api-chiseled)
py_distro=$(imgsize python-api-distroless)
row "python-api-original"   "$py_orig"
row "python-api-chiseled"   "$py_chisel"  "$py_orig"
row "python-api-distroless" "$py_distro"  "$py_orig"

echo ""
echo "Done."

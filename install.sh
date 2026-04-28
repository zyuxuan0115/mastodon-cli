#!/usr/bin/env bash
# Build masto-cli for the current machine and install it to a $PATH directory.
# Supports macOS (darwin) and Linux on amd64/arm64.
#
# Usage:
#   ./install.sh                    # build + install to /usr/local/bin (uses sudo if needed)
#   PREFIX=$HOME/.local ./install.sh  # install to ~/.local/bin instead
#   ./install.sh --build-only       # build only, leave binary in ./dist/

set -euo pipefail

BIN_NAME="masto"
PREFIX="${PREFIX:-/usr/local}"
INSTALL_DIR="$PREFIX/bin"
BUILD_ONLY=0

for arg in "$@"; do
    case "$arg" in
        --build-only) BUILD_ONLY=1 ;;
        -h|--help)
            sed -n '2,9p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *)
            echo "unknown argument: $arg" >&2
            exit 2
            ;;
    esac
done

if ! command -v go >/dev/null 2>&1; then
    echo "error: go is not installed or not on PATH" >&2
    echo "install Go 1.22+ from https://go.dev/dl/ and re-run this script" >&2
    exit 1
fi

OS="$(uname -s)"
ARCH="$(uname -m)"
case "$OS" in
    Darwin) GOOS=darwin ;;
    Linux)  GOOS=linux ;;
    *) echo "error: unsupported OS: $OS (this script supports macOS and Linux)" >&2; exit 1 ;;
esac
case "$ARCH" in
    x86_64|amd64) GOARCH=amd64 ;;
    arm64|aarch64) GOARCH=arm64 ;;
    *) echo "error: unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

mkdir -p dist
OUT="dist/${BIN_NAME}-${GOOS}-${GOARCH}"
echo "==> building $OUT (GOOS=$GOOS GOARCH=$GOARCH)"
GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$OUT" .

if [ "$BUILD_ONLY" -eq 1 ]; then
    echo "==> built $OUT"
    exit 0
fi

DEST="$INSTALL_DIR/$BIN_NAME"
echo "==> installing to $DEST"
if [ -w "$INSTALL_DIR" ] || { [ ! -e "$INSTALL_DIR" ] && [ -w "$(dirname "$INSTALL_DIR")" ]; }; then
    mkdir -p "$INSTALL_DIR"
    install -m 0755 "$OUT" "$DEST"
else
    echo "    $INSTALL_DIR is not writable; using sudo"
    sudo mkdir -p "$INSTALL_DIR"
    sudo install -m 0755 "$OUT" "$DEST"
fi

echo "==> installed: $(command -v "$BIN_NAME" 2>/dev/null || echo "$DEST")"
case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *)
        echo
        echo "note: $INSTALL_DIR is not on your \$PATH."
        echo "      add this line to your shell profile (~/.zshrc, ~/.bashrc, etc.):"
        echo "          export PATH=\"$INSTALL_DIR:\$PATH\""
        ;;
esac

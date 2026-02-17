#!/usr/bin/env bash
set -euo pipefail

BINARY="dfc"
INSTALL_DIR="${HOME}/.local/bin"

echo "Building ${BINARY}..."
go build -o "${BINARY}" ./cmd/dfc

mkdir -p "${INSTALL_DIR}"
mv "${BINARY}" "${INSTALL_DIR}/${BINARY}"

echo "Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"

# Check PATH
if [[ ":${PATH}:" != *":${INSTALL_DIR}:"* ]]; then
  echo ""
  echo "⚠  ${INSTALL_DIR} is not in your PATH."
  echo "   Add this to your shell profile:"
  echo "   export PATH=\"\${HOME}/.local/bin:\${PATH}\""
fi

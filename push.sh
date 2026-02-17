#!/usr/bin/env bash
set -euo pipefail

# Push DFC changes to both GitHub (standalone) and Bitbucket (monorepo)

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GITHUB_REPO="${SCRIPT_DIR}"
MONOREPO="/Users/solarisjon/src/Jons-Misc-Projects-Repo"
MSG="${1:-}"

cd "${GITHUB_REPO}"

# Check for uncommitted changes
if [[ -n "$(git status --porcelain)" ]]; then
  if [[ -z "${MSG}" ]]; then
    echo "Usage: $0 \"commit message\""
    echo "  (there are uncommitted changes)"
    exit 1
  fi
  git add -A
  git commit -m "${MSG}"
fi

# Push to GitHub
echo "⬆  Pushing to GitHub..."
git push
echo "✓  GitHub done"

# Push monorepo to Bitbucket (symlink means files are already there)
echo "⬆  Pushing monorepo to Bitbucket..."
cd "${MONOREPO}"
if [[ -n "$(git status --porcelain)" ]]; then
  git add -A
  git commit -m "${MSG:-sync DFC changes}"
fi
git push
echo "✓  Bitbucket done"

echo ""
echo "Both repos pushed successfully."

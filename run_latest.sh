#!/usr/bin/env bash

set -e

# --- CONFIG ---
REPO="YOUR_USERNAME/YOUR_REPO"
ARTIFACT_NAME="tvfeed-linux-amd64"
GITHUB_TOKEN="${GITHUB_TOKEN}"

if [ -z "$GITHUB_TOKEN" ]; then
  echo "❌ Please export GITHUB_TOKEN first"
  exit 1
fi

echo "🔍 Fetching latest workflow run..."

RUN_ID=$(curl -s -H "Authorization: token $GITHUB_TOKEN" \
  "https://api.github.com/repos/$REPO/actions/runs?per_page=1" \
  | jq -r '.workflow_runs[0].id')

echo "Latest run ID: $RUN_ID"

echo "📦 Fetching artifact..."

ARTIFACT_ID=$(curl -s -H "Authorization: token $GITHUB_TOKEN" \
  "https://api.github.com/repos/$REPO/actions/runs/$RUN_ID/artifacts" \
  | jq -r ".artifacts[] | select(.name==\"$ARTIFACT_NAME\") | .id")

echo "Artifact ID: $ARTIFACT_ID"

echo "⬇️ Downloading..."

curl -L -H "Authorization: token $GITHUB_TOKEN" \
  -o artifact.zip \
  "https://api.github.com/repos/$REPO/actions/artifacts/$ARTIFACT_ID/zip"

echo "📂 Extracting..."
unzip -o artifact.zip

echo "🚀 Running..."

chmod +x tvfeed

./tvfeed "$@"

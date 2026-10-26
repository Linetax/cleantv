#!/usr/bin/env bash
# Builds cleantv-installer.exe (and Linux/macOS binaries) with the package lists, the
# launcher APK and SmartTube embedded. Needs Go and launcher/build/dumbtv-home.apk
# (./launcher/build.sh). SmartTube is downloaded unless SMARTTUBE points at a local copy.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
APK=${APK:-../launcher/build/dumbtv-home.apk}
[[ -f $APK ]] || { echo "error: $APK missing — run ./launcher/build.sh first" >&2; exit 1; }
rm -rf assets && mkdir -p assets
cp -r ../data/fireos ../data/fireos6 assets/
cp "$APK" assets/dumbtv-home.apk

# Pinned: releases are immutable, the hash identifies the build. Universal runs on 32- and
# 64-bit Fire OS alike.
ST_URL=https://github.com/yuliskov/SmartTube/releases/download/32.68s/SmartTube_stable_32.68_universal.apk
ST_SHA=45a36e62110bcb866527ba8e9ab876f4122e1143647bbd0a97362d13803f6b94
if [[ -n ${SMARTTUBE:-} ]]; then cp "$SMARTTUBE" assets/smarttube.apk
else curl -fsSL --retry 4 --retry-all-errors -o assets/smarttube.apk "$ST_URL"; fi
got=$(sha256sum assets/smarttube.apk | awk '{print $1}')
[[ $got == "$ST_SHA" ]] || { echo "error: SmartTube checksum mismatch (got $got)" >&2; exit 1; }
go vet ./...
go test ./...
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags=-s -o cleantv-installer.exe .
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags=-s -o cleantv-installer-linux .
GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags=-s -o cleantv-installer-macos .
ls -l cleantv-installer*

#!/usr/bin/env bash
# Builds cleantv-installer.exe (and Linux/macOS binaries) with the package lists and the
# launcher APK embedded. Needs Go and launcher/build/dumbtv-home.apk (./launcher/build.sh).
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
APK=${APK:-../launcher/build/dumbtv-home.apk}
[[ -f $APK ]] || { echo "error: $APK missing — run ./launcher/build.sh first" >&2; exit 1; }
rm -rf assets && mkdir -p assets
cp -r ../data/fireos ../data/fireos6 assets/
cp "$APK" assets/dumbtv-home.apk
go vet ./...
go test ./...
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags=-s -o cleantv-installer.exe .
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags=-s -o cleantv-installer-linux .
GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags=-s -o cleantv-installer-macos .
ls -l cleantv-installer*

#!/usr/bin/env bash
# Builds cineflux/build/cineflux.apk, the Cineflux TV app (a WebView on cineflux.linetax.party),
# for the installer to embed. The sources are a copy of github.com/Linetax/Cineflux (commit
# 751e3bb): copy them again here when that repository changes, and raise VERSION.
# Same tools and signing key as launcher/build.sh (an Android SDK with platforms;android-25).
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
VERSION=1

SDK=${ANDROID_HOME:-${ANDROID_SDK_ROOT:?ANDROID_HOME missing}}
JAR=$SDK/platforms/android-25/android.jar
BT=$(find "$SDK/build-tools" -mindepth 1 -maxdepth 1 -type d | sort -V | tail -1)
KEYSTORE=${DUMBTV_KEYSTORE:-$PWD/../launcher/release.keystore}
KEYPASS=${DUMBTV_KEYPASS:-dumbtvhome}
[[ -f $KEYSTORE ]] || { echo "error: $KEYSTORE missing — run ./launcher/build.sh first" >&2; exit 1; }

WORK=$(mktemp -d "${TMPDIR:-/tmp}/cineflux.XXXXXX")
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/classes" build

"$BT/aapt2" compile --dir res -o "$WORK/res.zip"
"$BT/aapt2" link -I "$JAR" --manifest AndroidManifest.xml "$WORK/res.zip" \
  --min-sdk-version 22 --target-sdk-version 30 \
  --version-code "$VERSION" --version-name "1.$VERSION" -o "$WORK/app.apk"
javac --release 8 -nowarn -Xlint:-options -encoding UTF-8 -classpath "$JAR" -d "$WORK/classes" \
  src/party/linetax/cineflux/*.java
"$BT/d8" --release --min-api 22 --lib "$JAR" --output "$WORK" $(find "$WORK/classes" -name '*.class')
(cd "$WORK" && zip -q -X app.apk classes.dex)
"$BT/zipalign" -f -p 4 "$WORK/app.apk" "$WORK/aligned.apk"
"$BT/apksigner" sign --ks "$KEYSTORE" --ks-pass "pass:$KEYPASS" --key-pass "pass:$KEYPASS" \
  --out build/cineflux.apk "$WORK/aligned.apk"
"$BT/apksigner" verify build/cineflux.apk
echo "built build/cineflux.apk"

#!/usr/bin/env bash
# Drives the Linux build of the installer through each fake device in fake-adb and checks
# what it prints and which adb commands it sends. Usage: installer/e2e/run.sh BINARY
set -uo pipefail
bin=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
fails=0

# run MODE INPUT: runs the installer, leaves its output in $out and the adb log in $log.
run() {
  local mode=$1 input=$2 dir
  dir=$(mktemp -d)
  mkdir -p "$dir/bin" "$dir/cache"
  ln -s "$here/fake-adb" "$dir/bin/adb"
  out=$(printf "$input" | FAKE_DIR=$dir FAKE_MODE=$mode XDG_CACHE_HOME=$dir/cache HOME=$dir \
    PATH="$dir/bin:$PATH" timeout 300 "$bin" 2>&1)
  log=$(cat "$dir/log" 2>/dev/null)
  pushed=$(cat "$dir/pushed" 2>/dev/null)
  rm -rf "$dir"
}
has() { grep -qF -- "$2" <<<"$1"; }
check() { # check NAME CONDITION...
  local name=$1; shift
  if "$@"; then echo "ok    $name"; else echo "FAIL  $name"; fails=$((fails + 1)); fi
}

# Answers: Enter (TV ready), IP, menu choice, then the questions of that path, then Enter to close.
IP='203.0.113.7'

run fireos8 "\n$IP\n1\n\nn\n\n"
check "fire os 8: disables an ad package" has "$log" "pm disable-user --user 0 com.amazon.hybridadidservice"
check "fire os 8: never touches Amazon's launcher" bash -c '! grep -q "disable-user --user 0 com.amazon.tv.launcher" <<<"$0"' "$log"
check "fire os 8: installs SmartTube before the home screen" bash -c 'grep -n "install -r" <<<"$0" | head -1 | grep -q smarttube' "$log"
check "fire os 8: takes over HOME" has "$out" "bouton Home : cleantv"
check "fire os 8: alarm min_futurity=0" has "$log" "alarm_manager_constants min_futurity=0"
check "fire os 8: SmartTube first on a fresh home screen" has "$pushed" "apps.order=org.smarttube.stable"

run slowauth "\n$IP\n3\n\n"
check "slow authorization: asks to accept on the TV" has "$out" "Acceptez le message"
check "slow authorization: then connects" has "$out" "Connecté."

run dead "\n$IP\n\n"
check "silent TV: says it stopped answering" has "$out" "La TV ne répond plus"
check "silent TV: not mistaken for another device" bash -c '! grep -q "pas une Fire TV" <<<"$0"' "$out"

run hang "\n$IP\n\n"
check "TV that hangs adb: gives up with a clear message" has "$out" "La TV ne répond plus"

run notfire "\n$IP\n\n"
check "not Fire OS: refused" has "$out" "pas une Fire TV"

run oldkey "\n$IP\n1\n\n\nn\n\n"
check "home screen signed with another key: replaced after asking" has "$log" "uninstall tv.dumbtv.home"
check "home screen signed with another key: order left alone" test -z "$pushed"

run fireos6 "\n$IP\n1\n\nn\n\n"
check "fire os 6: usage-access mode" has "$out" "mode Fire OS 6"
check "fire os 6: no accessibility takeover" bash -c '! grep -q "set-home-activity" <<<"$0"' "$log"

run fireos8 "\n$IP\n2\nn\n\n"
check "restore: re-enables a disabled package" has "$log" "pm enable --user 0 com.amazon.shoptv.client"
check "restore: hands HOME back to Amazon" has "$log" "am start -a android.intent.action.MAIN -c android.intent.category.HOME"

echo
[[ $fails == 0 ]] && echo "all scenarios passed" || echo "$fails check(s) failed"
exit $((fails > 0))

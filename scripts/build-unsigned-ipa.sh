#!/usr/bin/env bash
# Humming 鈥?macOS / Xcode 鏈湴鏈鍚?IPA 鎵撳寘鑴氭湰
# 渚濊禆锛歑code銆丟o 1.22+銆乆codeGen锛坆rew install xcodegen锛?# 杈撳嚭锛歞ist/Humming-Unsigned.ipa 涓?.sha256
#
# 鐢ㄦ硶锛?#   ./scripts/build-unsigned-ipa.sh
#   ./scripts/build-unsigned-ipa.sh --skip-test
#   ./scripts/build-unsigned-ipa.sh --xcode /Applications/Xcode.app
#   ./scripts/build-unsigned-ipa.sh --clean

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT}"

SKIP_TEST=0
DO_CLEAN=0
XCODE_APP="${HUMMING_XCODE:-}"
CONFIGURATION="${HUMMING_CONFIGURATION:-Release}"
DERIVED_DATA="${HUMMING_DERIVED_DATA:-${ROOT}/build}"
DIST_DIR="${HUMMING_DIST:-${ROOT}/dist}"
IPA_NAME="Humming-Unsigned.ipa"
SCHEME="Humming"
PROJECT="Humming.xcodeproj"
APP_PATH="${DERIVED_DATA}/Build/Products/${CONFIGURATION}-iphoneos/Humming.app"

usage() {
  cat <<'EOF'
Humming 鏈鍚?IPA 鎵撳寘鑴氭湰锛堜粎 macOS + Xcode锛?
鐢ㄦ硶:
  ./scripts/build-unsigned-ipa.sh [閫夐」]

閫夐」:
  --skip-test          璺宠繃 Simulator XCTest
  --clean              娓呯悊 build/ 涓?dist/ 鍚庨噸鏂版墦鍖?  --xcode <璺緞>       鎸囧畾 Xcode.app锛堥粯璁よ嚜鍔ㄩ€夋嫨锛?  --configuration <鍚? Debug 鎴?Release锛堥粯璁?Release锛?  -h, --help           鏄剧ず甯姪

鐜鍙橀噺:
  HUMMING_XCODE           鍚?--xcode
  HUMMING_CONFIGURATION   鍚?--configuration
  HUMMING_DERIVED_DATA    DerivedData 鐩綍锛堥粯璁?./build锛?  HUMMING_DIST            杈撳嚭鐩綍锛堥粯璁?./dist锛?  CGO_ENABLED             寤鸿涓?0锛堣剼鏈細寮哄埗璁剧疆锛?
娉ㄦ剰:
  浜у嚭涓烘湭绛惧悕 IPA锛屼笉鑳界洿鎺ュ畨瑁呭埌璁惧銆?EOF
}

log() { printf '\n==> %s\n' "$*"; }
die() { printf '閿欒: %s\n' "$*" >&2; exit 1; }

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "鏈壘鍒板懡浠? $1"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-test) SKIP_TEST=1; shift ;;
    --clean) DO_CLEAN=1; shift ;;
    --xcode)
      [[ $# -ge 2 ]] || die "--xcode 闇€瑕佽矾寰勫弬鏁?
      XCODE_APP="$2"
      shift 2
      ;;
    --configuration)
      [[ $# -ge 2 ]] || die "--configuration 闇€瑕佸弬鏁?
      CONFIGURATION="$2"
      APP_PATH="${DERIVED_DATA}/Build/Products/${CONFIGURATION}-iphoneos/Humming.app"
      shift 2
      ;;
    -h|--help) usage; exit 0 ;;
    *) die "鏈煡鍙傛暟: $1锛堜娇鐢?--help 鏌ョ湅鐢ㄦ硶锛? ;;
  esac
done

[[ "$(uname -s)" == "Darwin" ]] || die "鏈剼鏈粎鏀寔 macOS"

export CGO_ENABLED=0

require_cmd go
require_cmd xcodebuild
require_cmd xcrun
require_cmd plutil
require_cmd lipo
require_cmd otool
require_cmd unzip
require_cmd shasum

if ! command -v xcodegen >/dev/null 2>&1; then
  die "鏈壘鍒?xcodegen銆傝鍏堟墽琛? brew install xcodegen"
fi

select_xcode() {
  local candidate=""
  if [[ -n "${XCODE_APP}" ]]; then
    candidate="${XCODE_APP}"
  elif [[ -d /Applications/Xcode_16.4.app ]]; then
    candidate="/Applications/Xcode_16.4.app"
  elif [[ -d /Applications/Xcode.app ]]; then
    candidate="/Applications/Xcode.app"
  else
    candidate="$(ls -d /Applications/Xcode*.app 2>/dev/null | head -n 1 || true)"
  fi
  [[ -n "${candidate}" && -d "${candidate}" ]] || die "鏈壘鍒板彲鐢?Xcode.app"
  if [[ "$(xcode-select -p)" != "${candidate}/Contents/Developer" ]]; then
    log "鍒囨崲 Xcode: ${candidate}"
    if [[ -w /usr/bin/xcode-select ]] || sudo -n true 2>/dev/null; then
      sudo xcode-select -s "${candidate}"
    else
      printf '璇锋墜鍔ㄦ墽琛? sudo xcode-select -s "%s"\n' "${candidate}" >&2
      die "褰撳墠 Xcode 寮€鍙戣€呯洰褰曚笌鐩爣涓嶄竴鑷?
    fi
  fi
}

print_toolchain() {
  log "宸ュ叿閾句俊鎭?
  sw_vers
  xcodebuild -version
  xcodebuild -showsdks | grep -E 'iphoneos|iphonesimulator' || true
  swift --version
  go version
  xcodegen --version || true
}

generate_config() {
  log "鐢熸垚骞舵牎楠岄厤缃?
  go run ./cmd/hummingctl generate
  go run ./cmd/hummingctl validate
}

generate_project() {
  log "鐢?XcodeGen 鐢熸垚宸ョ▼"
  xcodegen generate
  xcodebuild -list -project "${PROJECT}"
}

pick_simulator_destination() {
  local line dest udid
  line="$(xcodebuild -project "${PROJECT}" -scheme "${SCHEME}" -showdestinations 2>/dev/null \
    | grep 'platform:iOS Simulator' | grep 'id:' | head -n 1 || true)"
  if [[ -n "${line}" ]]; then
    dest="$(printf '%s' "${line}" | sed -E 's/.*platform:([^,}]+).*id:([^,}]+).*/platform=\1,id=\2/' | tr -d ' ')"
    printf '%s' "${dest}"
    return 0
  fi
  udid="$(xcrun simctl list devices available | grep -E 'iPhone' | grep -oE '[A-F0-9-]{36}' | head -n 1 || true)"
  if [[ -n "${udid}" ]]; then
    printf 'platform=iOS Simulator,id=%s' "${udid}"
    return 0
  fi
  return 1
}

run_tests() {
  log "杩愯 XCTest锛圫imulator锛?
  local dest
  if ! dest="$(pick_simulator_destination)"; then
    xcodebuild -project "${PROJECT}" -scheme "${SCHEME}" -showdestinations || true
    xcrun simctl list devices available || true
    die "鏈壘鍒板彲鐢?iOS Simulator"
  fi
  echo "浣跨敤娴嬭瘯鐩殑鍦? ${dest}"
  xcodebuild test \
    -project "${PROJECT}" \
    -scheme "${SCHEME}" \
    -destination "${dest}" \
    -derivedDataPath "${DERIVED_DATA}/test" \
    CODE_SIGNING_ALLOWED=NO \
    CODE_SIGNING_REQUIRED=NO \
    CODE_SIGN_IDENTITY=""
}

build_device_app() {
  log "缂栬瘧 ${CONFIGURATION} arm64 鐪熸満鍖?
  xcodebuild \
    -project "${PROJECT}" \
    -scheme "${SCHEME}" \
    -configuration "${CONFIGURATION}" \
    -sdk iphoneos \
    -destination 'generic/platform=iOS' \
    -derivedDataPath "${DERIVED_DATA}" \
    ARCHS=arm64 \
    ONLY_ACTIVE_ARCH=NO \
    CODE_SIGNING_ALLOWED=NO \
    CODE_SIGNING_REQUIRED=NO \
    CODE_SIGN_IDENTITY="" \
    build

  [[ -d "${APP_PATH}" ]] || {
    find "${DERIVED_DATA}" -name '*.app' -type d || true
    die "鏈壘鍒颁骇鐗? ${APP_PATH}"
  }
}

package_ipa() {
  log "鎵撳寘鏈鍚?IPA"
  mkdir -p "${DIST_DIR}"
  go run ./cmd/hummingctl package --app "${APP_PATH}" --output "${DIST_DIR}/${IPA_NAME}"
}

verify_ipa() {
  local ipa="${DIST_DIR}/${IPA_NAME}"
  local bundle_id exec_name version build bin

  log "鏍￠獙 IPA / Mach-O"
  plutil -p "${APP_PATH}/Info.plist" >/dev/null

  bundle_id="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "${APP_PATH}/Info.plist")"
  exec_name="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "${APP_PATH}/Info.plist")"
  version="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "${APP_PATH}/Info.plist")"
  build="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' "${APP_PATH}/Info.plist")"

  echo "Bundle ID: ${bundle_id}"
  echo "Executable: ${exec_name}"
  echo "Version: ${version} (${build})"

  [[ "${bundle_id}" == "com.glhpx.humming" ]] || die "Bundle ID 涓嶅尮閰? ${bundle_id}"
  [[ "${exec_name}" == "Humming" ]] || die "Executable 涓嶅尮閰? ${exec_name}"

  bin="${APP_PATH}/${exec_name}"
  lipo -info "${bin}"
  lipo -info "${bin}" | grep -q arm64 || die "缂哄皯 arm64 鏋舵瀯"

  if command -v vtool >/dev/null 2>&1; then
    vtool -show-build "${bin}" 2>/dev/null || vtool -show "${bin}" 2>/dev/null || true
  fi
  otool -l "${bin}" | grep -A3 LC_BUILD_VERSION || otool -l "${bin}" | grep -A3 LC_VERSION_MIN_IPHONEOS || true

  if otool -l "${bin}" | grep -qi 'platform ios-simulator'; then
    die "妫€娴嬪埌妯℃嫙鍣ㄥ钩鍙颁骇鐗╋紝鎷掔粷鎵撳寘"
  fi

  unzip -l "${ipa}" | head -n 40
  unzip -l "${ipa}" | grep -q 'Payload/Humming.app/' || die "IPA 缂哄皯 Payload/Humming.app/"
  unzip -l "${ipa}" | grep -q 'Payload/Humming.app/Info.plist' || die "IPA 缂哄皯 Info.plist"

  [[ -f "${ipa}.sha256" ]] || die "缂哄皯 SHA-256 鏂囦欢"
  (
    cd "${DIST_DIR}"
    shasum -a 256 -c "${IPA_NAME}.sha256"
  )

  log "瀹屾垚"
  echo "IPA: ${ipa}"
  echo "SHA: ${ipa}.sha256"
  cat "${ipa}.sha256"
  echo
  echo "璀﹀憡: 鏈鍚?IPA 涓嶅彲鐩存帴瀹夎锛岄渶浼佷笟璇佷功涓?In-House Profile 鍚庡啀绛惧悕鍒嗗彂銆?
}

main() {
  if [[ "${DO_CLEAN}" -eq 1 ]]; then
    log "娓呯悊鏋勫缓浜х墿"
    rm -rf "${DERIVED_DATA}" "${DIST_DIR}" "${ROOT}/${PROJECT}"
  fi

  select_xcode
  print_toolchain
  generate_config
  generate_project

  if [[ "${SKIP_TEST}" -eq 0 ]]; then
    run_tests
  else
    log "宸茶烦杩?XCTest"
  fi

  build_device_app
  package_ipa
  verify_ipa
}

main

#!/usr/bin/env bash
# Humming — macOS / Xcode local unsigned IPA build script
# Default Xcode: official App Store install at /Applications/Xcode.app
# Requires: Xcode, Go 1.22+, XcodeGen (brew install xcodegen)
# Output: dist/Humming-Unsigned.ipa and .sha256
#
# Usage:
#   ./scripts/build-unsigned-ipa.sh
#   ./scripts/build-unsigned-ipa.sh --skip-test
#   ./scripts/build-unsigned-ipa.sh --clean
#   ./scripts/build-unsigned-ipa.sh --xcode /Applications/Xcode.app

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
Humming unsigned IPA build script (macOS + Xcode only)

Usage:
  ./scripts/build-unsigned-ipa.sh [options]

Options:
  --skip-test          Skip Simulator XCTest
  --clean              Clean build/ and dist/ before packaging
  --xcode <path>       Xcode.app path (default: /Applications/Xcode.app)
  --configuration <n>  Debug or Release (default: Release)
  -h, --help           Show help

Notes:
  Defaults to official App Store Xcode at /Applications/Xcode.app.
  Output is an UNSIGNED IPA and cannot be installed directly.
EOF
}

log() { printf '\n==> %s\n' "$*"; }
die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "command not found: $1"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-test) SKIP_TEST=1; shift ;;
    --clean) DO_CLEAN=1; shift ;;
    --xcode)
      [[ $# -ge 2 ]] || die "--xcode requires a path"
      XCODE_APP="$2"
      shift 2
      ;;
    --configuration)
      [[ $# -ge 2 ]] || die "--configuration requires a value"
      CONFIGURATION="$2"
      APP_PATH="${DERIVED_DATA}/Build/Products/${CONFIGURATION}-iphoneos/Humming.app"
      shift 2
      ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown argument: $1 (use --help)" ;;
  esac
done

[[ "$(uname -s)" == "Darwin" ]] || die "this script only runs on macOS"

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
  die "xcodegen not found. Install with: brew install xcodegen"
fi

select_xcode() {
  local candidate=""
  # Prefer official App Store Xcode
  if [[ -n "${XCODE_APP}" ]]; then
    candidate="${XCODE_APP}"
  elif [[ -d /Applications/Xcode.app ]]; then
    candidate="/Applications/Xcode.app"
  else
    candidate="$(ls -d /Applications/Xcode*.app 2>/dev/null | head -n 1 || true)"
  fi
  [[ -n "${candidate}" && -d "${candidate}" ]] || die "Xcode.app not found (official install is /Applications/Xcode.app)"

  local developer_dir="${candidate}/Contents/Developer"
  [[ -d "${developer_dir}" ]] || die "invalid Xcode path: ${candidate}"

  local current
  current="$(xcode-select -p 2>/dev/null || true)"
  if [[ "${current}" == "${developer_dir}" ]]; then
    log "Using Xcode: ${candidate}"
    return 0
  fi

  log "Switching developer dir to: ${candidate}"
  sudo xcode-select -s "${candidate}"
  xcode-select -p
}

print_toolchain() {
  log "Toolchain"
  sw_vers
  xcodebuild -version
  xcodebuild -showsdks | grep -E 'iphoneos|iphonesimulator' || true
  swift --version
  go version
  xcodegen --version || true
}

generate_config() {
  log "Generate and validate config"
  go run ./cmd/hummingctl generate
  go run ./cmd/hummingctl validate
}

generate_project() {
  log "Generate Xcode project with XcodeGen"
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
  log "Run XCTest on Simulator"
  local dest
  if ! dest="$(pick_simulator_destination)"; then
    xcodebuild -project "${PROJECT}" -scheme "${SCHEME}" -showdestinations || true
    xcrun simctl list devices available || true
    die "no available iOS Simulator"
  fi
  echo "Simulator destination: ${dest}"
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
  log "Build ${CONFIGURATION} arm64 device app"
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
    die "app not found: ${APP_PATH}"
  }
}

package_ipa() {
  log "Package unsigned IPA"
  mkdir -p "${DIST_DIR}"
  go run ./cmd/hummingctl package --app "${APP_PATH}" --output "${DIST_DIR}/${IPA_NAME}"
}

verify_ipa() {
  local ipa="${DIST_DIR}/${IPA_NAME}"
  local bundle_id exec_name version build bin

  log "Verify IPA / Mach-O"
  plutil -p "${APP_PATH}/Info.plist" >/dev/null

  bundle_id="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIdentifier' "${APP_PATH}/Info.plist")"
  exec_name="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "${APP_PATH}/Info.plist")"
  version="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "${APP_PATH}/Info.plist")"
  build="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleVersion' "${APP_PATH}/Info.plist")"

  echo "Bundle ID: ${bundle_id}"
  echo "Executable: ${exec_name}"
  echo "Version: ${version} (${build})"

  [[ "${bundle_id}" == "com.glhpx.humming" ]] || die "Bundle ID mismatch: ${bundle_id}"
  [[ "${exec_name}" == "Humming" ]] || die "Executable mismatch: ${exec_name}"

  bin="${APP_PATH}/${exec_name}"
  lipo -info "${bin}"
  lipo -info "${bin}" | grep -q arm64 || die "arm64 arch missing"

  if command -v vtool >/dev/null 2>&1; then
    vtool -show-build "${bin}" 2>/dev/null || vtool -show "${bin}" 2>/dev/null || true
  fi
  otool -l "${bin}" | grep -A3 LC_BUILD_VERSION || otool -l "${bin}" | grep -A3 LC_VERSION_MIN_IPHONEOS || true

  if otool -l "${bin}" | grep -qi 'platform ios-simulator'; then
    die "simulator build detected; refusing to package"
  fi

  unzip -l "${ipa}" | head -n 40
  unzip -l "${ipa}" | grep -q 'Payload/Humming.app/' || die "IPA missing Payload/Humming.app/"
  unzip -l "${ipa}" | grep -q 'Payload/Humming.app/Info.plist' || die "IPA missing Info.plist"

  [[ -f "${ipa}.sha256" ]] || die "SHA-256 file missing"
  (
    cd "${DIST_DIR}"
    shasum -a 256 -c "${IPA_NAME}.sha256"
  )

  log "Done"
  echo "IPA: ${ipa}"
  echo "SHA: ${ipa}.sha256"
  cat "${ipa}.sha256"
  echo
  echo "WARNING: Unsigned IPA cannot be installed directly. Enterprise cert + In-House profile required."
}

main() {
  if [[ "${DO_CLEAN}" -eq 1 ]]; then
    log "Clean build artifacts"
    rm -rf "${DERIVED_DATA}" "${DIST_DIR}" "${ROOT}/${PROJECT}"
  fi

  select_xcode
  print_toolchain
  generate_config
  generate_project

  if [[ "${SKIP_TEST}" -eq 0 ]]; then
    run_tests
  else
    log "Skipping XCTest"
  fi

  build_device_app
  package_ipa
  verify_ipa
}

main

#!/bin/sh
# Generate Windows VERSIONINFO .syso for cmd/smartctl-exporter.
# Usage: gen-versioninfo.sh <version> <commit> <goarch> <outfile>
set -eu

VERSION="${1:?version}"
COMMIT="${2:?commit}"
GOARCH="${3:?goarch}"
OUT="${4:?outfile}"

VER="${VERSION#v}"
MAJOR=$(printf '%s' "$VER" | cut -d. -f1)
MINOR=$(printf '%s' "$VER" | cut -d. -f2)
PATCH=$(printf '%s' "$VER" | cut -d. -f3)
PATCH=${PATCH:-0}
MAJOR=${MAJOR:-0}
MINOR=${MINOR:-0}

ARCHFLAGS="-64"
if [ "$GOARCH" = "arm64" ]; then
  ARCHFLAGS="-64 -arm"
fi

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

# goversioninfo requires a JSON input; CLI flags override these fields.
cat >"$TMPDIR/versioninfo.json" <<EOF
{
  "FixedFileInfo": {
    "FileVersion": {"Major": ${MAJOR}, "Minor": ${MINOR}, "Patch": ${PATCH}, "Build": 0},
    "ProductVersion": {"Major": ${MAJOR}, "Minor": ${MINOR}, "Patch": ${PATCH}, "Build": 0},
    "FileFlagsMask": "3f",
    "FileFlags": "00",
    "FileOS": "040004",
    "FileType": "01",
    "FileSubType": "00"
  },
  "StringFileInfo": {
    "Comments": "",
    "CompanyName": "",
    "FileDescription": "smartctl-exporter",
    "FileVersion": "${VER}.0",
    "InternalName": "smartctl-exporter",
    "LegalCopyright": "",
    "LegalTrademarks": "",
    "OriginalFilename": "smartctl-exporter.exe",
    "PrivateBuild": "",
    "ProductName": "smartctl-exporter",
    "ProductVersion": "${VER} (${COMMIT})",
    "SpecialBuild": ""
  },
  "VarFileInfo": {
    "Translation": {"LangID": "0409", "CharsetID": "04B0"}
  }
}
EOF

# Build goversioninfo for the host (ARG/env GOARCH must not cross-compile the tool).
# shellcheck disable=SC2086
GOOS="$(go env GOHOSTOS)" GOARCH="$(go env GOHOSTARCH)" \
  go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.4.1 \
  $ARCHFLAGS \
  -o "$OUT" \
  "$TMPDIR/versioninfo.json"

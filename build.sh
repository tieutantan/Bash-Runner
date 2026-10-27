#!/usr/bin/env bash
# Packages Bash Runner as "Bash Runner.app" in the project folder.
set -euo pipefail

cd "$(dirname "$0")"

APP_NAME="Bash Runner"
ICON="icon.png"

# Quit the running app before building so the open copy is not overwritten. Skipped if it is not running.
pkill -f "$APP_NAME.app/Contents/MacOS/" 2>/dev/null || true

WAILS="$(go env GOPATH)/bin/wails"
if [[ ! -x "$WAILS" ]]; then
	echo "Installing the wails tool..."
	go install github.com/wailsapp/wails/v2/cmd/wails@latest
fi

# Wails takes the icon from build/appicon.png.
cp "$ICON" build/appicon.png
"$WAILS" build -clean -platform darwin/arm64 -s

rm -rf "$APP_NAME.app"
mv "build/bin/$APP_NAME.app" "$APP_NAME.app"

# Wails upscales icon.png to 1024px when it creates the icns, which makes the icon blurry.
# Rebuild the icns only from the sizes available in icon.png, without upscaling.
ICONSET="$(mktemp -d)/icon.iconset"
mkdir -p "$ICONSET"
SRC_SIZE="$(sips -g pixelWidth "$ICON" | awk '/pixelWidth/ {print $2}')"
for base in 16 32 128 256 512; do
	if (( base <= SRC_SIZE )); then
		sips -z "$base" "$base" "$ICON" --out "$ICONSET/icon_${base}x${base}.png" >/dev/null
	fi
	if (( base * 2 <= SRC_SIZE )); then
		sips -z "$((base * 2))" "$((base * 2))" "$ICON" --out "$ICONSET/icon_${base}x${base}@2x.png" >/dev/null
	fi
done
iconutil -c icns "$ICONSET" -o "$APP_NAME.app/Contents/Resources/icon.icns"
rm -rf "$(dirname "$ICONSET")"

echo "Done: $(pwd)/$APP_NAME.app"

# Reopen the app after the build finishes.
open "$APP_NAME.app"

#!/usr/bin/env bash
# Đóng gói Bash Runner thành "Bash Runner.app" trong thư mục dự án.
set -euo pipefail

cd "$(dirname "$0")"

APP_NAME="Bash Runner"
ICON="icon.png"

# Tắt app đang chạy trước khi build để không ghi đè lên bản đang mở. Không chạy thì bỏ qua.
pkill -f "$APP_NAME.app/Contents/MacOS/" 2>/dev/null || true

WAILS="$(go env GOPATH)/bin/wails"
if [[ ! -x "$WAILS" ]]; then
	echo "Đang cài công cụ wails..."
	go install github.com/wailsapp/wails/v2/cmd/wails@latest
fi

# Wails lấy icon từ build/appicon.png.
cp "$ICON" build/appicon.png
"$WAILS" build -clean -platform darwin/arm64 -s

rm -rf "$APP_NAME.app"
mv "build/bin/$APP_NAME.app" "$APP_NAME.app"

# Wails phóng to icon.png lên 1024px khi tạo icns nên icon bị mờ.
# Tạo lại icns chỉ từ các kích thước có sẵn trong icon.png, không phóng to.
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

echo "Xong: $(pwd)/$APP_NAME.app"

# Mở lại app sau khi build xong.
open "$APP_NAME.app"

#!/usr/bin/env bash
# Đóng gói Bash Runner thành "Bash Runner.app" trong thư mục dự án.
set -euo pipefail

cd "$(dirname "$0")"

APP_NAME="Bash Runner"
APP_ID="com.tantn.bashrunner"
APP_VERSION="1.0.0"
ICON="icon.png"

# Tắt app đang chạy trước khi build để không ghi đè lên bản đang mở. Không chạy thì bỏ qua.
pkill -f "$APP_NAME.app/Contents/MacOS/" 2>/dev/null || true

FYNE="$(go env GOPATH)/bin/fyne"
if [[ ! -x "$FYNE" ]]; then
	echo "Đang cài công cụ fyne..."
	go install fyne.io/tools/cmd/fyne@latest
fi

"$FYNE" package \
	-os darwin \
	-name "$APP_NAME" \
	-app-id "$APP_ID" \
	-icon "$ICON" \
	-app-version "$APP_VERSION" \
	-release

# fyne phóng to icon.png lên 1024px khi tạo icns nên icon bị mờ.
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

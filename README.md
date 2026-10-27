# Bash-Runner

App macOS (viết bằng Go + Wails, giao diện HTML/CSS) để chạy các file `.sh` bằng nút bấm, thay vì phải mở Terminal gõ `bash`.

## Tính năng

- Bấm **Thêm file** để chọn file `.sh` ở bất kỳ thư mục nào trên máy.
- Mỗi file hiện trong danh sách với tên hiển thị. Đường dẫn đầy đủ chỉ hiện khi file không còn tồn tại (xem thêm trong menu **⋮** của file).
- Bấm nút **▶** (icon Run) để chạy file bằng `/bin/bash`, thư mục làm việc là thư mục chứa file đó.
- Bấm nút **⋮** bên cạnh file để mở menu:
  - **Chạy với tham số…**: nhập tham số (ví dụ `--env staging "thư mục mới"`) để chạy một lần, hoặc bấm **Lưu** và đặt tên để giữ lại lần chạy đó.
  - **Đổi tên hiển thị**: đặt tên khác cho file trong app. Tên file gốc trên ổ đĩa không đổi.
  - **Xóa khỏi danh sách**: bỏ file khỏi app. File gốc không bị xóa.
- Các lần chạy có tham số đã lưu hiện thụt lề ngay dưới file gốc, có nút **▶** và menu **⋮** riêng (sửa tên/tham số hoặc xóa).
- Output (stdout và stderr) hiện ở khung bên dưới, kèm exit code khi chạy xong.
- Danh sách và tên hiển thị được lưu lại giữa các lần mở app.

## Yêu cầu

- macOS
- Go 1.21 trở lên
- Xcode Command Line Tools (Wails cần cgo): `xcode-select --install`

## Build thành app macOS

```bash
./build.sh
```

Script [build.sh](build.sh) tự cài công cụ `wails` nếu chưa có, rồi đóng gói thành `Bash Runner.app` trong thư mục dự án. Kéo file này vào thư mục Applications nếu muốn.

Chạy chế độ phát triển (tự tải lại khi sửa `frontend/`):

```bash
wails dev
```

Icon của app là [icon.png](icon.png).

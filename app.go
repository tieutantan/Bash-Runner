package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	storeDir  = "Bash Runner"
	storeFile = "scripts.json"
	// Phiên bản Fyne lưu danh sách trong preferences của nó. Đọc một lần để không mất dữ liệu cũ.
	legacyPrefs = "Library/Preferences/fyne/com.tantn.bashrunner/preferences.json"
)

// script là một file bash đã thêm vào danh sách. Name rỗng nghĩa là dùng tên file gốc.
// Runs là các lần chạy với tham số đã lưu, hiện thụt lề ngay dưới file.
type script struct {
	Path string      `json:"path"`
	Name string      `json:"name"`
	Runs []customRun `json:"runs,omitempty"`
}

// customRun là một lần chạy file kèm tham số, có tên riêng để nhận ra.
type customRun struct {
	Name   string `json:"name"`
	Params string `json:"params"`
}

// scriptView là script kèm các giá trị tính sẵn để giao diện chỉ việc hiển thị.
type scriptView struct {
	Path        string      `json:"path"`
	DisplayName string      `json:"displayName"`
	Dir         string      `json:"dir"`
	Exists      bool        `json:"exists"`
	Runs        []customRun `json:"runs"`
}

// outputEvent là một đoạn output gửi sang giao diện. Kind: cmd, out, err, ok, fail.
type outputEvent struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// runEvent báo một lần chạy bắt đầu hoặc kết thúc.
type runEvent struct {
	Key     string `json:"key"`
	Running bool   `json:"running"`
	OK      bool   `json:"ok"`
}

type App struct {
	ctx context.Context

	mu      sync.Mutex
	scripts []script
	running map[string]bool // khóa theo runKey, để không chạy trùng cùng một lần chạy
}

func newApp() *App {
	a := &App{running: map[string]bool{}}
	a.load()
	return a
}

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

func storePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, storeDir, storeFile), nil
}

func (a *App) load() {
	path, err := storePath()
	if err != nil {
		return
	}
	if data, err := os.ReadFile(path); err == nil {
		// Dữ liệu lỗi thì bỏ qua, bắt đầu với danh sách rỗng.
		_ = json.Unmarshal(data, &a.scripts)
		return
	}
	a.migrateLegacy()
}

// migrateLegacy chuyển danh sách từ bản Fyne cũ sang file mới.
func (a *App) migrateLegacy() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	data, err := os.ReadFile(filepath.Join(home, legacyPrefs))
	if err != nil {
		return
	}
	var prefs map[string]any
	if json.Unmarshal(data, &prefs) != nil {
		return
	}
	if raw, ok := prefs["scripts"].(string); ok && raw != "" {
		_ = json.Unmarshal([]byte(raw), &a.scripts)
	} else if raw, ok := prefs["scriptPaths"].(string); ok {
		var paths []string
		if json.Unmarshal([]byte(raw), &paths) == nil {
			for _, p := range paths {
				a.scripts = append(a.scripts, script{Path: p})
			}
		}
	}
	a.save()
}

// save ghi danh sách ra đĩa. Gọi khi đang giữ a.mu hoặc khi chưa có goroutine nào khác.
func (a *App) save() {
	path, err := storePath()
	if err != nil {
		return
	}
	data, err := json.MarshalIndent(a.scripts, "", "  ")
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// views dựng danh sách để gửi cho giao diện. Gọi khi đang giữ a.mu.
func (a *App) views() []scriptView {
	out := make([]scriptView, 0, len(a.scripts))
	for _, s := range a.scripts {
		name := s.Name
		if name == "" {
			name = filepath.Base(s.Path)
		}
		runs := s.Runs
		if runs == nil {
			runs = []customRun{}
		}
		out = append(out, scriptView{
			Path:        s.Path,
			DisplayName: name,
			Dir:         filepath.Dir(s.Path),
			Exists:      fileExists(s.Path),
			Runs:        runs,
		})
	}
	return out
}

// GetScripts trả về danh sách file hiện có.
func (a *App) GetScripts() []scriptView {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.views()
}

// AddScript mở hộp thoại chọn file .sh của hệ thống rồi thêm vào danh sách.
func (a *App) AddScript() ([]scriptView, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Chọn file bash",
		Filters: []runtime.FileFilter{
			{DisplayName: "File bash (*.sh)", Pattern: "*.sh"},
		},
	})
	a.mu.Lock()
	defer a.mu.Unlock()
	if err != nil || path == "" {
		return a.views(), err
	}
	for _, s := range a.scripts {
		if s.Path == path {
			return a.views(), nil
		}
	}
	a.scripts = append(a.scripts, script{Path: path})
	a.save()
	return a.views(), nil
}

// RemoveScript bỏ file khỏi danh sách. File gốc trên đĩa không bị xóa.
func (a *App) RemoveScript(path string) []scriptView {
	a.mu.Lock()
	defer a.mu.Unlock()
	kept := a.scripts[:0]
	for _, s := range a.scripts {
		if s.Path != path {
			kept = append(kept, s)
		}
	}
	a.scripts = kept
	a.save()
	return a.views()
}

// RenameScript đặt tên hiển thị khác cho file. Tên rỗng nghĩa là dùng lại tên file gốc.
func (a *App) RenameScript(path, name string) []scriptView {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.scripts {
		if a.scripts[i].Path == path {
			a.scripts[i].Name = strings.TrimSpace(name)
		}
	}
	a.save()
	return a.views()
}

// SaveRun lưu một lần chạy có tham số. idx -1 là tạo mới, còn không thì sửa lần chạy đã lưu.
func (a *App) SaveRun(path string, idx int, name, params string) ([]scriptView, error) {
	name, params = strings.TrimSpace(name), strings.TrimSpace(params)
	if params == "" {
		return nil, errors.New("Nhập tham số trước khi lưu")
	}
	// Không đặt tên thì dùng luôn tham số làm tên.
	if name == "" {
		name = params
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.scripts {
		if a.scripts[i].Path != path {
			continue
		}
		run := customRun{Name: name, Params: params}
		if idx >= 0 && idx < len(a.scripts[i].Runs) {
			a.scripts[i].Runs[idx] = run
		} else {
			a.scripts[i].Runs = append(a.scripts[i].Runs, run)
		}
	}
	a.save()
	return a.views(), nil
}

// DeleteRun xóa một lần chạy đã lưu. File gốc vẫn được giữ lại.
func (a *App) DeleteRun(path string, idx int) []scriptView {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.scripts {
		if a.scripts[i].Path == path && idx >= 0 && idx < len(a.scripts[i].Runs) {
			runs := a.scripts[i].Runs
			a.scripts[i].Runs = append(runs[:idx], runs[idx+1:]...)
		}
	}
	a.save()
	return a.views()
}

// OpenInEditor mở file bằng ứng dụng mặc định của hệ thống cho file .sh.
func (a *App) OpenInEditor(path string) error {
	return exec.Command("open", path).Run()
}

// CopyText đưa văn bản vào clipboard.
func (a *App) CopyText(text string) error {
	return runtime.ClipboardSetText(a.ctx, text)
}

// runKey định danh một lần chạy. Cùng một file nhưng khác tham số thì là lần chạy khác.
func runKey(path, params string) string {
	return path + "\x00" + params
}

// RunKey để giao diện tính cùng khóa với backend.
func (a *App) RunKey(path, params string) string { return runKey(path, params) }

// GetRunning trả về các khóa lần chạy đang chạy, dùng khi giao diện vừa mở.
func (a *App) GetRunning() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	keys := make([]string, 0, len(a.running))
	for k := range a.running {
		keys = append(keys, k)
	}
	return keys
}

// bashCommand tạo lệnh chạy file với tham số. Tham số được đưa vào shell nguyên văn,
// nên dấu nháy và khoảng trắng hoạt động như khi gõ trong Terminal.
func bashCommand(path, params string) *exec.Cmd {
	if strings.TrimSpace(params) == "" {
		return exec.Command("/bin/bash", path)
	}
	// $0 nhận đường dẫn file, nên đường dẫn có khoảng trắng không cần escape.
	return exec.Command("/bin/bash", "-c", `exec /bin/bash "$0" `+params, path)
}

func (a *App) emitOutput(kind, text string) {
	runtime.EventsEmit(a.ctx, "output", outputEvent{Kind: kind, Text: text})
}

func (a *App) emitRun(key string, running, ok bool) {
	runtime.EventsEmit(a.ctx, "run", runEvent{Key: key, Running: running, OK: ok})
}

// Run chạy file với tham số (có thể rỗng) và phát output về giao diện. Mỗi lần chạy có khóa riêng để không chạy trùng.
func (a *App) Run(path, params string) {
	key := runKey(path, params)
	a.mu.Lock()
	if a.running[key] {
		a.mu.Unlock()
		return
	}
	a.running[key] = true
	a.mu.Unlock()

	a.emitRun(key, true, false)
	runtime.EventsEmit(a.ctx, "clear")
	a.emitOutput("cmd", "$ bash "+strings.TrimSpace(path+" "+params)+"\n")

	go func() {
		ok := a.execute(path, params)
		a.mu.Lock()
		delete(a.running, key)
		a.mu.Unlock()
		a.emitRun(key, false, ok)
	}()
}

// execute chạy lệnh và chờ xong. Trả về true khi exit 0.
func (a *App) execute(path, params string) bool {
	// Chạy trong thư mục chứa file để các đường dẫn tương đối trong script hoạt động đúng.
	cmd := bashCommand(path, params)
	cmd.Dir = filepath.Dir(path)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		a.emitOutput("fail", "Lỗi: "+err.Error()+"\n")
		return false
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		a.emitOutput("fail", "Lỗi: "+err.Error()+"\n")
		return false
	}
	if err := cmd.Start(); err != nil {
		a.emitOutput("fail", "Lỗi: "+err.Error()+"\n")
		return false
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go a.pipe(stdout, "out", &wg)
	go a.pipe(stderr, "err", &wg)
	wg.Wait()

	if err := cmd.Wait(); err != nil {
		a.emitOutput("fail", fmt.Sprintf("\n[Lỗi: %v]\n", err))
		return false
	}
	a.emitOutput("ok", "\n[Hoàn tất, exit 0]\n")
	return true
}

// pipe đọc output theo từng đoạn để hiện ngay cả khi script chưa xuống dòng (thanh tiến trình, prompt).
// Đoạn gửi đi không bao giờ cắt giữa một ký tự UTF-8.
func (a *App) pipe(rd io.Reader, kind string, wg *sync.WaitGroup) {
	defer wg.Done()
	buf := make([]byte, 4096)
	var pending []byte
	for {
		n, err := rd.Read(buf)
		pending = append(pending, buf[:n]...)
		cut := len(pending)
		if err == nil {
			cut = completeUTF8(pending)
		}
		if cut > 0 {
			a.emitOutput(kind, string(pending[:cut]))
			pending = pending[cut:]
		}
		if err != nil {
			return
		}
	}
}

// completeUTF8 trả về độ dài phần đầu của b không kết thúc giữa một ký tự UTF-8.
func completeUTF8(b []byte) int {
	for i := 1; i <= utf8.UTFMax && i <= len(b); i++ {
		if utf8.RuneStart(b[len(b)-i]) {
			if utf8.FullRune(b[len(b)-i:]) {
				return len(b)
			}
			return len(b) - i
		}
	}
	return len(b)
}

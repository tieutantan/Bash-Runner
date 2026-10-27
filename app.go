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
	// The earlier Fyne version stored the list in its preferences. Read it once so no existing data is lost.
	legacyPrefs = "Library/Preferences/fyne/com.tantn.bashrunner/preferences.json"
)

// script is a bash file added to the list. An empty Name means the original file name is used.
// Runs are the saved runs with parameters, shown indented right below the file.
type script struct {
	Path string      `json:"path"`
	Name string      `json:"name"`
	Runs []customRun `json:"runs,omitempty"`
}

// customRun is a run of the file with parameters, with its own name so it can be recognized.
type customRun struct {
	Name   string `json:"name"`
	Params string `json:"params"`
}

// scriptView is a script with precomputed values so the UI only has to display them.
type scriptView struct {
	Path        string      `json:"path"`
	DisplayName string      `json:"displayName"`
	Dir         string      `json:"dir"`
	Exists      bool        `json:"exists"`
	Runs        []customRun `json:"runs"`
}

// outputEvent is a piece of output sent to the UI. Kind: cmd, out, err, ok, fail.
type outputEvent struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// runEvent reports that a run has started or finished.
type runEvent struct {
	Key     string `json:"key"`
	Running bool   `json:"running"`
	OK      bool   `json:"ok"`
}

type App struct {
	ctx context.Context

	mu      sync.Mutex
	scripts []script
	running map[string]bool // keyed by runKey, so the same run is never started twice
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
		// Corrupt data is ignored; start with an empty list.
		_ = json.Unmarshal(data, &a.scripts)
		return
	}
	a.migrateLegacy()
}

// migrateLegacy moves the list from the old Fyne version to the new file.
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

// save writes the list to disk. Call it while holding a.mu, or when no other goroutine is running.
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

// views builds the list to send to the UI. Call it while holding a.mu.
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

// GetScripts returns the current list of files.
func (a *App) GetScripts() []scriptView {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.views()
}

// AddScript opens a dialog to pick one or more .sh files and adds them to the list. Files already in the list are skipped.
func (a *App) AddScript() ([]scriptView, error) {
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Choose bash files",
		Filters: []runtime.FileFilter{
			{DisplayName: "Bash files (*.sh)", Pattern: "*.sh"},
		},
	})
	a.mu.Lock()
	defer a.mu.Unlock()
	if err != nil || len(paths) == 0 {
		return a.views(), err
	}
	added := false
	for _, path := range paths {
		if a.hasScript(path) {
			continue
		}
		a.scripts = append(a.scripts, script{Path: path})
		added = true
	}
	if added {
		a.save()
	}
	return a.views(), nil
}

// hasScript reports whether the file is already in the list. Call it while holding a.mu.
func (a *App) hasScript(path string) bool {
	for _, s := range a.scripts {
		if s.Path == path {
			return true
		}
	}
	return false
}

// RemoveScript removes the file from the list. The original file on disk is not deleted.
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

// RenameScript sets a different display name for the file. An empty name means going back to the original file name.
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

// SaveRun saves a run with parameters. idx -1 creates a new run; otherwise the saved run at idx is edited.
func (a *App) SaveRun(path string, idx int, name, params string) ([]scriptView, error) {
	name, params = strings.TrimSpace(name), strings.TrimSpace(params)
	if params == "" {
		return nil, errors.New("Enter parameters before saving")
	}
	// If no name is given, the parameters are used as the name.
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

// DeleteRun deletes a saved run. The original file is kept.
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

// OpenInEditor opens the file with the system's default app for .sh files.
func (a *App) OpenInEditor(path string) error {
	return exec.Command("open", path).Run()
}

// CopyText puts the text on the clipboard.
func (a *App) CopyText(text string) error {
	return runtime.ClipboardSetText(a.ctx, text)
}

// runKey identifies a run. The same file with different parameters is a different run.
func runKey(path, params string) string {
	return path + "\x00" + params
}

// RunKey lets the UI compute the same key as the backend.
func (a *App) RunKey(path, params string) string { return runKey(path, params) }

// GetRunning returns the keys of the runs in progress, used when the UI opens.
func (a *App) GetRunning() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	keys := make([]string, 0, len(a.running))
	for k := range a.running {
		keys = append(keys, k)
	}
	return keys
}

// bashCommand builds the command that runs the file with parameters. Parameters are passed to the shell verbatim,
// so quotes and spaces behave as they do when typed in Terminal.
func bashCommand(path, params string) *exec.Cmd {
	if strings.TrimSpace(params) == "" {
		return exec.Command("/bin/bash", path)
	}
	// $0 receives the file path, so paths with spaces need no escaping.
	return exec.Command("/bin/bash", "-c", `exec /bin/bash "$0" `+params, path)
}

func (a *App) emitOutput(kind, text string) {
	runtime.EventsEmit(a.ctx, "output", outputEvent{Kind: kind, Text: text})
}

func (a *App) emitRun(key string, running, ok bool) {
	runtime.EventsEmit(a.ctx, "run", runEvent{Key: key, Running: running, OK: ok})
}

// Run runs the file with parameters (may be empty) and streams the output to the UI. Each run has its own key so it is never started twice.
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

// execute runs the command and waits for it to finish. Returns true when it exits with 0.
func (a *App) execute(path, params string) bool {
	// Run in the file's directory so relative paths inside the script work correctly.
	cmd := bashCommand(path, params)
	cmd.Dir = filepath.Dir(path)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		a.emitOutput("fail", "Error: "+err.Error()+"\n")
		return false
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		a.emitOutput("fail", "Error: "+err.Error()+"\n")
		return false
	}
	if err := cmd.Start(); err != nil {
		a.emitOutput("fail", "Error: "+err.Error()+"\n")
		return false
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go a.pipe(stdout, "out", &wg)
	go a.pipe(stderr, "err", &wg)
	wg.Wait()

	if err := cmd.Wait(); err != nil {
		a.emitOutput("fail", fmt.Sprintf("\n[Error: %v]\n", err))
		return false
	}
	a.emitOutput("ok", "\n[Done, exit 0]\n")
	return true
}

// pipe reads the output in chunks so it appears right away even when the script has not printed a newline (progress bars, prompts).
// A chunk is never cut in the middle of a UTF-8 character.
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

// completeUTF8 returns the length of the prefix of b that does not end in the middle of a UTF-8 character.
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

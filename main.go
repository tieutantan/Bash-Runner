package main

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	prefScripts     = "scripts"
	prefLegacyPaths = "scriptPaths"

	addButtonHeight = 40.5 // chiều cao nút Thêm file (1.5 lần 27)
	clearSize       = 27   // đường kính nút Xóa output
	runSize         = 36   // đường kính nút Run tròn
	menuSize        = 36
	subRunSize      = 32 // nút Run của lần chạy có tham số, nhỏ hơn nút Run của file gốc
	indentWidth     = 24 // độ thụt lề của lần chạy có tham số so với file gốc
	cardRadius      = 18
	buttonRadius    = 10
	headingSize     = 20 // thang cỡ chữ: 12 / 14 / 16 / 20
	subHeadingSize  = 16
)

// Bảng màu (design tokens). Mọi màu trong app lấy từ đây, không hardcode trong component.
var (
	colBackground = color.NRGBA{R: 0x0f, G: 0x12, B: 0x19, A: 0xff} // nền cửa sổ
	colSurface    = color.NRGBA{R: 0x17, G: 0x1b, B: 0x26, A: 0xff} // thẻ file, ô nhập
	colRaised     = color.NRGBA{R: 0x1f, G: 0x24, B: 0x33, A: 0xff} // trạng thái hover/pressed
	colBorder     = color.NRGBA{R: 0x2a, G: 0x31, B: 0x43, A: 0xff} // viền và đường kẻ
	colText       = color.NRGBA{R: 0xe7, G: 0xe9, B: 0xf0, A: 0xff} // chữ chính
	colMuted      = color.NRGBA{R: 0x8d, G: 0x94, B: 0xa8, A: 0xff} // chữ phụ, đạt AA trên colSurface
	colPrimary    = color.NRGBA{R: 0x3b, G: 0x82, B: 0xf6, A: 0xff} // hành động chính, focus
	colSuccess    = color.NRGBA{R: 0x22, G: 0xc5, B: 0x5e, A: 0xff} // đang chạy, thành công
	colRun        = color.NRGBA{R: 0x16, G: 0xa3, B: 0x4a, A: 0xff} // nền nút Run (icon trắng đạt 3:1)
	colWarning    = color.NRGBA{R: 0xfb, G: 0xbf, B: 0x24, A: 0xff}
	colDanger     = color.NRGBA{R: 0xf8, G: 0x71, B: 0x71, A: 0xff} // lỗi, xóa
	colIdle       = color.NRGBA{R: 0x5b, G: 0x63, B: 0x76, A: 0xff} // trạng thái nghỉ
)

//go:embed icon.png
var iconData []byte

// darkTheme ghi đè màu và kích thước của theme mặc định theo design tokens ở trên.
type darkTheme struct {
	fyne.Theme
}

func (t darkTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return colBackground
	case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return colSurface
	case theme.ColorNameInputBackground:
		return colSurface
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return colBorder
	case theme.ColorNameButton, theme.ColorNameHover:
		return colRaised
	case theme.ColorNamePressed:
		return colBorder
	case theme.ColorNameForeground:
		return colText
	case theme.ColorNamePlaceHolder:
		return colMuted
	case theme.ColorNameDisabled:
		return colMuted
	case theme.ColorNamePrimary, theme.ColorNameFocus:
		return colPrimary
	case theme.ColorNameSuccess:
		return colSuccess
	case theme.ColorNameWarning:
		return colWarning
	case theme.ColorNameError:
		return colDanger
	case theme.ColorNameForegroundOnPrimary, theme.ColorNameForegroundOnSuccess, theme.ColorNameForegroundOnError:
		return color.White
	case theme.ColorNameScrollBar:
		return colBorder
	case theme.ColorNameShadow:
		return color.NRGBA{A: 0x66}
	}
	return t.Theme.Color(name, variant)
}

func (t darkTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding, theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameText:
		return 14
	case theme.SizeNameCaptionText:
		return 12
	case theme.SizeNameSubHeadingText:
		return subHeadingSize
	case theme.SizeNameHeadingText:
		return headingSize
	case theme.SizeNameInputRadius, theme.SizeNameButtonRadius:
		return buttonRadius
	case theme.SizeNameCardRadius:
		return cardRadius
	case theme.SizeNameSeparatorThickness:
		return 1
	}
	return t.Theme.Size(name)
}

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

func (s script) displayName() string {
	if s.Name != "" {
		return s.Name
	}
	return filepath.Base(s.Path)
}

type runner struct {
	app     fyne.App
	win     fyne.Window
	scripts []script
	running map[string]bool // các lần chạy đang chạy, khóa theo runKey để khóa nút Run
	list    *fyne.Container
	count   *widget.Label
	output  *widget.Entry
}

func main() {
	a := app.NewWithID("com.tantn.bashrunner")
	a.Settings().SetTheme(darkTheme{Theme: theme.DefaultTheme()})
	a.SetIcon(fyne.NewStaticResource("icon.png", iconData))
	w := a.NewWindow("Bash Runner")

	r := &runner{
		app:     a,
		win:     w,
		running: map[string]bool{},
		list:    container.NewVBox(),
		count:   widget.NewLabel(""),
		output:  widget.NewMultiLineEntry(),
	}
	r.count.Importance = widget.LowImportance
	r.output.Wrapping = fyne.TextWrapWord
	r.output.TextStyle = fyne.TextStyle{Monospace: true}
	r.loadScripts()

	title := canvas.NewText("Bash Runner", colText)
	title.TextSize = headingSize
	title.TextStyle = fyne.TextStyle{Bold: true}
	// Số file nằm ngang hàng với tên app, căn giữa theo chiều dọc.
	titleBox := container.NewBorder(nil, nil, title, nil, container.NewCenter(container.NewPadded(r.count)))

	// Border kéo đối tượng bên phải cao bằng cả hàng tiêu đề, nên bọc trong NewCenter để nút giữ đúng kích thước.
	addBtn := newRoundButton(theme.ContentAddIcon(), colPrimary, r.addScript)
	addBox := container.NewCenter(container.NewGridWrap(fyne.NewSize(addButtonHeight, addButtonHeight), addBtn))

	header := container.NewBorder(nil, nil, titleBox, addBox)

	outputTitle := canvas.NewText("Output", colText)
	outputTitle.TextSize = subHeadingSize
	outputTitle.TextStyle = fyne.TextStyle{Bold: true}
	clearBtn := newRoundButton(theme.ContentClearIcon(), colDanger, func() {
		r.output.SetText("")
	})
	outputHeader := container.NewBorder(nil, nil,
		outputTitle,
		container.NewGridWrap(fyne.NewSize(clearSize, clearSize), clearBtn),
	)

	listScroll := container.NewVScroll(container.NewPadded(r.list))
	outputArea := container.NewBorder(container.NewPadded(outputHeader), nil, nil, nil, // Ô output sát tiêu đề hơn: bỏ phần đệm phía trên, giữ đệm các cạnh còn lại.
		container.New(layout.NewCustomPaddedLayout(0, 8, 8, 8), r.output))
	split := container.NewVSplit(listScroll, outputArea)
	split.Offset = 0.775 // khung output mặc định thấp bằng nửa so với trước (0.45 → 0.225)

	w.SetContent(container.NewBorder(container.NewPadded(header), nil, nil, nil, split))
	w.Resize(fyne.NewSize(880, 680))
	r.refresh()
	w.ShowAndRun()
}

func (r *runner) loadScripts() {
	if raw := r.app.Preferences().String(prefScripts); raw != "" {
		// Dữ liệu lỗi trong preferences thì bỏ qua, bắt đầu với danh sách rỗng.
		_ = json.Unmarshal([]byte(raw), &r.scripts)
		return
	}

	// Chuyển dữ liệu từ phiên bản cũ (chỉ lưu đường dẫn) sang định dạng mới.
	raw := r.app.Preferences().String(prefLegacyPaths)
	if raw == "" {
		return
	}
	var paths []string
	if err := json.Unmarshal([]byte(raw), &paths); err != nil {
		return
	}
	for _, p := range paths {
		r.scripts = append(r.scripts, script{Path: p})
	}
}

func (r *runner) saveScripts() {
	data, err := json.Marshal(r.scripts)
	if err != nil {
		return
	}
	r.app.Preferences().SetString(prefScripts, string(data))
}

func (r *runner) addScript() {
	d := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, r.win)
			return
		}
		if rc == nil {
			return
		}
		defer rc.Close()

		path := rc.URI().Path()
		if !r.hasPath(path) {
			r.scripts = append(r.scripts, script{Path: path})
			r.saveScripts()
		}
		r.refresh()
	}, r.win)
	d.SetFilter(storage.NewExtensionFileFilter([]string{".sh"}))
	d.Resize(fyne.NewSize(700, 500))
	d.Show()
}

func (r *runner) hasPath(path string) bool {
	for _, s := range r.scripts {
		if s.Path == path {
			return true
		}
	}
	return false
}

func (r *runner) rename(s script) {
	entry := widget.NewEntry()
	entry.SetText(s.displayName())
	entry.SetPlaceHolder(filepath.Base(s.Path))

	d := dialog.NewCustomConfirm("Đổi tên hiển thị", "Lưu", "Hủy", entry, func(ok bool) {
		if !ok {
			return
		}
		name := strings.TrimSpace(entry.Text)
		for i := range r.scripts {
			if r.scripts[i].Path == s.Path {
				r.scripts[i].Name = name
			}
		}
		r.saveScripts()
		r.refresh()
	}, r.win)
	d.Resize(fyne.NewSize(420, 160))
	d.Show()
}

func (r *runner) removeScript(path string) {
	kept := r.scripts[:0]
	for _, s := range r.scripts {
		if s.Path != path {
			kept = append(kept, s)
		}
	}
	r.scripts = kept
	r.saveScripts()
	r.refresh()
}

// menuWidth là chiều rộng cố định của popup menu.
const menuWidth = 248

// menuRow là một dòng bấm được trong menu. Nền đổi màu khi rê chuột.
type menuRow struct {
	widget.BaseWidget
	bg      *canvas.Rectangle
	content fyne.CanvasObject
	onTap   func()
}

func newMenuRow(label string, res fyne.Resource, textColor color.Color, onTap func()) *menuRow {
	row := &menuRow{
		bg:    canvas.NewRectangle(color.Transparent),
		onTap: onTap,
	}
	row.bg.CornerRadius = buttonRadius

	text := canvas.NewText(label, textColor)
	text.TextSize = theme.TextSize()
	content := container.NewHBox(widget.NewIcon(res), text)
	row.ExtendBaseWidget(row)
	row.content = container.NewStack(row.bg, container.NewPadded(content))
	return row
}

func (m *menuRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(m.content)
}

func (m *menuRow) Tapped(*fyne.PointEvent) {
	if m.onTap != nil {
		m.onTap()
	}
}

func (m *menuRow) MouseIn(*desktop.MouseEvent)    { m.setHover(colRaised) }
func (m *menuRow) MouseMoved(*desktop.MouseEvent) {}
func (m *menuRow) MouseOut()                      { m.setHover(color.Transparent) }

func (m *menuRow) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (m *menuRow) setHover(c color.Color) {
	m.bg.FillColor = c
	m.bg.Refresh()
}

// popupHeader hiện icon, tên, dòng phụ và trạng thái file ở đầu popup menu.
func popupHeader(res fyne.Resource, title, subtitle string, exists bool) fyne.CanvasObject {
	tileBg := canvas.NewRectangle(colRaised)
	tileBg.CornerRadius = buttonRadius
	tile := container.NewGridWrap(fyne.NewSize(40, 40),
		container.NewStack(tileBg, container.NewPadded(widget.NewIcon(res))),
	)

	name := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	name.Truncation = fyne.TextTruncateEllipsis
	folder := widget.NewLabel(subtitle)
	folder.Truncation = fyne.TextTruncateEllipsis
	folder.Importance = widget.LowImportance

	statusText, statusColor := "Đã tìm thấy", colSuccess
	if !exists {
		statusText, statusColor = "Không tìm thấy file", colDanger
	}
	dot := container.NewGridWrap(fyne.NewSize(8, 8), canvas.NewCircle(statusColor))
	status := canvas.NewText(statusText, statusColor)
	status.TextSize = theme.CaptionTextSize()

	info := container.NewVBox(name, folder, container.NewHBox(container.NewCenter(dot), status))
	return container.NewBorder(nil, nil, tile, nil, info)
}

// menuItem là một dòng trong popup menu. divider vẽ đường kẻ phía trên dòng đó.
type menuItem struct {
	label   string
	res     fyne.Resource
	color   color.Color
	onTap   func()
	divider bool
}

// showPopup mở popup menu gồm phần đầu (header) và các dòng bấm được, neo dưới nút btn.
func (r *runner) showPopup(btn fyne.CanvasObject, header fyne.CanvasObject, items []menuItem) {
	var pop *widget.PopUp

	// Spacer đặt chiều rộng tối thiểu để menu không bị co lại theo nội dung ngắn.
	width := canvas.NewRectangle(color.Transparent)
	width.SetMinSize(fyne.NewSize(menuWidth, 0))

	card := canvas.NewRectangle(colSurface)
	card.StrokeColor = colBorder
	card.StrokeWidth = 1
	card.CornerRadius = cardRadius

	body := []fyne.CanvasObject{width, header, widget.NewSeparator()}
	for _, it := range items {
		if it.divider {
			body = append(body, widget.NewSeparator())
		}
		// act đóng menu trước rồi mới chạy hành động, để popup không còn nằm trên màn hình khi có hộp thoại.
		fn := it.onTap
		onTap := func() {
			pop.Hide()
			fn()
		}
		body = append(body, newMenuRow(it.label, it.res, it.color, onTap))
	}
	// Dùng PopUp thay vì PopUpMenu để bọc menu trong thẻ bo góc. Đổi lại PopUp không tự căn vị trí,
	// nên phải kéo menu vào trong cửa sổ.
	pop = widget.NewPopUp(container.NewStack(card, container.NewPadded(container.NewVBox(body...))), r.win.Canvas())

	pos := r.app.Driver().AbsolutePositionForObject(btn)
	pos.Y += btn.Size().Height
	size := pop.MinSize()
	canvasSize := r.win.Canvas().Size()
	pos.X = fyne.Max(0, fyne.Min(pos.X, canvasSize.Width-size.Width))
	pos.Y = fyne.Max(0, fyne.Min(pos.Y, canvasSize.Height-size.Height))
	pop.ShowAtPosition(pos)
}

// showMenu mở menu ⋮ của một file.
func (r *runner) showMenu(btn fyne.CanvasObject, s script) {
	header := popupHeader(theme.FileIcon(), s.displayName(), filepath.Dir(s.Path), fileExists(s.Path))
	r.showPopup(btn, header, []menuItem{
		{label: "Chạy với tham số…", res: theme.MediaPlayIcon(), color: colText, onTap: func() { r.paramDialog(s, -1) }},
		{label: "Đổi tên hiển thị", res: theme.DocumentCreateIcon(), color: colText, onTap: func() { r.rename(s) }},
		{label: "Sao chép đường dẫn", res: theme.ContentCopyIcon(), color: colText, onTap: func() {
			r.app.Clipboard().SetContent(s.Path)
		}},
		{label: "Mở để sửa", res: theme.DocumentIcon(), color: colText, onTap: func() {
			// Mở bằng ứng dụng mặc định của hệ thống cho file .sh.
			if err := r.app.OpenURL(&url.URL{Scheme: "file", Path: s.Path}); err != nil {
				dialog.ShowError(err, r.win)
			}
		}},
		{label: "Xóa khỏi danh sách", res: theme.DeleteIcon(), color: colDanger, divider: true, onTap: func() { r.removeScript(s.Path) }},
	})
}

// showRunMenu mở menu ⋮ của một lần chạy có tham số.
func (r *runner) showRunMenu(btn fyne.CanvasObject, s script, idx int) {
	run := s.Runs[idx]
	header := popupHeader(theme.MediaPlayIcon(), run.Name, run.Params, fileExists(s.Path))
	r.showPopup(btn, header, []menuItem{
		{label: "Sửa tên và tham số", res: theme.DocumentCreateIcon(), color: colText, onTap: func() { r.paramDialog(s, idx) }},
		{label: "Sao chép tham số", res: theme.ContentCopyIcon(), color: colText, onTap: func() {
			r.app.Clipboard().SetContent(run.Params)
		}},
		{label: "Xóa khỏi danh sách", res: theme.DeleteIcon(), color: colDanger, divider: true, onTap: func() {
			r.editRuns(s.Path, func(runs []customRun) []customRun {
				return append(runs[:idx], runs[idx+1:]...)
			})
		}},
	})
}

// paramDialog mở hộp thoại nhập tham số. idx -1 là tạo lần chạy mới, còn không thì sửa lần chạy đã lưu.
func (r *runner) paramDialog(s script, idx int) {
	name := widget.NewEntry()
	name.SetPlaceHolder("Ví dụ: Triển khai staging")
	params := widget.NewEntry()
	params.SetPlaceHolder(`Ví dụ: --env staging "thư mục mới"`)
	title := "Chạy với tham số"
	if idx >= 0 {
		title = "Sửa lần chạy"
		name.SetText(s.Runs[idx].Name)
		params.SetText(s.Runs[idx].Params)
	}

	hint := widget.NewLabel("Tham số được truyền nguyên văn như khi gõ trong Terminal. Lưu lại để lần chạy hiện ngay dưới file.")
	hint.Wrapping = fyne.TextWrapWord
	hint.Importance = widget.LowImportance

	var d dialog.Dialog
	runOnce := newTextButton("Chạy một lần", theme.MediaPlayIcon(), colRaised, func() {
		d.Hide()
		r.run(s.Path, params.Text)
	})
	save := newTextButton("Lưu", theme.DocumentSaveIcon(), colPrimary, func() {
		if strings.TrimSpace(params.Text) == "" {
			dialog.ShowError(fmt.Errorf("Nhập tham số trước khi lưu"), r.win)
			return
		}
		d.Hide()
		run := customRun{Name: strings.TrimSpace(name.Text), Params: strings.TrimSpace(params.Text)}
		// Không đặt tên thì dùng luôn tham số làm tên.
		if run.Name == "" {
			run.Name = run.Params
		}
		r.editRuns(s.Path, func(runs []customRun) []customRun {
			if idx >= 0 {
				runs[idx] = run
				return runs
			}
			return append(runs, run)
		})
	})
	// Dùng textButton thay vì widget.Button vì Button của Fyne luôn hiện con trỏ mũi tên, không đổi thành bàn tay.
	cancel := newTextButton("Hủy", theme.CancelIcon(), colRaised, func() { d.Hide() })

	form := container.NewVBox(
		widget.NewLabel("Tên"),
		name,
		widget.NewLabel("Tham số"),
		params,
		hint,
		container.NewGridWithColumns(3, cancel, runOnce, save),
	)
	// Không dùng nút mặc định của dialog để ba nút nằm chung một dòng.
	d = dialog.NewCustomWithoutButtons(title, form, r.win)
	d.Resize(fyne.NewSize(420, 320))
	d.Show()
}

// editRuns áp dụng thay đổi lên danh sách lần chạy của file, rồi lưu và vẽ lại.
func (r *runner) editRuns(path string, edit func([]customRun) []customRun) {
	for i := range r.scripts {
		if r.scripts[i].Path == path {
			r.scripts[i].Runs = edit(r.scripts[i].Runs)
		}
	}
	r.saveScripts()
	r.refresh()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// refresh vẽ lại danh sách: chưa có file thì hiện trạng thái trống, không thì vẽ từng file.
func (r *runner) refresh() {
	r.list.Objects = nil
	r.count.SetText(fmt.Sprintf("%d file bash", len(r.scripts)))

	if len(r.scripts) == 0 {
		r.list.Add(r.emptyState())
	} else {
		for _, s := range r.scripts {
			r.list.Add(r.card(s))
		}
	}
	r.list.Refresh()
}

// emptyState hiện khi chưa có file nào, kèm nút hành động để thêm file đầu tiên.
func (r *runner) emptyState() fyne.CanvasObject {
	icon := container.NewGridWrap(fyne.NewSize(48, 48), widget.NewIcon(theme.FileIcon()))
	title := widget.NewLabelWithStyle("Chưa có file nào", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	hint := widget.NewLabelWithStyle(
		"Bấm \"Thêm file\" để chọn file .sh ở bất kỳ đâu.",
		fyne.TextAlignCenter,
		fyne.TextStyle{},
	)
	hint.Importance = widget.LowImportance
	btn := newTextButton("Thêm file", theme.ContentAddIcon(), colPrimary, r.addScript)

	return container.NewCenter(container.NewVBox(
		container.NewCenter(icon),
		title,
		hint,
		container.NewCenter(btn),
	))
}

// card vẽ một dòng file: chấm trạng thái, tên hiển thị và đường dẫn, nút Run tròn và nút menu.
func (r *runner) card(s script) fyne.CanvasObject {
	bg := canvas.NewRectangle(colSurface)
	bg.StrokeColor = colBorder
	bg.StrokeWidth = 1
	bg.CornerRadius = cardRadius

	exists := fileExists(s.Path)

	name := widget.NewLabelWithStyle(s.displayName(), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	name.Truncation = fyne.TextTruncateEllipsis

	menuBtn := newRoundButton(theme.MoreVerticalIcon(), color.Transparent, nil)
	menuBtn.hoverBg = colRaised
	menuBtn.onTap = func() { r.showMenu(menuBtn, s) }

	actions := container.NewHBox(
		r.runButton(s.Path, "", runSize),
		container.NewGridWrap(fyne.NewSize(menuSize, menuSize), menuBtn),
	)
	// Chỉ hiện đường dẫn khi file không còn tồn tại, để người dùng biết cần tìm ở đâu.
	text := container.NewVBox(name)
	if !exists {
		path := widget.NewLabel("Không tìm thấy: " + s.Path)
		path.Truncation = fyne.TextTruncateEllipsis
		path.Importance = widget.DangerImportance
		text.Add(path)
	}
	rows := []fyne.CanvasObject{container.NewBorder(nil, nil, nil, actions, text)}
	// Các lần chạy có tham số hiện ngay dưới file, thụt lề để thấy cùng thuộc một file.
	for i, run := range s.Runs {
		rows = append(rows, r.runRow(s, i, run))
	}

	return container.NewStack(bg, container.NewPadded(container.NewVBox(rows...)))
}

// runRow vẽ một lần chạy có tham số: tên, tham số, nút Run và nút menu, thụt lề so với file gốc.
func (r *runner) runRow(s script, idx int, run customRun) fyne.CanvasObject {
	name := widget.NewLabel(run.Name)
	name.Truncation = fyne.TextTruncateEllipsis

	menuBtn := newRoundButton(theme.MoreVerticalIcon(), color.Transparent, nil)
	menuBtn.hoverBg = colRaised
	menuBtn.onTap = func() { r.showRunMenu(menuBtn, s, idx) }

	actions := container.NewHBox(
		r.runButton(s.Path, run.Params, subRunSize),
		container.NewGridWrap(fyne.NewSize(subRunSize, subRunSize), menuBtn),
	)
	indent := canvas.NewRectangle(color.Transparent)
	indent.SetMinSize(fyne.NewSize(indentWidth, 0))

	content := container.NewBorder(nil, nil, nil, actions, name)
	return container.NewBorder(nil, nil, indent, nil, content)
}

// runButton vẽ nút Run tròn cho một lần chạy. Nút bị khóa khi lần chạy đó đang chạy hoặc file không còn.
func (r *runner) runButton(path, params string, size float32) fyne.CanvasObject {
	runBg := colRun
	var onRun func()
	switch {
	case r.running[runKey(path, params)]:
		runBg = colIdle // đang chạy thì khóa nút
	case !fileExists(path):
		runBg = colBorder
	default:
		onRun = func() { r.run(path, params) }
	}
	return container.NewGridWrap(fyne.NewSize(size, size), newRoundButton(theme.MediaPlayIcon(), runBg, onRun))
}

// roundButton là nút icon hình tròn, dùng cho nút Run. onTap nil nghĩa là nút đang bị khóa.
// hoverBg nil nghĩa là không đổi màu khi rê chuột.
type roundButton struct {
	widget.BaseWidget
	bg      *canvas.Circle
	icon    *widget.Icon
	onTap   func()
	fill    color.Color
	hoverBg color.Color
}

func newRoundButton(res fyne.Resource, bg color.Color, onTap func()) *roundButton {
	b := &roundButton{
		bg:    canvas.NewCircle(bg),
		icon:  widget.NewIcon(res),
		onTap: onTap,
		fill:  bg,
	}
	b.ExtendBaseWidget(b)
	return b
}

func (b *roundButton) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewStack(b.bg, container.NewPadded(b.icon)))
}

func (b *roundButton) Tapped(*fyne.PointEvent) {
	if b.onTap != nil {
		b.onTap()
	}
}

// Cursor đổi thành bàn tay khi nút bấm được, và giữ mũi tên khi nút bị khóa.
func (b *roundButton) Cursor() desktop.Cursor {
	if b.onTap == nil {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

func (b *roundButton) MouseIn(*desktop.MouseEvent)    { b.setFill(b.hoverBg) }
func (b *roundButton) MouseMoved(*desktop.MouseEvent) {}
func (b *roundButton) MouseOut()                      { b.setFill(b.fill) }

func (b *roundButton) setFill(c color.Color) {
	if c == nil {
		return
	}
	b.bg.FillColor = c
	b.bg.Refresh()
}

// textButton là nút có chữ và icon, nền bo góc. Dùng cho nút "Thêm file" ở màn hình trống.
type textButton struct {
	widget.BaseWidget
	content fyne.CanvasObject
	onTap   func()
}

func newTextButton(text string, res fyne.Resource, bg color.Color, onTap func()) *textButton {
	label := canvas.NewText(text, color.White)
	label.TextSize = theme.TextSize()
	label.TextStyle = fyne.TextStyle{Bold: true}

	fill := canvas.NewRectangle(bg)
	fill.CornerRadius = buttonRadius
	b := &textButton{
		content: container.NewStack(fill, container.NewPadded(container.NewHBox(widget.NewIcon(res), label))),
		onTap:   onTap,
	}
	b.ExtendBaseWidget(b)
	return b
}

func (b *textButton) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(b.content)
}

func (b *textButton) Tapped(*fyne.PointEvent) {
	if b.onTap != nil {
		b.onTap()
	}
}

func (b *textButton) Cursor() desktop.Cursor { return desktop.PointerCursor }

// runKey định danh một lần chạy. Cùng một file nhưng khác tham số thì là lần chạy khác.
func runKey(path, params string) string {
	return path + "\x00" + params
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

// run chạy file với tham số (có thể rỗng) và hiện output. Mỗi lần chạy có khóa riêng để không chạy trùng.
func (r *runner) run(path, params string) {
	key := runKey(path, params)
	if r.running[key] {
		return
	}
	r.running[key] = true
	r.refresh()
	r.output.SetText("$ bash " + strings.TrimSpace(path+" "+params) + "\n")

	go func() {
		defer func() {
			fyne.Do(func() {
				delete(r.running, key)
				r.refresh()
			})
		}()

		// Chạy trong thư mục chứa file để các đường dẫn tương đối trong script hoạt động đúng.
		cmd := bashCommand(path, params)
		cmd.Dir = filepath.Dir(path)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			r.fail(err)
			return
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			r.fail(err)
			return
		}
		if err := cmd.Start(); err != nil {
			r.fail(err)
			return
		}

		var wg sync.WaitGroup
		wg.Add(2)
		go r.pipe(stdout, &wg)
		go r.pipe(stderr, &wg)
		wg.Wait()

		if err := cmd.Wait(); err != nil {
			r.appendOutput(fmt.Sprintf("\n[Lỗi: %v]\n", err))
		} else {
			r.appendOutput("\n[Hoàn tất, exit 0]\n")
		}
	}()
}

// fail ghi lỗi khi không khởi động được script.
func (r *runner) fail(err error) {
	r.appendOutput("Lỗi: " + err.Error() + "\n")
}

func (r *runner) pipe(rd io.Reader, wg *sync.WaitGroup) {
	defer wg.Done()
	sc := bufio.NewScanner(rd)
	for sc.Scan() {
		r.appendOutput(sc.Text() + "\n")
	}
}

func (r *runner) appendOutput(s string) {
	fyne.Do(func() {
		r.output.SetText(r.output.Text + s)
	})
}

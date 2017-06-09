package gonvim

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

// StatuslineItem is
type StatuslineItem interface {
	Redraw(bool) (int, int)
	// setPosition(x, y int)
}

// Statusline is
type Statusline struct {
	widget *widgets.QWidget
	bg     *RGBA

	borderTopWidth int
	paddingLeft    int
	paddingRight   int
	margin         int

	pos      *StatuslinePos
	mode     *StatusMode
	file     *StatuslineFile
	filetype *StatuslineFiletype
	git      *StatuslineGit
	encoding *StatuslineEncoding
	lint     *StatuslineLint
}

// StatuslineLint is
type StatuslineLint struct {
	errors     int
	warnings   int
	widget     *widgets.QWidget
	okIcon     *svg.QSvgWidget
	errorIcon  *svg.QSvgWidget
	warnIcon   *svg.QSvgWidget
	okLabel    *widgets.QLabel
	errorLabel *widgets.QLabel
	warnLabel  *widgets.QLabel
	svgLoaded  bool
}

// StatuslineFile is
type StatuslineFile struct {
	file        string
	fileType    string
	widget      *widgets.QWidget
	fileLabel   *widgets.QLabel
	folderLabel *widgets.QLabel
	icon        *svg.QSvgWidget
}

// StatuslineFiletype is
type StatuslineFiletype struct {
	filetype string
}

// StatuslinePos is
type StatuslinePos struct {
	ln  int
	col int
}

// StatusMode is
type StatusMode struct {
	label *widgets.QLabel
	mode  string
}

// StatuslineGit is
type StatuslineGit struct {
	branch    string
	file      string
	widget    *widgets.QWidget
	label     *widgets.QLabel
	icon      *svg.QSvgWidget
	svgLoaded bool
}

// StatuslineEncoding is
type StatuslineEncoding struct {
	encoding string
}

func initStatuslineNew() *Statusline {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 1, 0, 0)
	layout := newVFlowLayout(8, 8, 1, 3)
	widget.SetLayout(layout)
	widget.SetObjectName("statusline")
	widget.SetStyleSheet(`
	QWidget#statusline {
		border-top: 2px solid rgba(0, 0, 0, 1);
		background-color: rgba(24, 29, 34, 1);
	}
	* {
		color: rgba(212, 215, 214, 1);
	}
	`)

	modeLabel := widgets.NewQLabel(nil, 0)
	modeLabel.SetContentsMargins(4, 1, 4, 1)
	modeLayout := widgets.NewQHBoxLayout()
	modeLayout.AddWidget(modeLabel, 0, 0)
	modeLayout.SetContentsMargins(0, 0, 0, 0)
	modeWidget := widgets.NewQWidget(nil, 0)
	modeWidget.SetContentsMargins(0, 4, 0, 4)
	modeWidget.SetLayout(modeLayout)
	layout.AddWidget(modeWidget)
	mode := &StatusMode{
		label: modeLabel,
	}

	gitIcon := svg.NewQSvgWidget(nil)
	gitIcon.SetFixedSize2(14, 14)
	gitLabel := widgets.NewQLabel(nil, 0)
	gitLabel.SetContentsMargins(0, 0, 0, 0)
	gitLayout := widgets.NewQHBoxLayout()
	gitLayout.SetContentsMargins(0, 0, 0, 0)
	gitLayout.SetSpacing(2)
	gitLayout.AddWidget(gitIcon, 0, 0)
	gitLayout.AddWidget(gitLabel, 0, 0)
	gitWidget := widgets.NewQWidget(nil, 0)
	gitWidget.SetContentsMargins(0, 0, 0, 0)
	gitWidget.SetLayout(gitLayout)
	gitWidget.Hide()
	layout.AddWidget(gitWidget)
	git := &StatuslineGit{
		widget: gitWidget,
		icon:   gitIcon,
		label:  gitLabel,
	}

	fileIcon := svg.NewQSvgWidget(nil)
	fileIcon.SetFixedSize2(14, 14)
	fileLabel := widgets.NewQLabel(nil, 0)
	fileLabel.SetContentsMargins(0, 0, 0, 0)
	folderLabel := widgets.NewQLabel(nil, 0)
	folderLabel.SetContentsMargins(0, 0, 0, 0)
	folderLabel.SetStyleSheet("color: #838383;")
	folderLabel.SetContentsMargins(0, 0, 0, 0)
	fileLayout := widgets.NewQHBoxLayout()
	fileLayout.SetContentsMargins(0, 0, 0, 0)
	fileLayout.SetSpacing(2)
	fileLayout.AddWidget(fileIcon, 0, 0)
	fileLayout.AddWidget(fileLabel, 0, 0)
	fileLayout.AddWidget(folderLabel, 0, 0)
	fileWidget := widgets.NewQWidget(nil, 0)
	fileWidget.SetContentsMargins(0, 0, 0, 0)
	fileWidget.SetLayout(fileLayout)
	layout.AddWidget(fileWidget)
	file := &StatuslineFile{
		icon:        fileIcon,
		widget:      fileWidget,
		fileLabel:   fileLabel,
		folderLabel: folderLabel,
	}

	okIcon := svg.NewQSvgWidget(nil)
	okIcon.SetFixedSize2(14, 14)
	okLabel := widgets.NewQLabel(nil, 0)
	okLabel.SetContentsMargins(0, 0, 0, 0)
	okLabel.SetText("ok")
	errorIcon := svg.NewQSvgWidget(nil)
	errorIcon.SetFixedSize2(14, 14)
	errorIcon.Hide()
	errorLabel := widgets.NewQLabel(nil, 0)
	errorLabel.SetContentsMargins(0, 0, 0, 0)
	errorLabel.Hide()
	warnIcon := svg.NewQSvgWidget(nil)
	warnIcon.SetFixedSize2(14, 14)
	warnIcon.Hide()
	warnLabel := widgets.NewQLabel(nil, 0)
	warnLabel.SetContentsMargins(0, 0, 0, 0)
	warnLabel.Hide()
	lintLayout := widgets.NewQHBoxLayout()
	lintLayout.SetContentsMargins(0, 0, 0, 0)
	lintLayout.SetSpacing(2)
	lintLayout.AddWidget(okIcon, 0, 0)
	lintLayout.AddWidget(okLabel, 0, 0)
	lintLayout.AddWidget(errorIcon, 0, 0)
	lintLayout.AddWidget(errorLabel, 0, 0)
	lintLayout.AddWidget(warnIcon, 0, 0)
	lintLayout.AddWidget(warnLabel, 0, 0)
	lintWidget := widgets.NewQWidget(nil, 0)
	lintWidget.SetContentsMargins(0, 0, 0, 0)
	lintWidget.SetLayout(lintLayout)
	layout.AddWidget(lintWidget)
	lint := &StatuslineLint{
		widget:     lintWidget,
		okIcon:     okIcon,
		errorIcon:  errorIcon,
		warnIcon:   warnIcon,
		okLabel:    okLabel,
		errorLabel: errorLabel,
		warnLabel:  warnLabel,
	}

	return &Statusline{
		widget: widget,
		mode:   mode,
		git:    git,
		file:   file,
		lint:   lint,
	}
}

func initStatusline(width, height int) *Statusline {
	return &Statusline{}
	// box := ui.NewHorizontalBox()
	// box.SetSize(width, height)

	// fg := newRGBA(212, 215, 214, 1)
	// bg := newRGBA(24, 29, 34, 1)
	// statusline := &Statusline{
	// 	box:            box,
	// 	bg:             bg,
	// 	borderTopWidth: 2,
	// 	paddingLeft:    14,
	// 	paddingRight:   14,
	// 	margin:         14,
	// }

	// area := ui.NewArea(statusline)
	// statusline.area = area
	// statusline.setSize(width, height)
	// statusline.borderTop = &Border{
	// 	width: statusline.borderTopWidth,
	// 	color: newRGBA(0, 0, 0, 1),
	// }
	// box.Append(area, false)

	// mode := &StatusMode{}
	// mode.area = ui.NewArea(mode)
	// mode.bg = bg
	// mode.color = fg
	// mode.paddingTop = 2
	// mode.paddingBottom = mode.paddingTop
	// mode.paddingLeft = 4
	// mode.paddingRight = mode.paddingLeft
	// box.Append(mode.area, false)
	// statusline.mode = mode

	// file := &StatuslineFile{}
	// file.area = ui.NewArea(file)
	// file.bg = bg
	// file.color = fg
	// file.textType = "file"
	// box.Append(file.area, false)
	// statusline.file = file

	// filetype := &StatuslineFiletype{}
	// filetype.area = ui.NewArea(filetype)
	// filetype.bg = bg
	// filetype.color = fg
	// box.Append(filetype.area, false)
	// statusline.filetype = filetype

	// git := &StatuslineGit{}
	// git.area = ui.NewArea(git)
	// git.bg = bg
	// git.color = fg
	// box.Append(git.area, false)
	// statusline.git = git

	// encoding := &StatuslineEncoding{}
	// encoding.area = ui.NewArea(encoding)
	// encoding.bg = bg
	// encoding.color = fg
	// box.Append(encoding.area, false)
	// statusline.encoding = encoding

	// pos := &StatuslinePos{}
	// pos.area = ui.NewArea(pos)
	// pos.text = "Ln 128, Col 119"
	// pos.bg = bg
	// pos.color = fg
	// box.Append(pos.area, false)
	// statusline.pos = pos

	// lint := &StatuslineLint{
	// 	errors:   -1,
	// 	warnings: -1,
	// }
	// lint.area = ui.NewArea(lint)
	// lint.bg = bg
	// lint.color = fg
	// box.Append(lint.area, false)
	// statusline.lint = lint

	// return statusline
}

// Draw the statusline
// func (s *Statusline) Draw(a *ui.Area, dp *ui.AreaDrawParams) {
// 	p := ui.NewPath(ui.Winding)
// 	p.AddRectangle(dp.ClipX, dp.ClipY, dp.ClipWidth, dp.ClipHeight)
// 	p.End()
// dp.Context.Fill(p, &ui.Brush{
// 	Type: ui.Solid,
// 	R:    s.bg.R,
// 	G:    s.bg.G,
// 	B:    s.bg.B,
// 	A:    s.bg.A,
// })
// p.Free()
// s.drawBorder(dp)
// }

func (s *Statusline) redraw(force bool) {
	s.mode.redraw()
	s.git.redraw()
	s.file.redraw()
	s.lint.redraw()
	// return
	// margin := s.paddingLeft
	// margin = s.redrawItem(s.mode, force, margin, true)
	// margin = s.redrawItem(s.git, force, margin, true)
	// margin = s.redrawItem(s.file, force, margin, true)

	// margin = s.paddingRight
	// margin = s.redrawItem(s.filetype, force, margin, false)
	// margin = s.redrawItem(s.encoding, force, margin, false)
	// margin = s.redrawItem(s.pos, force, margin, false)
	// margin = s.redrawItem(s.lint, force, margin, false)
}

func (s *Statusline) redrawItem(item StatuslineItem, force bool, margin int, left bool) int {
	// w, h := item.Redraw(force)
	// if w > 0 {
	// 	y := (s.height-s.borderTopWidth-h)/2 + s.borderTopWidth
	// 	x := 0
	// 	if left {
	// 		x = margin
	// 	} else {
	// 		x = s.width - margin - w
	// 	}
	// 	// item.setPosition(x, y)
	// 	margin += w + s.margin
	// }
	// return margin
	return 0
}

func (s *StatusMode) redraw() {
	if editor.mode == s.mode {
		return
	}
	s.mode = editor.mode
	text := s.mode
	bg := newRGBA(102, 153, 204, 1)
	switch s.mode {
	case "normal":
		text = "normal"
		bg = newRGBA(102, 153, 204, 1)
	case "cmdline_normal":
		text = "normal"
		bg = newRGBA(102, 153, 204, 1)
	case "insert":
		text = "insert"
		bg = newRGBA(153, 199, 148, 1)
	case "visual":
		text = "visual"
		bg = newRGBA(250, 200, 99, 1)
	}
	s.label.SetText(text)
	s.label.SetStyleSheet(fmt.Sprintf("background-color: %s;", bg.String()))
}

// Redraw mode
// func (s *StatusMode) Redraw(force bool) (int, int) {
// w, h := s.getSize()
// if force || editor.mode != s.mode {
// 	s.mode = editor.mode
// 	switch s.mode {
// 	case "normal":
// 		s.text = "normal"
// 		s.bg = newRGBA(102, 153, 204, 1)
// 	case "cmdline_normal":
// 		s.text = "normal"
// 		s.bg = newRGBA(102, 153, 204, 1)
// 	case "insert":
// 		s.text = "insert"
// 		s.bg = newRGBA(153, 199, 148, 1)
// 	case "visual":
// 		s.text = "visual"
// 		s.bg = newRGBA(250, 200, 99, 1)
// 	default:
// 		s.bg = newRGBA(102, 153, 204, 1)
// 		s.text = s.mode
// 	}
// 	w, h = s.getSize()
// 	s.setSize(w, h)
// 	ui.QueueMain(func() {
// 		s.area.QueueRedrawAll()
// 	})
// }
// return w, h
// }

func (s *StatuslineGit) redraw() {
	file := ""
	editor.nvim.Call("expand", &file, "%:p")

	if file == "" || strings.HasPrefix(file, "term://") {
		s.file = file
		if s.branch == "" {
			return
		}
		s.widget.Hide()
		s.branch = ""
		return
	}

	if s.file == file {
		return
	}

	s.file = file
	dir := filepath.Dir(file)
	out, err := exec.Command("git", "-C", dir, "branch").Output()
	if err != nil {
		if s.branch == "" {
			return
		}
		s.widget.Hide()
		s.branch = ""
		return
	}

	branch := ""
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "* ") {
			if strings.HasPrefix(line, "* (HEAD detached at ") {
				branch = line[20 : len(line)-1]
			} else {
				branch = line[2:]
			}
		}
	}
	_, err = exec.Command("git", "-C", dir, "diff", "--quiet").Output()
	if err != nil {
		branch += "*"
	}

	if s.branch != branch {
		s.branch = branch
		s.label.SetText(branch)
		if !s.svgLoaded {
			s.svgLoaded = true
			svgContent := getSvg("git", newRGBA(212, 215, 214, 1))
			s.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		}
		s.widget.Show()
	}
}

// Redraw git
// func (s *StatuslineGit) Redraw(force bool) (int, int) {
// w, h := s.getSize()

// file := ""
// editor.nvim.Call("expand", &file, "%:p")

// if file == "" || strings.HasPrefix(file, "term://") {
// 	s.file = file
// 	if s.branch == "" {
// 		return 0, 0
// 	}
// 	s.branch = ""
// 	s.svg = ""
// 	s.setSize(0, 0)
// 	return 0, 0
// }

// if s.file == file {
// 	return w, h
// }

// s.file = file
// dir := filepath.Dir(file)
// out, err := exec.Command("git", "-C", dir, "branch").Output()
// if err != nil {
// 	if s.branch == "" {
// 		return 0, 0
// 	}
// 	s.branch = ""
// 	s.svg = ""
// 	s.setSize(0, 0)
// 	return 0, 0
// }

// branch := ""
// for _, line := range strings.Split(string(out), "\n") {
// 	if strings.HasPrefix(line, "* ") {
// 		if strings.HasPrefix(line, "* (HEAD detached at ") {
// 			branch = line[20 : len(line)-1]
// 		} else {
// 			branch = line[2:]
// 		}
// 	}
// }
// _, err = exec.Command("git", "-C", dir, "diff", "--quiet").Output()
// if err != nil {
// 	branch += "*"
// }

// if force || s.branch != branch {
// 	s.branch = branch
// 	s.text = branch
// 	s.paddingLeft = editor.font.height + 2
// 	s.svg = "git"
// 	s.svgColor = s.color
// 	w, h = s.getSize()
// 	s.setSize(w, h)
// 	ui.QueueMain(func() {
// 		s.area.QueueRedrawAll()
// 	})
// }
// return w, h
// }

func (s *StatuslineFile) redraw() {
	file := ""
	editor.nvim.Call("expand", &file, "%")
	if file == "" {
		file = "[No Name]"
	}

	if file == s.file {
		return
	}

	s.file = file

	base := filepath.Base(file)
	dir := filepath.Dir(file)
	if dir == "." {
		dir = ""
	}
	if strings.HasPrefix(file, "term://") {
		base = file
		dir = ""
	}
	fileType := getFileType(file)
	if s.fileType != fileType {
		s.fileType = fileType
		svgContent := getSvg(fileType, nil)
		s.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	}
	s.fileLabel.SetText(base)
	s.folderLabel.SetText(dir)
}

// Redraw file
// func (s *StatuslineFile) Redraw(force bool) (int, int) {
// w, h := s.getSize()
// file := ""
// editor.nvim.Call("expand", &file, "%")
// if file == "" {
// 	file = "[No Name]"
// }
// if force || file != s.file {
// 	if strings.HasPrefix(file, "term://") {
// 		s.textType = ""
// 	} else {
// 		s.textType = "file"
// 	}
// 	s.file = file
// 	s.text = file
// 	s.svg = getFileType(s.file)
// 	s.paddingLeft = editor.font.height + 2
// 	w, h = s.getSize()
// 	s.setSize(w, h)
// 	ui.QueueMain(func() {
// 		s.area.QueueRedrawAll()
// 	})
// }
// return w, h
// }

// Redraw pos
// func (s *StatuslinePos) Redraw(force bool) (int, int) {
// w, h := s.getSize()
// pos := new([]interface{})
// err := editor.nvim.Call("getpos", pos, ".")
// if err != nil {
// 	return 0, 0
// }
// ln := reflectToInt((*pos)[1])
// col := reflectToInt((*pos)[2])
// if force || ln != s.ln || col != s.col {
// 	s.ln = ln
// 	s.col = col
// 	s.text = fmt.Sprintf("Ln %d, Col %d", ln, col)
// 	w, h = s.getSize()
// 	s.setSize(w, h)
// 	ui.QueueMain(func() {
// 		s.area.QueueRedrawAll()
// 	})
// }
// return w, h
// }

// Redraw encoding
// func (s *StatuslineEncoding) Redraw(force bool) (int, int) {
// w, h := s.getSize()
// encoding := ""
// curbuf, _ := editor.nvim.CurrentBuffer()
// editor.nvim.BufferOption(curbuf, "fileencoding", &encoding)
// if force || s.encoding != encoding {
// 	s.encoding = encoding
// 	s.text = encoding
// 	w, h = s.getSize()
// 	s.setSize(w, h)
// 	ui.QueueMain(func() {
// 		s.area.QueueRedrawAll()
// 	})
// }
// return w, h
// }

// Redraw filetype
// func (s *StatuslineFiletype) Redraw(force bool) (int, int) {
// w, h := s.getSize()
// filetype := ""
// curbuf, _ := editor.nvim.CurrentBuffer()
// editor.nvim.BufferOption(curbuf, "filetype", &filetype)
// if force || s.filetype != filetype {
// 	s.filetype = filetype
// 	s.text = filetype
// 	w, h = s.getSize()
// 	s.setSize(w, h)
// 	ui.QueueMain(func() {
// 		s.area.QueueRedrawAll()
// 	})
// }
// return w, h
// }

func (s *StatuslineLint) redraw() {
	if !s.svgLoaded {
		s.svgLoaded = true
		svgContent := getSvg("check", newRGBA(141, 193, 73, 1))
		s.okIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		svgContent = getSvg("cross", newRGBA(204, 62, 68, 1))
		s.errorIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		svgContent = getSvg("exclamation", nil)
		s.warnIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	}

	result := new([]map[string]interface{})
	err := editor.nvim.Call("getloclist", result, "winnr(\"$\")")
	if err != nil {
		s.errors = -1
		s.warnings = -1
		return
	}

	errors := 0
	warnings := 0
	for _, loc := range *result {
		locType := loc["type"].(string)
		switch locType {
		case "E":
			errors++
		case "W":
			warnings++
		}
	}

	if errors == s.errors && warnings == s.warnings {
		return
	}
	s.errors = errors
	s.warnings = warnings
	if errors == 0 && warnings == 0 {
		s.okIcon.Show()
		s.okLabel.Show()
		s.errorIcon.Hide()
		s.errorLabel.Hide()
		s.warnIcon.Hide()
		s.warnLabel.Hide()
	} else {
		s.okIcon.Hide()
		s.okLabel.Hide()
		s.errorLabel.SetText(strconv.Itoa(errors))
		s.warnLabel.SetText(strconv.Itoa(warnings))
		s.errorIcon.Show()
		s.errorLabel.Show()
		s.warnIcon.Show()
		s.warnLabel.Show()
	}
}

// Redraw lint
// func (s *StatuslineLint) Redraw(force bool) (int, int) {
// w, h := s.getSize()
// result := new([]map[string]interface{})
// err := editor.nvim.Call("getloclist", result, "winnr(\"$\")")
// if err != nil {
// 	fmt.Println("lint error", err)
// 	s.errors = -1
// 	s.warnings = -1
// 	return 0, 0
// }
// errors := 0
// warnings := 0
// for _, loc := range *result {
// 	locType := loc["type"].(string)
// 	switch locType {
// 	case "E":
// 		errors++
// 	case "W":
// 		warnings++
// 	}
// }
// if force || s.errors != errors || s.warnings != warnings {
// 	s.errors = errors
// 	s.warnings = warnings
// 	if errors == 0 && warnings == 0 {
// 		s.text = "ok"
// 		s.svg = "check"
// 		s.svgColor = newRGBA(141, 193, 73, 1)
// 		s.svgSecond = ""
// 		s.paddingLeft = editor.font.height + 2
// 		w, h = s.getSize()
// 		s.setSize(w, h)
// 	} else {
// 		s.text = fmt.Sprintf("%d   %d", s.errors, s.warnings)
// 		s.svg = "cross"
// 		s.svgColor = newRGBA(204, 62, 68, 1)
// 		s.svgSecond = "exclamation"
// 		s.svgSecondPadding = int(editor.font.truewidth * float64(len(fmt.Sprintf("%d", s.errors))+3))
// 		s.paddingLeft = editor.font.height + 2
// 		w, h = s.getSize()
// 		s.setSize(w, h)
// 	}
// 	ui.QueueMain(func() {
// 		s.area.QueueRedrawAll()
// 	})
// }
// return w, h
// }

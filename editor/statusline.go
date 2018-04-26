package editor

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dzhou121/gonvim/osdepend"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

// Statusline is
type Statusline struct {
	ws     *Workspace
	widget *widgets.QWidget
	bg     *RGBA

	borderTopWidth int
	paddingLeft    int
	paddingRight   int
	margin         int
	height         int

	pos      *StatuslinePos
	mode     *StatusMode
	file     *StatuslineFile
	filetype *StatuslineFiletype
	git      *StatuslineGit
	encoding *StatuslineEncoding
	lint     *StatuslineLint
	updates  chan []interface{}
}

// StatuslineLint is
type StatuslineLint struct {
	s          *Statusline
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
	s           *Statusline
	file        string
	fileType    string
	widget      *widgets.QWidget
	fileLabel   *widgets.QLabel
	folderLabel *widgets.QLabel
	icon        *svg.QSvgWidget
	base        string
	dir         string
}

// StatuslineFiletype is
type StatuslineFiletype struct {
	filetype string
	label    *widgets.QLabel
}

// StatuslinePos is
type StatuslinePos struct {
	ln    int
	col   int
	label *widgets.QLabel
	text  string
}

// StatusMode is
type StatusMode struct {
	s         *Statusline
	label     *widgets.QLabel
	//mode    string
 mode      string
 modeIcon  *svg.QSvgWidget
	svgLoaded bool
	text      string
	//bg        *RGBA
}

// StatuslineGit is
type StatuslineGit struct {
	s         *Statusline
	branch    string
	file      string
	widget    *widgets.QWidget
	label     *widgets.QLabel
	icon      *svg.QSvgWidget
	svgLoaded bool
	hidden    bool
}

// StatuslineEncoding is
type StatuslineEncoding struct {
	encoding string
	label    *widgets.QLabel
}

func initStatuslineNew() *Statusline {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 1, 0, 0)
	layout := newVFlowLayout(14, 8, 1, 3, 0)
	widget.SetLayout(layout)
	widget.SetObjectName("statusline")

	s := &Statusline{
		widget:  widget,
		updates: make(chan []interface{}, 1000),
	}

	modeIcon := svg.NewQSvgWidget(nil)
	modeIcon.SetFixedSize2(14, 14)
	modeLabel := widgets.NewQLabel(nil, 0)
	modeLabel.SetContentsMargins(0, 0, 0, 0)
	modeLayout := widgets.NewQHBoxLayout()
	modeLayout.AddWidget(modeIcon, 0, 0)
	//modeLayout.AddWidget(modeLabel, 0, 0)
	modeLayout.SetContentsMargins(0, 0, 0, 0)
	modeWidget := widgets.NewQWidget(nil, 0)
	modeWidget.SetContentsMargins(0, 5, 0, 5)
	modeWidget.SetLayout(modeLayout)

	mode := &StatusMode{
		s:     s,
		label: modeLabel,
  modeIcon: modeIcon,
	}
	s.mode = mode

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
	git := &StatuslineGit{
		s:      s,
		widget: gitWidget,
		icon:   gitIcon,
		label:  gitLabel,
	}
	s.git = git

	fileIcon := svg.NewQSvgWidget(nil)
	fileIcon.SetFixedSize2(13, 13)
	fileLabel := widgets.NewQLabel(nil, 0)
	fileLabel.SetContentsMargins(0, 6, 0, 6)
	folderLabel := widgets.NewQLabel(nil, 0)
	folderLabel.SetContentsMargins(0, 0, 0, 0)
	folderLabel.SetContentsMargins(0, 0, 0, 0)
	fileLayout := widgets.NewQHBoxLayout()
	fileLayout.SetContentsMargins(0, 0, 0, 0)
	fileLayout.SetSpacing(3)
	fileLayout.AddWidget(fileIcon, 0, 0)
	fileLayout.AddWidget(fileLabel, 0, 0)
	fileLayout.AddWidget(folderLabel, 0, 0)
	fileWidget := widgets.NewQWidget(nil, 0)
	fileWidget.SetContentsMargins(-1, 0, 0, 0)
	fileWidget.SetLayout(fileLayout)
	file := &StatuslineFile{
		s:           s,
		icon:        fileIcon,
		widget:      fileWidget,
		fileLabel:   fileLabel,
		folderLabel: folderLabel,
	}
	s.file = file

	encodingLabel := widgets.NewQLabel(nil, 0)
	encodingLabel.SetContentsMargins(0, 0, 0, 0)
	encoding := &StatuslineEncoding{
		label: encodingLabel,
	}
	s.encoding = encoding

	posLabel := widgets.NewQLabel(nil, 0)
	posLabel.SetContentsMargins(0, 0, 0, 0)
	pos := &StatuslinePos{
		label: posLabel,
	}
	s.pos = pos

	filetypeLabel := widgets.NewQLabel(nil, 0)
	filetypeLabel.SetContentsMargins(0, 0, 0, 0)
	filetype := &StatuslineFiletype{
		label: filetypeLabel,
	}
	s.filetype = filetype

	okIcon := svg.NewQSvgWidget(nil)
	okIcon.SetFixedSize2(14, 14)
	okLabel := widgets.NewQLabel(nil, 0)
	okLabel.SetContentsMargins(0, 0, 0, 0)
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
	lintLayout.SetSpacing(0)
	lintLayout.AddWidget(okIcon, 0, 0)
	//lintLayout.AddWidget(okLabel, 0, 0)
	lintLayout.AddWidget(errorIcon, 0, 0)
	lintLayout.AddWidget(errorLabel, 0, 0)
	lintLayout.AddWidget(warnIcon, 0, 0)
	lintLayout.AddWidget(warnLabel, 0, 0)
	lintWidget := widgets.NewQWidget(nil, 0)
	lintWidget.SetContentsMargins(0, 0, 0, 0)
	lintWidget.SetLayout(lintLayout)
	lint := &StatuslineLint{
		s:          s,
		widget:     lintWidget,
		okIcon:     okIcon,
		errorIcon:  errorIcon,
		warnIcon:   warnIcon,
		okLabel:    okLabel,
		errorLabel: errorLabel,
		warnLabel:  warnLabel,
		errors:     -1,
		warnings:   -1,
	}
	s.lint = lint

	layout.AddWidget(modeWidget)
	layout.AddWidget(gitWidget)
	layout.AddWidget(fileWidget)
	layout.AddWidget(filetypeLabel)
	layout.AddWidget(encodingLabel)
	layout.AddWidget(lintWidget)
	layout.AddWidget(posLabel)

	return s
}

func (s *Statusline) subscribe() {
	if !s.ws.drawStatusline {
		s.widget.Hide()
		return
	}
	s.ws.signal.ConnectStatuslineSignal(func() {
		updates := <-s.updates
		s.handleUpdates(updates)
	})
	s.ws.signal.ConnectLintSignal(func() {
		s.lint.update()
	})
	s.ws.signal.ConnectGitSignal(func() {
		s.git.update()
	})
	s.ws.nvim.RegisterHandler("statusline", func(updates ...interface{}) {
		s.updates <- updates
		s.ws.signal.StatuslineSignal()
	})
	s.ws.nvim.Subscribe("statusline")
	s.ws.nvim.Command(`autocmd BufEnter * call rpcnotify(0, "statusline", "bufenter", expand("%:p"), &filetype, &fileencoding)`)
	s.ws.nvim.Command(`autocmd CursorMoved,CursorMovedI * call rpcnotify(0, "statusline", "cursormoved", getpos("."))`)
}

func (s *Statusline) handleUpdates(updates []interface{}) {
	event := updates[0].(string)
	switch event {
	case "bufenter":
		file := updates[1].(string)
		filetype := updates[2].(string)
		encoding := updates[3].(string)
		s.file.redraw(file)
		s.filetype.redraw(filetype)
		s.encoding.redraw(encoding)
		go s.git.redraw(file)
  bg := s.ws.screen.highlight.background
  fg := s.ws.screen.highlight.foreground
  s.ws.statusline.widget.SetStyleSheet(fmt.Sprintf("QWidget#statusline {	border-top: 0px solid rgba(%d, %d, %d, 1);	background-color: rgba(%d, %d, %d, 1);	}	* {	color: rgba(%d, %d, %d, 1);	}", shiftColor(bg, 20).R, shiftColor(bg, 20).G, shiftColor(bg, 20).B, shiftColor(bg, 10).R, shiftColor(bg, 10).G, shiftColor(bg, 10).B, shiftColor(fg, -12).R, shiftColor(fg, -12).G, shiftColor(fg, -12).B))
	case "cursormoved":
		pos := updates[1].([]interface{})
		ln := reflectToInt(pos[1])
		col := reflectToInt(pos[2]) + reflectToInt(pos[3])
		s.pos.redraw(ln, col)
	default:
		fmt.Println("unhandled statusline event", event)
	}
}

//func (s *StatusMode) update() {
	//s.label.SetText(s.text)
	//s.label.SetStyleSheet(fmt.Sprintf("background-color: %s;", s.bg.String()))
//}

func (s *StatusMode) redraw() {
	if s.s.ws.mode == s.mode {
		return
	}

 fg := s.s.ws.screen.highlight.foreground

	s.mode = s.s.ws.mode
	text := s.mode
	//bg := newRGBA(102, 153, 204, 1)
	switch s.mode {
	case "normal":
		text = "normal"
		//bg = newRGBA(102, 153, 204, 1)
	case "cmdline_normal":
		text = "normal"
		//bg = newRGBA(102, 153, 204, 1)
	case "insert":
		text = "insert"
		//bg = newRGBA(153, 199, 148, 1)
	 svgContent := s.s.ws.getSvg("edit", newRGBA(shiftColor(fg, -12).R, shiftColor(fg, -12).G, shiftColor(fg, -12).B, 1))
	 s.modeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	case "visual":
		text = "visual"
		//bg = newRGBA(250, 200, 99, 1)
	 svgContent := s.s.ws.getSvg("select", newRGBA(shiftColor(fg, -12).R, shiftColor(fg, -12).G, shiftColor(fg, -12).B, 1))
	 s.modeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	case "replace":
		text = "replace"
		//bg = newRGBA(250, 200, 99, 1)
	 svgContent := s.s.ws.getSvg("transform", newRGBA(shiftColor(fg, -12).R, shiftColor(fg, -12).G, shiftColor(fg, -12).B, 1))
	 s.modeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	}
 if s.mode == "normal" {
	 s.modeIcon.Hide()
 } else {
	 s.modeIcon.Show()
 }
	s.text = text
	//s.bg = bg
}

func (s *StatuslineGit) hide() {
	if s.hidden {
		return
	}
	s.hidden = true
	s.s.ws.signal.GitSignal()
}

func (s *StatuslineGit) update() {
	if s.hidden {
		s.widget.Hide()
		return
	}
 fg := s.s.ws.screen.highlight.foreground
	s.label.SetText(s.branch)
	if !s.svgLoaded {
		s.svgLoaded = true
	 svgContent := s.s.ws.getSvg("git", newRGBA(shiftColor(fg, -12).R, shiftColor(fg, -12).G, shiftColor(fg, -12).B, 1))
		s.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	}
	s.widget.Show()
}

func (s *StatuslineGit) redraw(file string) {
	if file == "" || strings.HasPrefix(file, "term://") {
		s.file = file
		s.hide()
		s.branch = ""
		return
	}

	if s.file == file {
		return
	}

	s.file = file
	dir := filepath.Dir(file)
 cmd := exec.Command("git", "-C", dir, "branch")
 osdepend.PrepareRunProc(cmd)
	out, err := cmd.Output()
	if err != nil {
		s.hide()
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
 cmd = exec.Command("git", "-C", dir, "diff", "--quiet")
 osdepend.PrepareRunProc(cmd)
	_, err = cmd.Output()
	if err != nil {
		branch += "*"
	}

	if s.branch != branch {
		s.branch = branch
		s.hidden = false
		s.s.ws.signal.GitSignal()
	}
}

func (s *StatuslineFile) updateIcon() {
	svgContent := s.s.ws.getSvg(s.fileType, nil)
	s.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
}

func (s *StatuslineFile) redraw(file string) {
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
		s.updateIcon()
	}
	if s.base != base {
		s.base = base
		s.fileLabel.SetText(s.base)
	}
	if s.dir != dir {
		s.dir = dir
		s.folderLabel.SetText(s.dir)
	}
}

func (s *StatuslinePos) redraw(ln, col int) {
	if ln == s.ln && col == s.col {
		return
	}
	text := fmt.Sprintf("%d,%d", ln, col)
	if text != s.text {
		s.text = text
		s.label.SetText(text)
	}
}

func (s *StatuslineEncoding) redraw(encoding string) {
	if s.encoding == encoding {
		return
	}
	s.encoding = encoding
	s.label.SetText(s.encoding)
}

func (s *StatuslineFiletype) redraw(filetype string) {
	if filetype == s.filetype {
		return
	}
	s.filetype = filetype
	s.label.SetText(s.filetype)
}

func (s *StatuslineLint) update() {
	if !s.svgLoaded {
		s.svgLoaded = true
		//svgContent := s.s.ws.getSvg("check", newRGBA(141, 193, 73, 1))
		//s.okIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	 var svgErrContent, svgWrnContent string
	 if s.errors != 0 {
	  svgErrContent = s.s.ws.getSvg("cross", newRGBA(204, 62, 68, 1))
  } else {
	  svgErrContent = s.s.ws.getSvg("cross", nil)
  }
	 if s.warnings != 0 {
	  svgWrnContent = s.s.ws.getSvg("exclamation", newRGBA(203, 203, 65, 1))
  } else {
	  svgWrnContent = s.s.ws.getSvg("exclamation", nil)
  }
	 s.errorIcon.Load2(core.NewQByteArray2(svgErrContent, len(svgErrContent)))
	 s.warnIcon.Load2(core.NewQByteArray2(svgWrnContent, len(svgWrnContent)))
	}

	//if s.errors == 0 && s.warnings == 0 {
	//	s.okIcon.Show()
	//	//s.okLabel.SetText("ok")
	//	s.okLabel.Show()
	//	s.errorIcon.Hide()
	//	s.errorLabel.Hide()
	//	s.warnIcon.Hide()
	//	s.warnLabel.Hide()
	//} else {
	s.okIcon.Hide()
	s.okLabel.Hide()
	s.errorLabel.SetText(strconv.Itoa(s.errors))
	s.warnLabel.SetText(strconv.Itoa(s.warnings))
	s.errorIcon.Show()
	s.errorLabel.Show()
	s.warnIcon.Show()
	s.warnLabel.Show()
	//}
}

func (s *StatuslineLint) redraw(errors, warnings int) {
	if errors == s.errors && warnings == s.warnings {
   return
	}
	var svgErrContent, svgWrnContent string
	if errors != 0 {
	 svgErrContent = s.s.ws.getSvg("cross", newRGBA(204, 62, 68, 1))
 } else {
	 svgErrContent = s.s.ws.getSvg("cross", nil)
 }
	if warnings != 0 {
	 svgWrnContent = s.s.ws.getSvg("exclamation", newRGBA(203, 203, 65, 1))
 } else {
	 svgWrnContent = s.s.ws.getSvg("exclamation", nil)
 }
	s.errorIcon.Load2(core.NewQByteArray2(svgErrContent, len(svgErrContent)))
	s.warnIcon.Load2(core.NewQByteArray2(svgWrnContent, len(svgWrnContent)))
	s.errors = errors
	s.warnings = warnings
	s.s.ws.signal.LintSignal()
}

package gonvim

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dzhou121/ui"
)

// StatuslineItem is
type StatuslineItem interface {
	Redraw() (int, int)
	setPosition(x, y int)
}

// Statusline is
type Statusline struct {
	AreaHandler
	box *ui.Box
	bg  *RGBA

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
	SpanHandler
	errors   int
	warnings int
}

// StatuslineFile is
type StatuslineFile struct {
	SpanHandler
	file string
}

// StatuslineFiletype is
type StatuslineFiletype struct {
	SpanHandler
	filetype string
}

// StatuslinePos is
type StatuslinePos struct {
	SpanHandler
	ln  int
	col int
}

// StatusMode is
type StatusMode struct {
	SpanHandler
	mode string
}

// StatuslineGit is
type StatuslineGit struct {
	SpanHandler
	branch string
	file   string
}

// StatuslineEncoding is
type StatuslineEncoding struct {
	SpanHandler
	encoding string
}

func initStatusline(width, height int) *Statusline {
	box := ui.NewHorizontalBox()
	box.SetSize(width, height)

	fg := newRGBA(212, 215, 214, 1)
	bg := newRGBA(24, 29, 34, 1)
	statusline := &Statusline{
		box:            box,
		bg:             bg,
		borderTopWidth: 2,
		paddingLeft:    14,
		paddingRight:   14,
		margin:         14,
	}

	area := ui.NewArea(statusline)
	statusline.area = area
	statusline.setSize(width, height)
	statusline.borderTop = &Border{
		width: statusline.borderTopWidth,
		color: newRGBA(0, 0, 0, 1),
	}
	box.Append(area, false)

	pos := &StatuslinePos{}
	pos.area = ui.NewArea(pos)
	pos.text = "Ln 128, Col 119"
	pos.bg = bg
	pos.color = fg
	box.Append(pos.area, false)
	statusline.pos = pos

	mode := &StatusMode{}
	mode.area = ui.NewArea(mode)
	mode.bg = bg
	mode.color = fg
	mode.paddingTop = 2
	mode.paddingBottom = mode.paddingTop
	mode.paddingLeft = 4
	mode.paddingRight = mode.paddingLeft
	box.Append(mode.area, false)
	statusline.mode = mode

	file := &StatuslineFile{}
	file.area = ui.NewArea(file)
	file.bg = bg
	file.color = fg
	file.textType = "file"
	box.Append(file.area, false)
	statusline.file = file

	filetype := &StatuslineFiletype{}
	filetype.area = ui.NewArea(filetype)
	filetype.bg = bg
	filetype.color = fg
	box.Append(filetype.area, false)
	statusline.filetype = filetype

	git := &StatuslineGit{}
	git.area = ui.NewArea(git)
	git.bg = bg
	git.color = fg
	box.Append(git.area, false)
	statusline.git = git

	encoding := &StatuslineEncoding{}
	encoding.area = ui.NewArea(encoding)
	encoding.bg = bg
	encoding.color = fg
	box.Append(encoding.area, false)
	statusline.encoding = encoding

	lint := &StatuslineLint{
		errors:   -1,
		warnings: -1,
	}
	lint.area = ui.NewArea(lint)
	lint.bg = bg
	lint.color = fg
	box.Append(lint.area, false)
	statusline.lint = lint

	return statusline
}

// Draw the statusline
func (s *Statusline) Draw(a *ui.Area, dp *ui.AreaDrawParams) {
	p := ui.NewPath(ui.Winding)
	p.AddRectangle(dp.ClipX, dp.ClipY, dp.ClipWidth, dp.ClipHeight)
	p.End()
	dp.Context.Fill(p, &ui.Brush{
		Type: ui.Solid,
		R:    s.bg.R,
		G:    s.bg.G,
		B:    s.bg.B,
		A:    s.bg.A,
	})
	p.Free()
	s.drawBorder(dp)
}

func (s *Statusline) redraw() {
	margin := s.paddingLeft
	margin = s.redrawItem(s.mode, margin, true)
	margin = s.redrawItem(s.git, margin, true)
	margin = s.redrawItem(s.file, margin, true)

	margin = s.paddingRight
	margin = s.redrawItem(s.filetype, margin, false)
	margin = s.redrawItem(s.encoding, margin, false)
	margin = s.redrawItem(s.pos, margin, false)
	margin = s.redrawItem(s.lint, margin, false)
}

func (s *Statusline) redrawItem(item StatuslineItem, margin int, left bool) int {
	w, h := item.Redraw()
	if w > 0 {
		y := (s.height-s.borderTopWidth-h)/2 + s.borderTopWidth
		x := 0
		if left {
			x = margin
		} else {
			x = s.width - margin - w
		}
		item.setPosition(x, y)
		margin += w + s.margin
	}
	return margin
}

// Redraw mode
func (s *StatusMode) Redraw() (int, int) {
	w, h := s.getSize()
	if editor.mode != s.mode {
		s.mode = editor.mode
		switch s.mode {
		case "normal":
			s.text = "normal"
			s.bg = newRGBA(102, 153, 204, 1)
		case "cmdline_normal":
			s.text = "normal"
			s.bg = newRGBA(102, 153, 204, 1)
		case "insert":
			s.text = "insert"
			s.bg = newRGBA(153, 199, 148, 1)
		case "visual":
			s.text = "visual"
			s.bg = newRGBA(250, 200, 99, 1)
		default:
			s.bg = newRGBA(102, 153, 204, 1)
			s.text = s.mode
		}
		w, h = s.getSize()
		s.setSize(w, h)
		ui.QueueMain(func() {
			s.area.QueueRedrawAll()
		})
	}
	return w, h
}

// Redraw git
func (s *StatuslineGit) Redraw() (int, int) {
	w, h := s.getSize()

	file := ""
	editor.nvim.Call("expand", &file, "%:p")

	if file == "" || strings.HasPrefix(file, "term://") {
		s.file = file
		if s.branch == "" {
			return 0, 0
		}
		s.branch = ""
		s.svg = ""
		s.setSize(0, 0)
		return 0, 0
	}

	if s.file == file {
		return w, h
	}

	s.file = file
	dir := filepath.Dir(file)
	out, err := exec.Command("git", "-C", dir, "branch").Output()
	if err != nil {
		if s.branch == "" {
			return 0, 0
		}
		s.branch = ""
		s.svg = ""
		s.setSize(0, 0)
		return 0, 0
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
		s.text = branch
		s.paddingLeft = editor.font.height + 2
		s.svg = "git"
		s.svgColor = s.color
		w, h = s.getSize()
		s.setSize(w, h)
		ui.QueueMain(func() {
			s.area.QueueRedrawAll()
		})
	}
	return w, h
}

// Redraw file
func (s *StatuslineFile) Redraw() (int, int) {
	w, h := s.getSize()
	file := ""
	editor.nvim.Call("expand", &file, "%")
	if file == "" {
		file = "[No Name]"
	}
	if file != s.file {
		if strings.HasPrefix(file, "term://") {
			s.textType = ""
		} else {
			s.textType = "file"
		}
		s.file = file
		s.text = file
		s.svg = getFileType(s.file)
		s.paddingLeft = editor.font.height + 2
		w, h = s.getSize()
		s.setSize(w, h)
		ui.QueueMain(func() {
			s.area.QueueRedrawAll()
		})
	}
	return w, h
}

// Redraw pos
func (s *StatuslinePos) Redraw() (int, int) {
	w, h := s.getSize()
	pos := new([]interface{})
	err := editor.nvim.Call("getpos", pos, ".")
	if err != nil {
		return 0, 0
	}
	ln := reflectToInt((*pos)[1])
	col := reflectToInt((*pos)[2])
	if ln != s.ln || col != s.col {
		s.ln = ln
		s.col = col
		s.text = fmt.Sprintf("Ln %d, Col %d", ln, col)
		w, h = s.getSize()
		s.setSize(w, h)
		ui.QueueMain(func() {
			s.area.QueueRedrawAll()
		})
	}
	return w, h
}

// Redraw encoding
func (s *StatuslineEncoding) Redraw() (int, int) {
	w, h := s.getSize()
	encoding := ""
	curbuf, _ := editor.nvim.CurrentBuffer()
	editor.nvim.BufferOption(curbuf, "fileencoding", &encoding)
	if s.encoding != encoding {
		s.encoding = encoding
		s.text = encoding
		w, h = s.getSize()
		s.setSize(w, h)
		ui.QueueMain(func() {
			s.area.QueueRedrawAll()
		})
	}
	return w, h
}

// Redraw filetype
func (s *StatuslineFiletype) Redraw() (int, int) {
	w, h := s.getSize()
	filetype := ""
	curbuf, _ := editor.nvim.CurrentBuffer()
	editor.nvim.BufferOption(curbuf, "filetype", &filetype)
	if s.filetype != filetype {
		s.filetype = filetype
		s.text = filetype
		w, h = s.getSize()
		s.setSize(w, h)
		ui.QueueMain(func() {
			s.area.QueueRedrawAll()
		})
	}
	return w, h
}

// Redraw lint
func (s *StatuslineLint) Redraw() (int, int) {
	w, h := s.getSize()
	result := new([]map[string]interface{})
	err := editor.nvim.Call("getloclist", result, "winnr(\"$\")")
	if err != nil {
		fmt.Println("lint error", err)
		s.errors = -1
		s.warnings = -1
		return 0, 0
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
	if s.errors != errors || s.warnings != warnings {
		s.errors = errors
		s.warnings = warnings
		if errors == 0 && warnings == 0 {
			s.text = "ok"
			s.svg = "check"
			s.svgColor = newRGBA(141, 193, 73, 1)
			s.svgSecond = ""
			s.paddingLeft = editor.font.height + 2
			w, h = s.getSize()
			s.setSize(w, h)
		} else {
			s.text = fmt.Sprintf("%d   %d", s.errors, s.warnings)
			s.svg = "cross"
			s.svgColor = newRGBA(204, 62, 68, 1)
			s.svgSecond = "exclamation"
			s.svgSecondPadding = editor.font.width * (len(fmt.Sprintf("%d", s.errors)) + 3)
			s.paddingLeft = editor.font.height + 2
			w, h = s.getSize()
			s.setSize(w, h)
		}
		ui.QueueMain(func() {
			s.area.QueueRedrawAll()
		})
	}
	return w, h
}

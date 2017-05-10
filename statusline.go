package gonvim

import (
	"fmt"
	"path/filepath"
	"strings"

	git "gopkg.in/src-d/go-git.v4"

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
	margin         int

	pos  *StatuslinePos
	mode *StatusMode
	file *StatuslineFile
	git  *StatuslineGit
}

// StatuslineFile is
type StatuslineFile struct {
	SpanHandler
	file string
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

	git := &StatuslineGit{}
	git.area = ui.NewArea(git)
	git.bg = bg
	git.color = fg
	box.Append(git.area, false)
	statusline.git = git

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
	margin = s.redrawItem(s.mode, margin)
	margin = s.redrawItem(s.git, margin)

	w, h := s.pos.getSize()
	pos := new([]interface{})
	editor.nvim.Call("getpos", pos, ".")
	ln := reflectToInt((*pos)[1])
	col := reflectToInt((*pos)[2])
	if ln != s.pos.ln || col != s.pos.col {
		s.pos.ln = ln
		s.pos.col = col
		s.pos.text = fmt.Sprintf("Ln %d, Col %d", ln, col)
		w, h = s.pos.getSize()
		s.pos.setSize(w, h)
		ui.QueueMain(func() {
			s.pos.area.QueueRedrawAll()
		})
	}
	s.pos.setPosition(margin, (s.height-s.borderTopWidth-h)/2+s.borderTopWidth)
	margin += w + s.margin

	w, h = s.file.getSize()
	file := ""
	editor.nvim.Call("expand", &file, "%")
	if file != s.file.file {
		s.file.file = file
		s.file.text = file
		w, h = s.file.getSize()
		s.file.setSize(w, h)
		ui.QueueMain(func() {
			s.file.area.QueueRedrawAll()
		})
	}
	s.file.setPosition(margin, (s.height-s.borderTopWidth-h)/2+s.borderTopWidth)
	margin += w + s.margin

}

func (s *Statusline) redrawItem(item StatuslineItem, margin int) int {
	w, h := item.Redraw()
	item.setPosition(margin, (s.height-s.borderTopWidth-h)/2+s.borderTopWidth)
	return margin + w + s.margin
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
		if s.branch == "" {
			return 0, 0
		}
		s.branch = ""
		s.setSize(0, 0)
		return 0, 0
	}

	dir := filepath.Dir(file)
	repo, err := git.PlainOpen(dir)
	for dir != "/" && dir != "." && err != nil {
		dir = filepath.Dir(dir)
		repo, err = git.PlainOpen(dir)
	}
	if err != nil {
		if s.branch == "" {
			return 0, 0
		}
	}

	ref, _ := repo.Head()
	tree, _ := repo.Worktree()
	status, _ := tree.Status()
	branch := ref.Name().Short()
	if !status.IsClean() {
		branch += "*"
	}

	if s.branch != branch {
		s.branch = branch
		s.text = branch
		w, h = s.getSize()
		s.setSize(w, h)
		ui.QueueMain(func() {
			s.area.QueueRedrawAll()
		})
	}
	return w, h
}

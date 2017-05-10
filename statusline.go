package gonvim

import "github.com/dzhou121/ui"

// Statusline is
type Statusline struct {
	AreaHandler
	box *ui.Box
	bg  *RGBA

	file     string
	filetype string
	encoding string
	pos      *StatuslinePos
}

// StatuslineFile is
type StatuslineFile struct {
}

// StatuslinePos is
type StatuslinePos struct {
	SpanHandler
	ln  int
	col int
}

func initStatusline(width, height int) *Statusline {
	box := ui.NewHorizontalBox()
	box.SetSize(width, height)

	statusline := &Statusline{
		box: box,
		bg:  newRGBA(24, 29, 34, 1),
	}

	area := ui.NewArea(statusline)
	statusline.area = area
	statusline.setSize(width, height)
	statusline.borderTop = &Border{
		width: 2,
		color: newRGBA(0, 0, 0, 1),
	}
	box.Append(area, false)

	pos := &StatuslinePos{}
	pos.area = ui.NewArea(pos)
	pos.text = "Ln 128, Col 119"
	pos.setSize(10, 10)
	box.Append(pos.area, false)
	statusline.pos = pos

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
	ui.QueueMain(func() {
		s.pos.area.QueueRedrawAll()
	})
}

package gonvim

import "github.com/dzhou121/ui"

// CursorHandler is the cursor
type CursorHandler struct {
	AreaHandler
	bg *RGBA
}

// Draw the cursor
func (c *CursorHandler) Draw(a *ui.Area, dp *ui.AreaDrawParams) {
	p := ui.NewPath(ui.Winding)
	p.AddRectangle(dp.ClipX, dp.ClipY, dp.ClipWidth, dp.ClipHeight)
	p.End()
	dp.Context.Fill(p, &ui.Brush{
		Type: ui.Solid,
		R:    c.bg.R,
		G:    c.bg.G,
		B:    c.bg.B,
		A:    c.bg.A,
	})
	p.Free()
}

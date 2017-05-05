package gonvim

import (
	"github.com/dzhou121/ui"
)

// CursorBox is
type CursorBox struct {
	box       *ui.Box
	cursor    *CursorHandler
	locpopup  *Locpopup
	signature *Signature
}

// CursorHandler is the cursor
type CursorHandler struct {
	AreaHandler
	bg *RGBA
}

func initCursorBox(width, height int) *CursorBox {
	box := ui.NewHorizontalBox()
	box.SetSize(width, height)

	cursor := &CursorHandler{}
	cursorArea := ui.NewArea(cursor)
	cursor.area = cursorArea

	loc := initLocpopup()
	sig := initSignature()

	box.Append(cursorArea, false)
	box.Append(loc.box, false)
	box.Append(sig.box, false)

	ui.QueueMain(func() {
		cursorArea.Show()
		box.Show()
	})

	return &CursorBox{
		box:       box,
		cursor:    cursor,
		locpopup:  loc,
		signature: sig,
	}
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

func (c *CursorBox) draw() {
	row := editor.screen.cursor[0]
	col := editor.screen.cursor[1]
	ui.QueueMain(func() {
		c.cursor.area.SetPosition(col*editor.font.width, row*editor.font.lineHeight)
	})
	c.locpopup.move()

	mode := editor.mode
	if mode == "normal" {
		ui.QueueMain(func() {
			c.cursor.area.SetSize(editor.font.width, editor.font.lineHeight)
			c.cursor.bg = newRGBA(255, 255, 255, 0.5)
			c.cursor.area.QueueRedrawAll()
		})
	} else if mode == "insert" {
		ui.QueueMain(func() {
			c.cursor.area.SetSize(1, editor.font.lineHeight)
			c.cursor.bg = newRGBA(255, 255, 255, 0.9)
			c.cursor.area.QueueRedrawAll()
		})
	}
}

func (c *CursorBox) setSize(width, height int) {
	c.box.SetSize(width, height)
}

package gonvim

import (
	"github.com/therecipe/qt/widgets"
)

// CursorBox is
// type CursorBox struct {
// 	box       *ui.Box
// 	cursor    *CursorHandler
// 	locpopup  *Locpopup
// 	signature *Signature
// }

// CursorHandler is the cursor
// type CursorHandler struct {
// 	AreaHandler
// 	bg *RGBA
// }

type Cursor struct {
	widget *widgets.QWidget
}

func initCursorNew() *Cursor {
	widget := widgets.NewQWidget(nil, 0)
	return &Cursor{
		widget: widget,
	}
}

func (c *Cursor) update() {
	row := editor.screen.cursor[0]
	col := editor.screen.cursor[1]
	mode := editor.mode
	if mode == "normal" {
		c.widget.Resize2(editor.font.width, editor.font.lineHeight)
		c.widget.SetStyleSheet("background-color: rgba(255, 255, 255, 0.5)")
	} else if mode == "insert" {
		c.widget.Resize2(1, editor.font.lineHeight)
		c.widget.SetStyleSheet("background-color: rgba(255, 255, 255, 0.9)")
	}
	c.widget.Move2(int(float64(col)*editor.font.truewidth), row*editor.font.lineHeight)
}

// func initCursorBox(width, height int) *CursorBox {
// 	box := ui.NewHorizontalBox()
// 	box.SetSize(width, height)

// 	cursor := &CursorHandler{}
// 	cursorArea := ui.NewArea(cursor)
// 	cursor.area = cursorArea

// 	loc := initLocpopup()
// 	sig := initSignature()

// 	box.Append(cursorArea, false)
// 	box.Append(loc.box, false)
// 	box.Append(sig.box, false)

// 	ui.QueueMain(func() {
// 		cursorArea.Show()
// 		box.Show()
// 	})

// 	return &CursorBox{
// 		box:       box,
// 		cursor:    cursor,
// 		locpopup:  loc,
// 		signature: sig,
// 	}
// }

// Draw the cursor
// func (c *CursorHandler) Draw(a *ui.Area, dp *ui.AreaDrawParams) {
// }

// func (c *CursorBox) draw() {
// row := editor.screen.cursor[0]
// col := editor.screen.cursor[1]
// ui.QueueMain(func() {
// 	c.cursor.area.SetPosition(int(float64(col)*editor.font.truewidth), row*editor.font.lineHeight)
// })
// c.locpopup.move()

// cursorBg := newRGBA(255, 255, 255, 1)

// mode := editor.mode
// if mode == "normal" {
// 	ui.QueueMain(func() {
// 		cursorBg.A = 0.5
// 		c.cursor.bg = cursorBg
// 		c.cursor.area.SetSize(editor.font.width, editor.font.lineHeight)
// 		c.cursor.area.QueueRedrawAll()
// 	})
// } else if mode == "insert" {
// 	ui.QueueMain(func() {
// 		cursorBg.A = 0.9
// 		c.cursor.bg = cursorBg
// 		c.cursor.area.SetSize(1, editor.font.lineHeight)
// 		c.cursor.area.QueueRedrawAll()
// 	})
// }
// }

// func (c *CursorBox) setSize(width, height int) {
// 	c.box.SetSize(width, height)
// }

package editor

import (
	"github.com/therecipe/qt/widgets"
	"github.com/therecipe/qt/core"
)

// Cursor is
type Cursor struct {
	widget *widgets.QWidget
	mode   string
	x      int
	y      int
	row    int
	col    int
}

func initCursorNew() *Cursor {
	widget := widgets.NewQWidget(nil, 0)
	cursor := &Cursor{
		widget: widget,
	}
	return cursor
}

func (c *Cursor) move() {
	c.widget.Move2(c.x, c.y)
	editor.loc.widget.Move2(c.x, c.y+editor.font.lineHeight)
}

func (c *Cursor) updateShape() {
	mode := editor.mode
	if mode == "normal" {
		c.widget.Resize2(editor.font.width, editor.font.lineHeight)
		c.widget.SetStyleSheet("background-color: rgba(255, 255, 255, 0.5)")
	} else if mode == "insert" {
		c.widget.Resize2(1, editor.font.lineHeight)
		c.widget.SetStyleSheet("background-color: rgba(255, 255, 255, 0.9)")
	}
}

func (c *Cursor) update() {
	if c.mode != editor.mode {
		c.mode = editor.mode
		c.updateShape()
	}
	row := editor.screen.cursor[0]
	col := editor.screen.cursor[1]
	if c.row != row || c.col != col {
		c.x = int(float64(col) * editor.font.truewidth)
		c.y = row * editor.font.lineHeight
		c.move()
	}
	tooltip.Move(core.NewQPoint2(c.x, c.y))
}

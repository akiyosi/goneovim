package editor

import (
	"fmt"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/widgets"
)

// Cursor is
type Cursor struct {
	ws     *Workspace
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
	c.ws.loc.widget.Move2(c.x, c.y+c.ws.font.lineHeight)
}

func (c *Cursor) updateShape() {
	mode := c.ws.mode
	bg := c.ws.background
	if mode == "normal" {
		c.widget.Resize2(c.ws.font.width, c.ws.font.lineHeight)
		//c.widget.SetStyleSheet("background-color: rgba(255, 255, 255, 0.5)")
		c.widget.SetStyleSheet(fmt.Sprintf("background-color: rgba(%d, %d, %d, 0.5)", reverseColor(bg).R, reverseColor(bg).G, reverseColor(bg).B))
	} else if mode == "insert" {
		c.widget.Resize2(1, c.ws.font.lineHeight)
		//c.widget.SetStyleSheet("background-color: rgba(255, 255, 255, 0.9)")
		c.widget.SetStyleSheet(fmt.Sprintf("background-color: rgba(%d, %d, %d, 0.9)", reverseColor(bg).R, reverseColor(bg).G, reverseColor(bg).B))
	}
}

func (c *Cursor) update() {
	if c.mode != c.ws.mode {
		c.mode = c.ws.mode
		c.updateShape()
	}
	row := c.ws.screen.cursor[0]
	col := c.ws.screen.cursor[1]
	if c.row != row || c.col != col {
		c.x = int(float64(col) * c.ws.font.truewidth)
		c.y = row * c.ws.font.lineHeight
		c.move()
	}
	c.ws.screen.tooltip.Move(core.NewQPoint2(c.x, c.y))
}

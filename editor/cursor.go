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
	isShut bool
}

func initCursorNew() *Cursor {
	widget := widgets.NewQWidget(nil, 0)
	cursor := &Cursor{
		widget: widget,
	}

	if editor.config.Editor.CursorBlink {
		timer := core.NewQTimer(nil)
		timer.ConnectTimeout(cursor.blink)
		timer.Start(1000)
		timer.SetInterval(500)
	}

	return cursor
}

func (c *Cursor) blink() {
	bg := c.ws.background
	if c.isShut {
		c.widget.SetStyleSheet(fmt.Sprintf("background-color: rgba(%d, %d, %d, 0.1)", reverseColor(bg).R, reverseColor(bg).G, reverseColor(bg).B))
		c.isShut = false
	} else {
		switch c.ws.mode {
		case "visual":
			visualColor := hexToRGBA(editor.config.SideBar.AccentColor)
			c.widget.SetStyleSheet(fmt.Sprintf("background-color: rgba(%d, %d, %d, 0.5)", visualColor.R, visualColor.G, visualColor.B))
		default:
			c.widget.SetStyleSheet(fmt.Sprintf("background-color: rgba(%d, %d, %d, 0.5)", reverseColor(bg).R, reverseColor(bg).G, reverseColor(bg).B))
		}
		c.isShut = true
	}
	c.widget.Hide()
	c.widget.Show()
}

func (c *Cursor) move() {
	c.widget.Move2(c.x, c.y+int(float64(c.ws.font.lineSpace)/2))
	c.ws.loc.widget.Move2(c.x, c.y+c.ws.font.lineHeight)
}

func (c *Cursor) updateShape() {
	mode := c.ws.mode
	bg := c.ws.background
	switch mode {
	case "normal":
		c.widget.Resize2(c.ws.font.width, c.ws.font.height+2)
		//c.widget.SetStyleSheet("background-color: rgba(255, 255, 255, 0.5)")
		c.widget.SetStyleSheet(fmt.Sprintf("background-color: rgba(%d, %d, %d, 0.5)", reverseColor(bg).R, reverseColor(bg).G, reverseColor(bg).B))
	case "insert":
		c.widget.Resize2(2, c.ws.font.height+2)
		//c.widget.SetStyleSheet("background-color: rgba(255, 255, 255, 0.9)")
		c.widget.SetStyleSheet(fmt.Sprintf("background-color: rgba(%d, %d, %d, 0.9)", reverseColor(bg).R, reverseColor(bg).G, reverseColor(bg).B))
	case "visual":
		c.widget.Resize2(c.ws.font.width, c.ws.font.height+2)
		//c.widget.SetStyleSheet("background-color: rgba(255, 255, 255, 0.9)")
		visualColor := hexToRGBA(editor.config.SideBar.AccentColor)
		c.widget.SetStyleSheet(fmt.Sprintf("background-color: rgba(%d, %d, %d, 0.5)", visualColor.R, visualColor.G, visualColor.B))
	}
}

func (c *Cursor) update() {
	if c.mode != c.ws.mode {
		c.mode = c.ws.mode
		c.updateShape()
	}
	row := c.ws.screen.cursor[0]
	col := c.ws.screen.cursor[1]
	if c.x != row || c.y != col {
		c.x = int(float64(col) * c.ws.font.truewidth)
		c.y = row * c.ws.font.lineHeight
		c.move()
	}
	c.ws.screen.tooltip.Move(core.NewQPoint2(c.x, c.y))
}

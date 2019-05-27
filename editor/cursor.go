package editor

import (
	"fmt"

	"github.com/akiyosi/gonvim/util"
	"github.com/therecipe/qt/widgets"
)

// Cursor is
type Cursor struct {
	ws        *Workspace
	widget    *widgets.QWidget
	mode      string
	modeIdx   int
	x         int
	y         int
	isShut    bool
	color     *RGBA
}

func initCursorNew() *Cursor {
	widget := widgets.NewQWidget(nil, 0)
	cursor := &Cursor{
		widget: widget,
	}

	// if editor.config.Editor.CursorBlink {
	// 	timer := core.NewQTimer(nil)
	// 	timer.ConnectTimeout(cursor.blink)
	// 	timer.Start(1000)
	// 	timer.SetInterval(500)
	// }

	return cursor
}

func (c *Cursor) blink() {
	if c.color == nil {
		c.color = invertColor(c.ws.background)
	}
	if c.isShut {
		c.widget.SetStyleSheet(fmt.Sprintf("background-color: rgba(%d, %d, %d, 0.1)", c.color.R, c.color.G, c.color.B))
		c.isShut = false
	} else {
		switch c.ws.mode {
		case "visual":
			visualColor := hexToRGBA(editor.config.SideBar.AccentColor)
			c.widget.SetStyleSheet(fmt.Sprintf("background-color: rgba(%d, %d, %d, 0.5)", visualColor.R, visualColor.G, visualColor.B))
		default:
			c.widget.SetStyleSheet(fmt.Sprintf("background-color: rgba(%d, %d, %d, 0.7)", c.color.R, c.color.G, c.color.B))
		}
		c.isShut = true
	}
	c.widget.Hide()
	c.widget.Show()
}

func (c *Cursor) move() {
	c.widget.Move2(c.x, c.y+int(float64(c.ws.font.lineSpace)/2))
	c.ws.loc.widget.Move2(c.x, c.y+c.ws.font.lineHeight)
	// c.updateColor()
	// c.updateShape()
}

func (c *Cursor) updateColor() {
	// 	screen := c.ws.screen
	// 	row := screen.cursor[0]
	// 	col := screen.cursor[1]
	// 	// s := screen.colorContent
	// 	if screen.activeGrid == 1 {
	// 		return
	// 	}
	// 	s := screen.windows[screen.activeGrid].colorContent
	// 	if s == nil {
	// 		return
	// 	}
	// 	if len(s) <= row {
	// 		return
	// 	}
	// 	for _, line := range s {
	// 		if len(line) <= col+1 {
	// 			return
	// 		}
	// 	}
	// 	color := s[row][col+1] // FIXME: out of index
	// 	if color != nil && !c.color.equals(color) {
	// 		c.color = invertColor(color)
	// 	}
	// 	if c.color == nil {
	// c.color = invertColor(c.ws.background)
	// 	}
}

func (c *Cursor) updateShape() {
	mode := c.ws.mode
	c.updateColor()
	switch mode {
	case "normal":
		c.widget.Resize2(c.ws.font.width, c.ws.font.height+2)
		//c.widget.SetStyleSheet("background-color: rgba(255, 255, 255, 0.5)")
		c.widget.SetStyleSheet(fmt.Sprintf("background-color: rgba(%d, %d, %d, 0.7)", c.color.R, c.color.G, c.color.B))
	case "insert":
		c.widget.Resize2(2, c.ws.font.height+2)
		//c.widget.SetStyleSheet("background-color: rgba(255, 255, 255, 0.9)")
		c.widget.SetStyleSheet(fmt.Sprintf("background-color: rgba(%d, %d, %d, 0.9)", c.color.R, c.color.G, c.color.B))
	case "visual":
		c.widget.Resize2(c.ws.font.width, c.ws.font.height+2)
		//c.widget.SetStyleSheet("background-color: rgba(255, 255, 255, 0.9)")
		visualColor := hexToRGBA(editor.config.SideBar.AccentColor)
		c.widget.SetStyleSheet(fmt.Sprintf("background-color: rgba(%d, %d, %d, 0.5)", visualColor.R, visualColor.G, visualColor.B))
	}
}

func (c *Cursor) updateCursorShape() {
	if !c.ws.cursorStyleEnabled {
		return
	}
	cursorShape := "block"
	cursorShapeITF, ok := c.ws.modeInfo[c.modeIdx]["cursor_shape"]
	if ok {
		cursorShape = cursorShapeITF.(string)
	}
	cellPercentage := 100
	cellPercentageITF, ok := c.ws.modeInfo[c.modeIdx]["cell_percentage"]
	if ok {
		cellPercentage = util.ReflectToInt(cellPercentageITF)
	}

	height := c.ws.font.height+2
	width := c.ws.font.width
	switch cursorShape {
	case "block":
	case "horizontal":
		height = int(float64(height) * float64(cellPercentage) / float64(100))
	case "vertical":
		width = int(float64(width) * float64(cellPercentage) / float64(100))
	default:
	}

	attrId := 0
	attrIdITF, ok := c.ws.modeInfo[c.modeIdx]["attr_id"]
	if ok {
		attrId = util.ReflectToInt(attrIdITF)
	}
	bgcolor := c.ws.screen.highAttrDef[attrId].foreground

	c.widget.Resize2(width, height)
	c.widget.SetStyleSheet(fmt.Sprintf("background-color: rgba(%d, %d, %d, 0.7)", bgcolor.R, bgcolor.G, bgcolor.B))
}

func (c *Cursor) update2() {
	if c.modeIdx != c.ws.modeIdx {
		c.modeIdx = c.ws.modeIdx
		c.updateCursorShape()
	}
	row := c.ws.screen.cursor[0]
	col := c.ws.screen.cursor[1]
	if c.x != row || c.y != col {
		c.x = int(float64(col) * c.ws.font.truewidth)
		c.y = row * c.ws.font.lineHeight
		c.move()
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
}

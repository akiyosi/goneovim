package editor

import (
	"fmt"

	"github.com/akiyosi/gonvim/util"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/widgets"
)

// Cursor is
type Cursor struct {
	ws *Workspace
	//widget         *widgets.QWidget
	widget         *widgets.QLabel
	mode           string
	modeIdx        int
	x              int
	y              int
	currAttrId     int
	defaultColorId int
	gridid         int
	shift          int
	isShut         bool
	timer          *core.QTimer
	color          *RGBA
	isTextDraw     bool
}

func initCursorNew() *Cursor {
	// widget := widgets.NewQWidget(nil, 0)
	widget := widgets.NewQLabel(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	cursor := &Cursor{
		widget: widget,
		timer:  core.NewQTimer(nil),
	}

	return cursor
}

func (c *Cursor) setBlink(wait, on, off int) {
	bg := c.ws.screen.highAttrDef[c.currAttrId].background
	fg := c.ws.screen.highAttrDef[c.currAttrId].foreground
	c.timer.DisconnectTimeout()
	if wait == 0 || on == 0 || off == 0 {
		c.widget.SetStyleSheet(fmt.Sprintf(
			"background-color: rgba(%d, %d, %d, 0.8); color: rgba(%d, %d, %d, 1);",
			bg.R,
			bg.G,
			bg.B,
			fg.R,
			fg.G,
			fg.B,
		))
		return
	}
	c.timer.ConnectTimeout(func() {
		alpha := 0.8
		if !c.isShut {
			c.timer.SetInterval(off)
			c.isShut = true
			alpha = 0.1
		} else {
			c.timer.SetInterval(on)
			c.isShut = false
		}
		c.widget.SetStyleSheet(fmt.Sprintf(
			"background-color: rgba(%d, %d, %d, %f); color: rgba(%d, %d, %d, %f);",
			bg.R,
			bg.G,
			bg.B,
			alpha,
			fg.R,
			fg.G,
			fg.B,
			alpha,
		))
	})
	c.timer.Start(wait)
	c.timer.SetInterval(off)
}

func (c *Cursor) move() {
	c.widget.Move2(c.x, c.y+int(float64(c.ws.font.lineSpace)/2))
	c.ws.loc.widget.Move2(c.x, c.y+c.ws.font.lineHeight)
}

func (c *Cursor) updateCursorShape2() {
	mode := c.ws.mode
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

	height := c.ws.font.height + 2
	width := c.ws.font.width
	p := float64(cellPercentage) / float64(100)

	switch cursorShape {
	case "horizontal":
		height = int(float64(height) * p)
		c.shift = int(float64(c.ws.font.lineHeight) * float64(1.0-p))
		if cellPercentage < 99 {
			c.isTextDraw = false
		} else {
			c.isTextDraw = true
		}
	case "vertical":
		c.isTextDraw = true
		width = int(float64(width) * p)
		c.shift = 0
	default:
		c.isTextDraw = true
		c.shift = 0
	}

	attrId := 0
	attrIdITF, ok := c.ws.modeInfo[c.modeIdx]["attr_id"]
	if ok {
		attrId = util.ReflectToInt(attrIdITF)
		c.currAttrId = attrId
	}

	var bg, fg *RGBA
	if attrId != 0 {
		bg = c.ws.screen.highAttrDef[attrId].background
		fg = c.ws.screen.highAttrDef[attrId].foreground
	} else {
		bg = c.ws.screen.highAttrDef[c.defaultColorId].background
		fg = c.ws.screen.highAttrDef[c.defaultColorId].foreground
	}
	if bg == nil {
		return
	}

	var blinkWait, blinkOn, blinkOff int
	blinkWaitITF, ok := c.ws.modeInfo[c.modeIdx]["blinkwait"]
	if ok {
		blinkWait = util.ReflectToInt(blinkWaitITF)
	}
	blinkOnITF, ok := c.ws.modeInfo[c.modeIdx]["blinkon"]
	if ok {
		blinkOn = util.ReflectToInt(blinkOnITF)
	}
	blinkOffITF, ok := c.ws.modeInfo[c.modeIdx]["blinkoff"]
	if ok {
		blinkOff = util.ReflectToInt(blinkOffITF)
	}
	c.setBlink(blinkWait, blinkOn, blinkOff)

	c.widget.Resize2(width, height)
	c.widget.SetStyleSheet(fmt.Sprintf(
		"background-color: rgba(%d, %d, %d, 0.8); color: rgba(%d, %d, %d, 1.0)",
		bg.R,
		bg.G,
		bg.B,
		fg.R,
		fg.G,
		fg.B,
	))
}

func (c *Cursor) update() {
	c.updateCursorShape()
	row := c.ws.screen.cursor[0]
	col := c.ws.screen.cursor[1]
	x := int(float64(col) * c.ws.font.truewidth)
	y := row*c.ws.font.lineHeight + c.shift
	c.x = x
	c.y = y
	c.move()
	c.paint()

}

func (c *Cursor) paint() {
	if !c.isTextDraw {
		return
	}

	win := c.ws.screen.windows[c.gridid]
	if win == nil ||
		win.content == nil {
		return
	}

	text := ""
	y := c.ws.screen.cursor[1]
	x := c.ws.screen.cursor[0]

	if x >= len(win.content) ||
		y >= len(win.content[0]) ||
		win.content[x][y] == nil ||
		win.content[x][y].char == "" ||
		c.ws.palette.widget.IsVisible() {
	} else {
		text = win.content[x][y].char
	}

	c.widget.SetText(text)
	return
}

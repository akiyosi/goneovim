package editor

import (
	"fmt"

	"github.com/akiyosi/goneovim/util"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/widgets"
)

// Cursor is
type Cursor struct {
	ws *Workspace
	//widget         *widgets.QWidget
	widget     *widgets.QLabel
	mode       string
	modeIdx    int
	x          int
	y          int
	currAttrId int
	gridid     int
	shift      int
	isShut     bool
	timer      *core.QTimer
	color      *RGBA
	isTextDraw bool
	fg         *RGBA
	bg         *RGBA
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
	bg := c.bg
	fg := c.fg
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
	win, ok := c.ws.screen.windows[c.gridid]
	if !ok {
		return
	}
	if win == nil {
		return
	}
	font := win.getFont()

	shift := 0
	if editor.config.Editor.CachedDrawing {
		c.widget.Move2(c.x, c.y)
	} else {
		shift = int(float64(font.lineSpace) / 2)
		c.widget.Move2(c.x, c.y+shift)
	}

	if !c.ws.loc.shown {
		return
	}

	col := c.ws.screen.cursor[1]
	row := c.ws.screen.cursor[0]
	x, y := c.ws.getPointInWidget(col, row, c.gridid)

	if row < 3 {
		y += c.ws.loc.widget.Height()
	} else {
		y -= c.ws.loc.widget.Height()
	}

	c.ws.loc.widget.Move2(x, y)
}

func (c *Cursor) updateFont(font *Font) {
	c.widget.SetFont(font.fontNew)
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

	win, ok := c.ws.screen.windows[c.gridid]
	if !ok {
		return
	}
	if win == nil {
		return
	}
	var font *Font
	if c.ws.palette.widget.IsVisible() {
		font = c.ws.font
	} else {
		font = win.getFont()
	}

	height := font.height + 2
	width := font.width
	p := float64(cellPercentage) / float64(100)

	switch cursorShape {
	case "horizontal":
		height = int(float64(height) * p)
		c.shift = int(float64(font.lineHeight) * float64(1.0-p))
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
		fg = c.ws.background
		bg = c.ws.foreground
	}
	c.fg = fg
	c.bg = bg
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
	if c.mode != c.ws.mode {
		c.mode = c.ws.mode
		if c.mode == "terminal-input" {
			c.widget.Hide()
			return
		} else {
			c.widget.Show()
		}
	}
	c.updateCursorShape()
	if c.ws.palette.widget.IsVisible() {
		return
	}
	win, ok := c.ws.screen.windows[c.gridid]
	if !ok {
		return
	}
	if win == nil {
		return
	}
	font := win.getFont()

	row := c.ws.screen.cursor[0]
	col := c.ws.screen.cursor[1]
	x := int(float64(col) * font.truewidth)
	y := row*font.lineHeight + c.shift
	c.x = x
	c.y = y
	c.move()
	c.paint()

}

func (c *Cursor) paint() {
	if !c.isTextDraw {
		c.widget.SetText("")
		return
	}

	win, ok := c.ws.screen.windows[c.gridid]
	if !ok {
		return
	}
	if win == nil {
		return
	}
	if win.content == nil {
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
}

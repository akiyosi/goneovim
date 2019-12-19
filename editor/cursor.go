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
	gridid     int
	shift      int
	isShut     bool
	timer      *core.QTimer
	color      *RGBA
	isTextDraw bool
	fg         *RGBA
	bg         *RGBA
	font       *Font

	isNeedUpdateModeInfo bool
	modeInfoModeIdx      int
	cursorShape          string
	cellPercentage       int
	currAttrId           int
	blinkWait            int
	blinkOn              int
	blinkOff             int
}

func initCursorNew() *Cursor {
	// widget := widgets.NewQWidget(nil, 0)
	widget := widgets.NewQLabel(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	cursor := &Cursor{
		widget:               widget,
		timer:                core.NewQTimer(nil),
		isNeedUpdateModeInfo: true,
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
			alpha = 0.4
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
	// win, ok := c.ws.screen.windows[c.gridid]
	// if !ok {
	// 	return
	// }
	// if win == nil {
	// 	return
	// }
	win, ok := c.ws.screen.getWindow(c.gridid)
	if !ok {
		return
	}
	font := win.getFont()

	shift := 0
	shift = int(float64(font.lineSpace) / 2)
	c.widget.Move2(c.x, c.y+shift)

	c.ws.loc.updatePos()
}

func (c *Cursor) updateFont(font *Font) {
	c.widget.SetFont(font.fontNew)
}

func (c *Cursor) updateCursorShape() {
	if !c.ws.cursorStyleEnabled {
		return
	}

	var isUpdateStyle bool
	if c.modeInfoModeIdx != c.modeIdx || c.isNeedUpdateModeInfo {
		c.modeInfoModeIdx = c.modeIdx
		modeInfo := c.ws.modeInfo[c.modeIdx]

		c.currAttrId = 0
		attrIdITF, ok := modeInfo["attr_id"]
		if ok {
			c.currAttrId = util.ReflectToInt(attrIdITF)
		}
		fg := c.ws.screen.highAttrDef[c.currAttrId].fg()
		bg := c.ws.screen.highAttrDef[c.currAttrId].bg()
		isUpdateStyle = c.fg != fg || c.bg != bg
		c.fg = fg
		c.bg = bg
		if c.bg == nil {
			return
		}

		c.cursorShape = "block"
		cursorShapeITF, ok := modeInfo["cursor_shape"]
		if ok {
			c.cursorShape = cursorShapeITF.(string)
		}
		c.cellPercentage = 100
		cellPercentageITF, ok := modeInfo["cell_percentage"]
		if ok {
			c.cellPercentage = util.ReflectToInt(cellPercentageITF)
		}

		blinkWaitITF, ok := modeInfo["blinkwait"]
		if ok {
			c.blinkWait = util.ReflectToInt(blinkWaitITF)
		}
		blinkOnITF, ok := modeInfo["blinkon"]
		if ok {
			c.blinkOn = util.ReflectToInt(blinkOnITF)
		}
		blinkOffITF, ok := modeInfo["blinkoff"]
		if ok {
			c.blinkOff = util.ReflectToInt(blinkOffITF)
		}
		c.setBlink(c.blinkWait, c.blinkOn, c.blinkOff)

		c.isNeedUpdateModeInfo = false
	}


	height := c.font.height + 2
	width := c.font.width
	p := float64(c.cellPercentage) / float64(100)

	switch c.cursorShape {
	case "horizontal":
		height = int(float64(height) * p)
		c.shift = int(float64(c.font.lineHeight) * (1.0 - p))
		if c.cellPercentage < 99 {
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

	if width == 0 {
		width = 1
	}
	if height == 0 {
		height = 1
	}

	c.widget.Resize2(width, height)
	c.timer.StartDefault(0)
	if !isUpdateStyle {
		return
	}
	c.widget.SetStyleSheet(
		fmt.Sprintf(` QLabel {
		background-color: rgba(%d, %d, %d, 0.8);
		color: rgba(%d, %d, %d, 1.0)
		}`,
		c.bg.R,
		c.bg.G,
		c.bg.B,
		c.fg.R,
		c.fg.G,
		c.fg.B,
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
	// win, ok := c.ws.screen.windows[c.gridid]
	// if !ok {
	// 	return
	// }
	// if win == nil {
	// 	return
	// }
	win, ok := c.ws.screen.getWindow(c.gridid)
	if !ok {
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

	// win, ok := c.ws.screen.windows[c.gridid]
	// if !ok {
	// 	return
	// }
	// if win == nil {
	// 	return
	// }
	win, ok := c.ws.screen.getWindow(c.gridid)
	if !ok {
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

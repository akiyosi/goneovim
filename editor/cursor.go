package editor

import (
	"math"
	"runtime"

	"github.com/akiyosi/goneovim/util"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/widgets"
)

// Cursor is
type Cursor struct {
	ws               *Workspace
	widget           *widgets.QWidget
	mode             string
	modeIdx          int
	x                int
	y                int
	width            int
	height           int
	text             string
	normalWidth      bool
	gridid           int
	bufferGridid     int
	shift            int
	isShut           bool
	timer            *core.QTimer
	isTextDraw       bool
	fg               *RGBA
	bg               *RGBA
	brend            float64
	font             *Font
	fontwide         *Font
	isInPalette      bool
	charCache        *Cache
	devicePixelRatio float64

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
	widget := widgets.NewQWidget(nil, 0)
	// widget := widgets.NewQLabel(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	c := &Cursor{
		widget:               widget,
		timer:                core.NewQTimer(nil),
		isNeedUpdateModeInfo: true,
	}
	widget.ConnectPaintEvent(c.paint)

	return c
}

func (c *Cursor) paint(event *gui.QPaintEvent) {
	font := c.font
	if font == nil {
		return
	}
	if c.charCache == nil {
		return
	}
	if c.widget == nil {
		return
	}

	// If guifontwide is set
	shift := font.ascent

	width := font.truewidth
	if !c.normalWidth {
		width = width * 2
	}

	p := gui.NewQPainter2(c.widget)

	// Draw cursor background
	p.FillRect4(
		core.NewQRectF4(
			0,
			0,
			width,
			float64(font.lineHeight),
		),
		c.bg.brend(c.ws.background, c.brend).QColor(),
	)

	if c.text == "" || c.devicePixelRatio == 0 {
		p.DestroyQPainter()
		return
	}

	// Draw cursor foreground
	if editor.config.Editor.CachedDrawing {
		var image *gui.QImage
		charCache := *c.charCache
		imagev, err := charCache.get(HlChars{
			text:   c.text,
			fg:     c.fg,
			italic: false,
			bold:   false,
		})
		if err != nil {
			image = c.newCharCache(c.text, c.fg, c.normalWidth)
			c.setCharCache(c.text, c.fg, image)
		} else {
			image = imagev.(*gui.QImage)
		}
		p.DrawImage7(
			core.NewQPointF3(
				0,
				0,
			),
			image,
		)
	} else {
		if !c.normalWidth && c.fontwide != nil {
			p.SetFont(c.fontwide.fontNew)
			if c.fontwide.lineHeight > font.lineHeight {
				shift = shift + c.fontwide.ascent - font.ascent
			}
		} else {
			p.SetFont(font.fontNew)
		}
		p.SetPen2(c.fg.QColor())
		p.DrawText(
			core.NewQPointF3(
				0,
				shift,
			),
			c.text,
		)
	}

	p.DestroyQPainter()
}

func (c *Cursor) newCharCache(text string, fg *RGBA, isNormalWidth bool) *gui.QImage {
	font := c.font

	width := float64(len(text)) * font.italicWidth
	if !isNormalWidth {
		width = math.Ceil(c.ws.screen.runeTextWidth(font, text))
	}

	// QImage default device pixel ratio is 1.0,
	// So we set the correct device pixel ratio
	image := gui.NewQImage2(
		core.NewQRectF4(
			0,
			0,
			c.devicePixelRatio*width,
			c.devicePixelRatio*float64(font.height),
		).Size().ToSize(),
		gui.QImage__Format_ARGB32_Premultiplied,
	)
	image.SetDevicePixelRatio(c.devicePixelRatio)
	image.Fill3(core.Qt__transparent)

	pi := gui.NewQPainter2(image)
	pi.SetPen2(fg.QColor())

	if !isNormalWidth && font == nil && c.ws.fontwide != nil {
		pi.SetFont(c.ws.fontwide.fontNew)
	} else {
		pi.SetFont(font.fontNew)
	}

	// TODO
	// Set bold, italic styles

	pi.DrawText6(
		core.NewQRectF4(
			0,
			0,
			width,
			float64(font.height),
		), text, gui.NewQTextOption2(core.Qt__AlignVCenter),
	)

	pi.DestroyQPainter()

	return image
}

func (c *Cursor) setCharCache(text string, fg *RGBA, image *gui.QImage) {
	c.charCache.set(
		HlChars{
			text:   text,
			fg:     c.fg,
			italic: false,
			bold:   false,
		},
		image,
	)
}

func (c *Cursor) setBlink() {
	c.timer.DisconnectTimeout()

	wait := c.blinkWait
	on := c.blinkOn
	off := c.blinkOff
	if wait == 0 || on == 0 || off == 0 {
		c.brend = 0.0
		c.widget.Update()
		return
	}
	c.timer.ConnectTimeout(func() {
		c.brend = 0.0
		if !c.isShut {
			c.timer.SetInterval(off)
			c.isShut = true
			c.brend = 0.6
		} else {
			c.timer.SetInterval(on)
			c.isShut = false
		}
		c.widget.Update()
	})
	c.timer.Start(wait)
	c.timer.SetInterval(off)
}

func (c *Cursor) move() {
	c.widget.Move(
		core.NewQPoint2(
			c.x,
			c.y,
		),
	)

	// if c.ws.loc != nil {
	// 	c.ws.loc.updatePos()
	// }

	// Fix #119: Wrong candidate window position when using ibus
	if runtime.GOOS == "linux" {
		gui.QGuiApplication_InputMethod().Update(core.Qt__ImCursorRectangle)
	}
}

func (c *Cursor) updateFont(font *Font) {
	c.font = font
}

func (c *Cursor) updateCursorShape() {
	if !c.ws.cursorStyleEnabled {
		return
	}

	if c.modeInfoModeIdx != c.modeIdx || c.isNeedUpdateModeInfo {
		c.modeInfoModeIdx = c.modeIdx

		modeInfo := c.ws.modeInfo[c.modeIdx]
		attrIdITF, ok := modeInfo["attr_id"]
		if ok {
			c.currAttrId = util.ReflectToInt(attrIdITF)
		}
		var bg, fg *RGBA
		if c.currAttrId == 0 {
			fg = c.ws.screen.hlAttrDef[0].background
			bg = c.ws.screen.hlAttrDef[0].foreground
		} else {
			fg = c.ws.screen.hlAttrDef[c.currAttrId].fg()
			bg = c.ws.screen.hlAttrDef[c.currAttrId].bg()
		}
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

		c.setBlink()

		c.isNeedUpdateModeInfo = false
	}

	height := c.font.height
	width := int(math.Trunc(c.font.truewidth))
	if !c.normalWidth {
		width = width * 2
	}
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

	if c.blinkWait != 0 {
		c.brend = 0.0
		c.timer.Start(c.blinkWait)
	}

	if !(c.width == width && c.height == height) {
		c.width = width
		c.height = height
		c.widget.Resize2(width, height)
	}

	c.widget.Update()
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

	win, ok := c.ws.screen.getWindow(c.gridid)
	if !ok {
		return
	}
	charCache := win.getCache()
	c.charCache = &charCache
	c.devicePixelRatio = win.devicePixelRatio

	row := c.ws.screen.cursor[0]
	col := c.ws.screen.cursor[1]

	if row >= len(win.content) ||
		col >= len(win.content[0]) ||
		win.content[row][col] == nil ||
		win.content[row][col].char == "" {
		// c.ws.palette.widget.IsVisible() {
		c.text = ""
		c.normalWidth = true
	} else {
		c.text = win.content[row][col].char
		c.normalWidth = win.content[row][col].normalWidth
	}
	if c.ws.palette != nil {
		if c.isInPalette {
			c.text = ""
		}
	}

	c.updateCursorShape()

	if c.ws.palette != nil {
		if c.ws.palette.widget.IsVisible() {
			return
		}
	}
	font := c.font
	if font == nil {
		return
	}

	x := int(float64(col) * font.truewidth)
	y := row*font.lineHeight + int(float64(font.lineSpace)/2.0) + c.shift + win.scrollPixels[1]
	c.x = x
	c.y = y
	c.move()
}

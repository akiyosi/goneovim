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
	widgets.QWidget
	_ float64 `property:"animationProp"`

	doAnimate     bool
	hasSmoothMove bool

	ws               *Workspace
	mode             string
	modeIdx          int
	x                float64
	y                float64
	oldx             float64
	oldy             float64
	delta            float64
	deltax           float64
	deltay           float64
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
	c := NewCursor(nil, 0)
	c.SetContentsMargins(0, 0, 0, 0)
	c.SetAttribute(core.Qt__WA_OpaquePaintEvent, true)
	c.SetStyleSheet(" * { background-color: rgba(0, 0, 0, 0);}")
	c.timer = core.NewQTimer(nil)
	c.isNeedUpdateModeInfo = true
	c.ConnectPaintEvent(c.paint)
	c.delta = 0.1
	c.hasSmoothMove = editor.config.Cursor.SmoothMove
	c.ConnectWheelEvent(c.wheelEvent)

	return c
}

func (c *Cursor) wheelEvent(event *gui.QWheelEvent) {
	var targetwin *Window
	c.ws.screen.windows.Range(func(_, winITF interface{}) bool {
		win := winITF.(*Window)
		if win == nil {
			return true
		}
		if !win.IsVisible() {
			return true
		}
		if win.grid == 1 {
			return true
		}
		if win.isMsgGrid {
			return true
		}
		if win.isFloatWin {
			if win.Geometry().Contains(event.Pos(), true) {
				targetwin = win
			}
			return false
		}

		return true
	})
	if targetwin == nil {
		c.ws.screen.windows.Range(func(_, winITF interface{}) bool {
			win := winITF.(*Window)
			if win == nil {
				return true
			}
			if !win.IsVisible() {
				return true
			}
			if win.grid == 1 {
				return true
			}
			if win.isMsgGrid {
				return true
			}
			if win.Geometry().Contains(event.Pos(), true) {
				targetwin = win
				return false
			}

			return true
		})
	}

	targetwin.WheelEvent(
		// NOTE: This is not an exact implementation, as it requires 
		// a coordinate transformation of the Pos of QwheelEvent, but
		// in the current handling of QWheelevent, it can be substituted as is.
		event,
	)
}

func (c *Cursor) paint(event *gui.QPaintEvent) {
	font := c.font
	if font == nil {
		return
	}
	if c == nil {
		return
	}
	if c.charCache == nil {
		return
	}

	// If guifontwide is set
	shift := font.ascent

	// width := font.truewidth
	// if !c.normalWidth {
	// 	width = width * 2
	// }

	p := gui.NewQPainter2(c)

	color := c.bg
	if color == nil {
		color = c.ws.foreground
	}

	var X, Y float64
	if c.deltax != 0 || c.deltay != 0 {
		if math.Abs(c.deltax) > 0 {
			X = c.oldx + c.deltax
		} else {
			X = c.x
		}
		if math.Abs(c.deltay) > 0 {
			Y = c.oldy + c.deltay
		} else {
			Y = c.y
		}
	} else {
		X = c.x
		Y = c.y
	}

	// Draw cursor background
	p.FillRect4(
		core.NewQRectF4(
			X,
			Y,
			float64(c.width),
			float64(c.height),
		),
		color.brend(c.ws.background, c.brend).QColor(),
	)

	if c.text == "" || c.devicePixelRatio == 0 {
		p.DestroyQPainter()
		return
	}

	var fx, fy float64
	var isDraw bool
	if c.deltax < 0 && c.deltay == 0 ||
	c.deltax > 0 && c.deltay == 0 ||
	c.deltax == 0 && c.deltay < 0 ||
	c.deltax == 0 && c.deltay > 0 {
		fx = c.x
		fy = c.y
		isDraw = true
	} else {
		fx = X
		fy = Y
		if c.delta > 0.95 || (c.deltax == 0 && c.deltay == 0) {
			isDraw = true
		} else {
			isDraw = false
		}
	}

	if isDraw && c.width > int(font.truewidth/2.0) {
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
					fx,
					fy,
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
					fx,
					fy+shift,
				),
				c.text,
			)
		}
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
	image := gui.NewQImage3(
		int(c.devicePixelRatio*width),
		int(c.devicePixelRatio*float64(font.height)),
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
		c.Update()
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
		c.Update()
	})
	c.timer.Start(wait)
	c.timer.SetInterval(off)
}

func (c *Cursor) move() {
	var y float64
	if c.ws.tabline != nil {
		if c.ws.tabline.widget.IsVisible() {
			y = y + float64(c.ws.tabline.height)
		}
	}
	c.Move(
		core.NewQPoint2(
			0,
			int(y),
		),
	)

	// if c.ws.loc != nil {
	// 	c.ws.loc.updatePos()
	// }
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
		if fg == nil {
			fg = c.ws.foreground
		}
		if bg == nil {
			bg = c.ws.background
		}
		c.fg = fg
		c.bg = bg

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
		width = int(math.Ceil(float64(width) * p))
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
	}

}

func (c *Cursor) updateContent() {
	if c.mode != c.ws.mode {
		c.mode = c.ws.mode
		if c.mode == "terminal-input" {
			c.Hide()
			return
		} else {
			c.Show()
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

	winx := win.pos[0]
	winy := win.pos[1]
	if win.isExternal {
		winx = 0
		winy = 0
	}

	winbordersize := 0
	if win.isExternal {
		winbordersize = EXTWINBORDERSIZE
	}

	if row >= len(win.content) ||
		col >= len(win.content[0]) ||
		win.content[row][col] == nil ||
		win.content[row][col].char == "" {
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

	res := 0
	if win.isMsgGrid {
		res = win.s.widget.Height() - win.rows*font.lineHeight
	}
	if res < 0 {
		res = 0
	}

	// Set smooth scroll offset
	scrollPixels := 0
	if editor.config.Editor.LineToScroll == 1 {
		scrollPixels += win.scrollPixels[1]
	}

	x := float64(col+winx)*font.truewidth + float64(winbordersize)
	y := float64((row+winy)*font.lineHeight) + float64(font.lineSpace)/2.0 + float64(c.shift + scrollPixels + res + winbordersize)

	c.move()
	if !(c.x == x && c.y == y) {
		c.oldx = c.x
		c.oldy = c.y
		c.x = x
		c.y = y
		c.doAnimate = true
		c.animateMove()
	}
}

func (c *Cursor) update() {
	c.updateContent()
	c.updateRegion()

	// Fix #119: Wrong candidate window position when using ibus
	if runtime.GOOS == "linux" {
		gui.QGuiApplication_InputMethod().Update(core.Qt__ImCursorRectangle)
	}
}

func(c *Cursor) updateRegion() {
	if !c.hasSmoothMove {
		c.updateMinimumArea()
	} else {
		if /*  */ c.deltax < 0 && c.deltay == 0 {
				c.Update2(int(math.Ceil(c.oldx+c.deltax)), 0, c.Width(), int(c.y)+c.height)
		} else if c.deltax > 0 && c.deltay == 0 {
				c.Update2(0, 0, int(math.Ceil(c.oldx+c.deltax+float64(c.width))), int(c.y)+c.height)
		} else if c.deltax == 0 && c.deltay < 0 {
				c.Update2(0, int(math.Ceil(c.oldy+c.deltay)), int(math.Ceil(c.x+float64(c.width))), c.Height())
		} else if c.deltax == 0 && c.deltay > 0 {
				c.Update2(0, 0, int(math.Ceil(c.oldx+float64(c.width))), int(c.oldy+c.deltay)+c.height)
		} else {
			if editor.isKeyAutoRepeating {
				c.updateMinimumArea()
			} else {
				c.Update()
			}
		}
	}
}

func(c *Cursor) updateMinimumArea() {
	c.Update2(
		int(math.Trunc(c.oldx)),
		int(math.Trunc(c.oldy)),
		int(math.Ceil(c.oldx+float64(c.width))),
		int(math.Ceil(c.oldy+float64(c.height))),
	)
	c.Update2(
		int(math.Trunc(c.x)),
		int(math.Trunc(c.y)),
		int(math.Ceil(c.x+float64(c.width))),
		int(math.Ceil(c.y+float64(c.height))),
	)
}

func(c *Cursor) animateMove() {
	if !c.doAnimate {
		return
	}
	if !c.hasSmoothMove {
		return
	}

	// process smooth scroll
	a := core.NewQPropertyAnimation2(c, core.NewQByteArray2("animationProp", len("animationProp")), c)
	a.ConnectValueChanged(func(value *core.QVariant) {
		if editor.isKeyAutoRepeating {
			return
		}
		ok := false
		v := value.ToDouble(&ok)
		if !ok {
			return
		}

		c.delta = v
		c.deltax = (c.x - c.oldx) * v
		c.deltay = (c.y - c.oldy) * v

		if v == 1.0 {
			c.delta = 0.1
			c.deltax = 0
			c.deltay = 0
			c.doAnimate = false
		}

		c.updateRegion()
	})
	d := math.Sqrt(
			math.Pow((c.x - c.oldx),2)+math.Pow((c.y - c.oldy),2),
	)
	f := 20*math.Pow(
		math.Atan(d/200),
		2,
	)
	duration := editor.config.Cursor.Duration+int(f)
	a.SetDuration(int(duration))
	a.SetStartValue(core.NewQVariant10(float64(0.1)))
	a.SetEndValue(core.NewQVariant10(1))
	// a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__OutQuart))
	// a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__OutExpo))
	// a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__OutQuint))
	// a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__InOutCubic))
	// a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__InOutQuint))
	a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__InOutCirc))
	// a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__Linear))
	// a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__InQuart))
	// a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__OutCubic))
	// a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__InOutQuart))
	// a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__OutInQuart))
	// a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__InOutExpo))
	// a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__OutCirc))
	// a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__InCubic))
	a.Start(core.QAbstractAnimation__DeletionPolicy(core.QAbstractAnimation__DeleteWhenStopped))
}

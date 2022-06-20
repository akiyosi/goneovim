package editor

import (
	"math"
	"runtime"
	"sync"

	"github.com/akiyosi/goneovim/util"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/widgets"
)

// Cursor is
type Cursor struct {
	widgets.QWidget
	charCache                  *Cache
	font                       *Font
	bg                         *RGBA
	fg                         *RGBA
	snapshot                   *gui.QPixmap
	fontwide                   *Font
	ws                         *Workspace
	timer                      *core.QTimer
	cursorShape                string
	desttext                   string
	sourcetext                 string
	mode                       string
	delta                      float64
	animationStartY            float64
	xprime                     float64
	yprime                     float64
	animationStartX            float64
	y                          float64
	deltay                     float64
	width                      int
	height                     int
	currAttrId                 int
	deltax                     float64
	gridid                     int
	prevGridid                 int
	bufferGridid               int
	horizontalShift            int
	modeIdx                    int
	blinkWait                  int
	modeInfoModeIdx            int
	blinkOn                    int
	blinkOff                   int
	brend                      float64
	_                          float64 `property:"animationProp"`
	devicePixelRatio           float64
	x                          float64
	cellPercentage             int
	paintMutex                 sync.RWMutex
	isBusy                     bool
	isInPalette                bool
	isNeedUpdateModeInfo       bool
	isTextDraw                 bool
	isShut                     bool
	avoidedToTakeFirstSnapshot bool
	isStopScroll               bool
	hasSmoothMove              bool
	doAnimate                  bool
	normalWidth                bool
}

func initCursorNew() *Cursor {
	c := NewCursor(nil, 0)

	c.SetContentsMargins(0, 0, 0, 0)
	c.SetAttribute(core.Qt__WA_OpaquePaintEvent, true)
	c.SetStyleSheet(" * { background-color: rgba(0, 0, 0, 0);}")
	c.timer = core.NewQTimer(nil)
	c.isNeedUpdateModeInfo = true
	c.ConnectPaintEvent(c.paintEvent)
	c.delta = 0.1
	c.hasSmoothMove = editor.config.Cursor.SmoothMove
	c.cellPercentage = 100
	c.SetFocusPolicy(core.Qt__NoFocus)
	c.ConnectWheelEvent(c.wheelEvent)

	return c
}

func (c *Cursor) paintEvent(event *gui.QPaintEvent) {
	if c == nil {
		return
	}

	if c.charCache == nil {
		return
	}

	if c.isBusy {
		return
	}

	font := c.font
	if font == nil {
		return
	}

	p := gui.NewQPainter2(c)

	c.drawBackground(p)

	if c.sourcetext == "" || c.devicePixelRatio == 0 || c.width < int(font.cellwidth/2.0) {
		p.DestroyQPainter()
		return
	}

	// Paint source cell text
	c.paintMutex.RLock()
	X, Y := c.getDrawingPos(c.x, c.y, c.xprime, c.yprime, c.deltax, c.deltay)

	if (c.deltax != 0 || c.deltay != 0) && c.delta < 1.0 {
		c.drawForeground(p, X, Y, c.animationStartX, c.animationStartY, c.sourcetext)
	}
	c.paintMutex.RUnlock()

	if c.desttext == "" {
		p.DestroyQPainter()
		return
	}

	// Paint destination text
	X2, Y2 := c.getDrawingPos(c.x, c.y, 0, 0, 0, 0)
	c.drawForeground(p, X, Y, X2, Y2, c.desttext)

	p.DestroyQPainter()
}

func (c *Cursor) wheelEvent(event *gui.QWheelEvent) {
	win, ok := c.ws.screen.getWindow(c.gridid)
	if !ok {
		return
	}
	win.WheelEvent(event)
}

func (c *Cursor) drawBackground(p *gui.QPainter) {
	// Draw cursor background
	color := c.bg
	if color == nil {
		color = c.ws.foreground
	}
	p.FillRect4(
		core.NewQRectF4(
			0,
			0,
			float64(c.width),
			float64(c.height),
		),
		color.brend(c.ws.background, c.brend).QColor(),
	)
}

func (c *Cursor) drawForeground(p *gui.QPainter, sx, sy, dx, dy float64, text string) {
	font := c.font
	if font == nil {
		return
	}
	shift := font.ascent

	// Paint target cell text
	if editor.config.Editor.CachedDrawing {
		var image *gui.QImage
		charCache := *c.charCache
		imagev, err := charCache.get(HlChars{
			text:   text,
			fg:     c.fg,
			italic: false,
			bold:   false,
		})
		if err != nil {
			image = c.newCharCache(text, c.fg, c.normalWidth)
			c.setCharCache(text, c.fg, image)
		} else {
			image = imagev.(*gui.QImage)
		}
		yy := dy - sy - float64(c.horizontalShift)
		if c.font.lineSpace < 0 {
			yy += float64(font.lineSpace) / 2.0
		}
		p.DrawImage9(
			int(dx-sx),
			int(yy),
			image,
			0, 0,
			-1, -1,
			core.Qt__AutoColor,
		)
	} else {
		if !c.normalWidth && c.fontwide != nil {
			p.SetFont(c.fontwide.fontNew)
			if c.fontwide.lineHeight > font.lineHeight {
				shift += c.fontwide.ascent - font.ascent
			}
		} else {
			p.SetFont(font.fontNew)
		}
		p.SetPen2(c.fg.QColor())

		yy := dy - sy + shift - float64(c.horizontalShift)
		if c.font.lineSpace < 0 {
			yy += float64(font.lineSpace) / 2.0
		}
		p.DrawText3(
			int(dx-sx),
			int(yy),
			text,
		)
	}
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

func (c *Cursor) setBlink(isUpdateBlinkWait, isUpdateBlinkOn, isUpdateBlinkOff bool) {
	c.timer.DisconnectTimeout()

	wait := c.blinkWait
	on := c.blinkOn
	off := c.blinkOff
	if wait == 0 || on == 0 || off == 0 {
		c.brend = 0.0
		c.paint()
		return
	}
	c.timer.ConnectTimeout(func() {
		if editor.isKeyAutoRepeating {
			c.brend = 0.0
			return
		}
		c.brend = 0.0
		if !c.isShut {
			c.timer.SetInterval(off)
			c.isShut = true
			c.brend = 0.6
		} else {
			c.timer.SetInterval(on)
			c.isShut = false
		}
		c.paint()
	})
	if isUpdateBlinkWait && wait != 0 {
		c.timer.Start(wait)
	}
	c.timer.SetInterval(off)
}

func (c *Cursor) getDrawingPos(x, y, xprime, yprime, deltax, deltay float64) (float64, float64) {
	var X, Y float64
	if deltax != 0 || deltay != 0 {
		if math.Abs(deltax) > 0 {
			X = xprime + deltax
		} else {
			X = x
		}
		if math.Abs(deltay) > 0 {
			Y = yprime + deltay
		} else {
			Y = y
		}
	} else {
		X = x
		Y = y
	}
	Y += float64(c.horizontalShift)

	return X, Y
}

func (c *Cursor) move() {
	X, Y := c.getDrawingPos(c.x, c.y, c.xprime, c.yprime, c.deltax, c.deltay)

	c.Move2(
		int(X),
		int(Y),
	)
}

func (c *Cursor) updateFont(targetWin *Window, font *Font) {
	win := targetWin
	ok := false
	if win == nil {
		win, ok = c.ws.screen.getWindow(c.bufferGridid)
		if !ok {
			return
		}
	}

	if win == nil {
		return
	}
	if win.font == nil {
		c.font = font
	} else {
		c.font = win.font
		c.paintMutex.Lock()
		c.snapshot.DestroyQPixmap()
		c.snapshot = nil
		c.paintMutex.Unlock()
	}
}

func (c *Cursor) updateCursorShape() {
	if !c.ws.cursorStyleEnabled {
		return
	}
	if editor.isKeyAutoRepeating {
		return
	}

	var cursorShape string
	var cellPercentage, blinkWait, blinkOn, blinkOff int
	var isUpdateBlinkWait, isUpdateBlinkOn, isUpdateBlinkOff bool
	if c.modeInfoModeIdx != c.modeIdx || c.isNeedUpdateModeInfo {
		c.modeInfoModeIdx = c.modeIdx

		modeInfo := c.ws.modeInfo[c.modeIdx]
		attrIdITF, ok := modeInfo["attr_id"]
		if ok {
			c.currAttrId = util.ReflectToInt(attrIdITF)
		}
		var bg, fg *RGBA
		if c.currAttrId == 0 {
			// Cursor attribute id (defined by `hl_attr_define`).
			// When attr_id is 0, the background and foreground
			// colors should be swapped. (See: runtime/doc/ui.txt)
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

		// c.cursorShape = "block"
		cursorShapeITF, ok := modeInfo["cursor_shape"]
		if ok {
			cursorShape = cursorShapeITF.(string)
			if c.cursorShape != cursorShape {
				c.cursorShape = cursorShape
			}
		}
		cellPercentageITF, ok := modeInfo["cell_percentage"]
		if ok {
			cellPercentage = util.ReflectToInt(cellPercentageITF)
			if c.cellPercentage != cellPercentage {
				c.cellPercentage = cellPercentage
			}
		}
		blinkWaitITF, blinkWaitOk := modeInfo["blinkwait"]
		if blinkWaitOk {
			blinkWait = util.ReflectToInt(blinkWaitITF)
		}
		blinkOnITF, blinkOnOk := modeInfo["blinkon"]
		if blinkOnOk {
			blinkOn = util.ReflectToInt(blinkOnITF)
		}
		blinkOffITF, blinkOffOk := modeInfo["blinkoff"]
		if blinkOffOk {
			blinkOff = util.ReflectToInt(blinkOffITF)
		}

		isUpdateBlinkWait = (blinkWaitOk && c.blinkWait != blinkWait)
		isUpdateBlinkOn = (blinkOnOk && c.blinkOn != blinkOn)
		isUpdateBlinkOff = (blinkOffOk && c.blinkOff != blinkOff)

		if isUpdateBlinkWait {
			c.blinkWait = blinkWait
		}
		if isUpdateBlinkOn {
			c.blinkOn = blinkOn
		}
		if isUpdateBlinkOff {
			c.blinkOff = blinkOff
		}
		c.setBlink(
			isUpdateBlinkWait,
			isUpdateBlinkOn,
			isUpdateBlinkOff,
		)

		c.isNeedUpdateModeInfo = false
	}

	var width, height int
	if c.font != nil {
		height = c.font.height
		if c.font.lineSpace < 0 {
			height += c.font.lineSpace
		}
		width = int(math.Trunc(c.font.cellwidth))
	}
	if !c.normalWidth {
		width = width * 2
	}
	p := float64(c.cellPercentage) / float64(100)

	switch c.cursorShape {
	case "horizontal":
		height = int(float64(height) * p)
		c.horizontalShift = int(float64(c.font.lineHeight) * (1.0 - p))
		if c.cellPercentage < 99 {
			c.isTextDraw = false
		} else {
			c.isTextDraw = true
		}
	case "vertical":
		c.isTextDraw = true
		width = int(math.Ceil(float64(width) * p))
		c.horizontalShift = 0
	default:
		c.isTextDraw = true
		c.horizontalShift = 0
	}

	if width == 0 {
		width = 1
	}
	if height == 0 {
		height = 1
	}

	if !(c.width == width && c.height == height) {
		c.width = width
		c.height = height
		c.resize(c.width, c.height)
	}
}

func (c *Cursor) getRowAndColFromScreen() (row, col int) {
	row = c.ws.screen.cursor[0]
	col = c.ws.screen.cursor[1]

	if row < 0 {
		row = 0
	}
	if col < 0 {
		col = 0
	}

	return
}

func (c *Cursor) updateCursorPos(row, col int, win *Window) {
	// Get the current font applied to the cursor.
	// If the cursor is on a window that has its own font setting,
	// get its own font.
	c.font = win.getFont()
	font := c.font
	if font == nil {
		return
	}

	// The position of the window is represented by coordinates
	// based on the width and height of the guifont or
	// the application's default font.
	baseFont := c.ws.screen.font

	winx := int(float64(win.pos[0]) * baseFont.cellwidth)
	winy := int(float64(win.pos[1] * baseFont.lineHeight))

	// Fix https://github.com/akiyosi/goneovim/issues/316#issuecomment-1039978355
	if win.isFloatWin && !win.isMsgGrid {
		winx, winy = win.repositioningFloatwindow()
	}

	if win.isExternal {
		winx = EXTWINBORDERSIZE
		winy = EXTWINBORDERSIZE
	}

	// Set smooth scroll offset
	scrollPixels := 0
	if editor.config.Editor.LineToScroll == 1 {
		scrollPixels += win.scrollPixels[1]
	}

	x := float64(winx + int(float64(col)*font.cellwidth))
	y := float64(winy + int(float64(row*font.lineHeight)+float64(scrollPixels)))
	if font.lineSpace > 0 {
		y += float64(font.lineSpace) / 2.0
	}

	if c.x == x && c.y == y {
		return
	}

	c.isStopScroll = (win.lastScrollphase == core.Qt__ScrollEnd)

	// If the cursor has not finished its animated movement
	if c.deltax != 0 || c.deltay != 0 {
		c.xprime = c.xprime + c.deltax
		c.yprime = c.yprime + c.deltay

		// Suppress cursor animation while touchpad scrolling is in progress.
		if !c.isStopScroll {
			c.xprime = x
			c.yprime = y
		}
	} else {
		c.xprime = c.x
		c.yprime = c.y
	}
	if c.deltax == 0 && c.deltay == 0 {
		c.animationStartX = c.x
		c.animationStartY = c.y
	}
	c.x = x
	c.y = y

	c.doAnimate = true
	// Suppress cursor animation while touchpad scrolling is in progress.
	if !c.isStopScroll {
		c.doAnimate = false
	}

	c.animateMove()
}

func (c *Cursor) updateCursorText(row, col int, win *Window) {
	if row >= len(win.content) ||
		col >= len(win.content[0]) ||
		win.content[row][col] == nil ||
		win.content[row][col].char == "" {
		c.desttext = ""
		c.normalWidth = true
	} else {
		if c.deltax == 0 && c.deltay == 0 {
			c.sourcetext = c.desttext
		}
		c.desttext = win.content[row][col].char
		c.normalWidth = win.content[row][col].normalWidth
	}
	if c.ws.palette != nil {
		if c.isInPalette {
			c.desttext = ""
		}
	}
}

func (c *Cursor) update() {
	if c.mode != c.ws.mode {
		c.mode = c.ws.mode
	}

	// get current window
	win, ok := c.ws.screen.getWindow(c.gridid)
	if !ok {
		return
	}

	// Set Window-specific properties
	charCache := win.getCache()
	c.charCache = &charCache
	c.devicePixelRatio = win.devicePixelRatio

	// Get row, col from screen
	row, col := c.getRowAndColFromScreen()

	// update cursor text
	c.updateCursorText(row, col, win)

	// update cursor shape
	c.updateCursorShape()

	// if ext_cmdline is true
	if c.ws.palette != nil {
		if c.ws.palette.widget.IsVisible() {
			c.redraw()
			return
		}
	}

	// update cursor pos on window
	c.updateCursorPos(row, col, win)

	// redraw cursor widget
	c.redraw()
}

func (c *Cursor) setColor() {
	color := c.bg
	if color == nil {
		color = c.ws.foreground
	}
	if color != nil {
		return
	}

	c.SetAutoFillBackground(true)
	p := gui.NewQPalette()
	p.SetColor2(gui.QPalette__Background, color.QColor())
	c.SetPalette(p)
}

func (c *Cursor) redraw() {
	c.move()
	c.paint()

	// Fix #119: Wrong candidate window position when using ibus
	if runtime.GOOS == "linux" {
		gui.QGuiApplication_InputMethod().Update(core.Qt__ImCursorRectangle)
	}
}

// paint() is to request update cursor widget.
// NOTE: This function execution may not be necessary.
//       This is because move() is performed in the redraw() of the cursor,
//       and it seems that paintEvent is fired inside
//       the cursor widget in conjunction with this move processing.
func (c *Cursor) paint() {
	if editor.isKeyAutoRepeating {
		return
	}

	c.Update()
}

func (c *Cursor) getSnapshot() {
	if !editor.isKeyAutoRepeating && editor.config.Cursor.SmoothMove {
		if !c.isStopScroll {
			return
		}
		// Avoid the error "QObject::setParent: Cannot set parent, new parent is in a different thread"
		if !c.ws.widget.IsVisible() && !c.avoidedToTakeFirstSnapshot {
			c.avoidedToTakeFirstSnapshot = true
			return
		}
		if c.deltax != 0 || c.deltay != 0 {
			return
		}

		snapshot := c.Grab(
			core.NewQRect4(
				0,
				0,
				c.width,
				c.height,
			),
		)
		c.paintMutex.Lock()
		c.snapshot.DestroyQPixmap()
		c.snapshot = snapshot
		c.paintMutex.Unlock()
	}
}

func (c *Cursor) animateMove() {
	if !c.doAnimate {
		return
	}
	if !c.hasSmoothMove {
		return
	}

	// process smooth scroll
	a := core.NewQPropertyAnimation2(c, core.NewQByteArray2("animationProp", len("animationProp")), c)
	a.ConnectValueChanged(func(value *core.QVariant) {
		if !c.doAnimate {
			return
		}
		ok := false
		v := value.ToDouble(&ok)
		if !ok {
			return
		}

		c.delta = v
		c.deltax = (c.x - c.xprime) * v
		c.deltay = (c.y - c.yprime) * v

		if v == 1.0 {
			c.delta = 0.09
			c.deltax = 0
			c.deltay = 0
			c.doAnimate = false
		}

		c.paint()
		c.move()
	})
	duration := editor.config.Cursor.Duration
	a.SetDuration(int(duration))
	a.SetStartValue(core.NewQVariant10(float64(0.1)))
	a.SetEndValue(core.NewQVariant10(1))
	a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__InOutCirc))
	// a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__OutQuart))
	// a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__OutExpo))
	// a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__OutQuint))
	// a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__InOutCubic))
	// a.SetEasingCurve(core.NewQEasingCurve(core.QEasingCurve__InOutQuint))
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

func (c *Cursor) resize(width, height int) {
	c.Resize2(width, height)
}

func (c *Cursor) raise() {
	c.Raise()
	c.Hide()
	c.Show()
}

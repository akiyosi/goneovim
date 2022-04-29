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
	text                       string
	mode                       string
	layerPos                   [2]int
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
	c.ConnectPaintEvent(c.paint)
	c.delta = 0.1
	c.hasSmoothMove = editor.config.Cursor.SmoothMove
	c.cellPercentage = 100

	return c
}

func (c *Cursor) setBypassScreenEvent() {
	c.ConnectWheelEvent(c.wheelEvent)
	c.ConnectMousePressEvent(c.ws.screen.mousePressEvent)
	c.ConnectMouseReleaseEvent(c.ws.screen.mouseEvent)
	c.ConnectMouseMoveEvent(c.ws.screen.mouseEvent)
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
		if win.isExternal {
			if win.grid == c.gridid {
				if win.Geometry().Contains(event.Pos(), true) {
					targetwin = win
				}
				// targetwin = win
				return false
			}
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
			if win.isFloatWin && !win.isExternal {
				if targetwin != nil {
					if targetwin.Geometry().Contains2(win.Geometry(), true) {
						targetwin = win
					}
				} else {
					if win.Geometry().Contains(event.Pos(), true) {
						targetwin = win
					}
				}
			}

			return true
		})
	}

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
			if win.isFloatWin || win.isExternal {
				return true
			}
			if win.Geometry().Contains(event.Pos(), true) {
				targetwin = win
				return false
			}

			return true
		})
	}

	if targetwin != nil {
		targetwin.WheelEvent(
			event,
		)
	}
}

func (c *Cursor) paint(event *gui.QPaintEvent) {
	if c == nil {
		return
	}

	font := c.font
	if font == nil {
		return
	}
	if c.charCache == nil {
		return
	}

	// If guifontwide is set
	shift := font.ascent

	p := gui.NewQPainter2(c)
	if c.isBusy {
		p.DestroyQPainter()
		return
	}

	var X, Y float64
	if c.deltax != 0 || c.deltay != 0 {
		if math.Abs(c.deltax) > 0 {
			X = c.xprime + c.deltax
		} else {
			X = c.x
		}
		if math.Abs(c.deltay) > 0 {
			Y = c.yprime + c.deltay
		} else {
			Y = c.y
		}
	} else {
		X = c.x
		Y = c.y
	}

	if c.hasSmoothMove {
		p.SetClipRect2(
			core.NewQRect4(
				int(X),
				int(Y)+c.horizontalShift,
				c.width,
				c.height,
			), core.Qt__IntersectClip,
		)
	}

	// Draw cursor background
	color := c.bg
	if color == nil {
		color = c.ws.foreground
	}
	p.FillRect4(
		core.NewQRectF4(
			X,
			Y+float64(c.horizontalShift),
			float64(c.width),
			float64(c.height),
		),
		color.brend(c.ws.background, c.brend).QColor(),
	)

	// Draw source cell text
	c.paintMutex.RLock()
	if c.snapshot != nil && (c.deltax != 0 || c.deltay != 0) {
		p.DrawPixmap9(
			int(c.animationStartX),
			int(c.animationStartY),
			c.snapshot,
		)
	}
	c.paintMutex.RUnlock()

	if c.text == "" || c.devicePixelRatio == 0 {
		p.DestroyQPainter()
		return
	}

	// Draw target cell text
	if c.width > int(font.cellwidth/2.0) {
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
					c.x,
					c.y,
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
					c.x,
					c.y+shift,
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

func (c *Cursor) setBlink(isUpdateBlinkWait, isUpdateBlinkOn, isUpdateBlinkOff bool) {
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
		c.Update()
	})
	if isUpdateBlinkWait && wait != 0 {
		c.timer.Start(wait)
	}
	c.timer.SetInterval(off)
}

func (c *Cursor) move(win *Window) {
	if win == nil {
		return
	}

	var x, y float64
	if !win.isExternal {
		if c.ws.tabline != nil {
			if c.ws.tabline.widget.IsVisible() {
				y = y + float64(c.ws.tabline.height)
			}
		}
	}

	if c.layerPos[0] != int(x) || c.layerPos[1] != int(y) {
		c.Move(
			core.NewQPoint2(
				int(x),
				int(y),
			),
		)
	}
	c.layerPos[0] = int(x)
	c.layerPos[1] = int(y)
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
	}

}

func (c *Cursor) updateContent(win *Window) {
	charCache := win.getCache()
	c.charCache = &charCache
	c.devicePixelRatio = win.devicePixelRatio

	row := c.ws.screen.cursor[0]
	col := c.ws.screen.cursor[1]

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
		winx = 0
		winy = 0
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

	x := float64(winx + int(float64(col)*font.cellwidth+float64(winbordersize)))
	y := float64(winy + int(float64(row*font.lineHeight)+float64(font.lineSpace)/2.0+float64(scrollPixels+res+winbordersize)))

	isStopScroll := (win.lastScrollphase == core.Qt__ScrollEnd)
	c.move(win)
	if !(c.x == x && c.y == y) {
		// If the cursor has not finished its animated movement
		if c.deltax != 0 || c.deltay != 0 {
			c.xprime = c.xprime + c.deltax
			c.yprime = c.yprime + c.deltay

			// Suppress cursor animation while touchpad scrolling is in progress.
			if !isStopScroll {
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
		if !isStopScroll {
			c.doAnimate = false
		}
		c.animateMove()
	}
}

func (c *Cursor) update() {
	if c.mode != c.ws.mode {
		c.mode = c.ws.mode
	}

	win, ok := c.ws.screen.getWindow(c.gridid)
	if !ok {
		return
	}
	c.isStopScroll = (win.lastScrollphase == core.Qt__ScrollEnd)

	c.updateContent(win)

	c.updateRegion()

	// Fix #119: Wrong candidate window position when using ibus
	if runtime.GOOS == "linux" {
		gui.QGuiApplication_InputMethod().Update(core.Qt__ImCursorRectangle)
	}
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
				int(c.x),
				int(c.y),
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

func (c *Cursor) updateRegion() {
	if c.font == nil {
		return
	}
	width := int(math.Ceil(c.font.cellwidth))
	height := c.font.height
	if c.width > width {
		width = c.width
	}
	if c.height > height {
		width = c.height
	}

	if !c.hasSmoothMove {
		c.Update2(
			int(math.Trunc(c.xprime)),
			int(math.Trunc(c.yprime)),
			int(math.Ceil(c.xprime+float64(c.width))),
			int(math.Ceil(c.yprime+float64(c.height))),
		)
		c.Update2(
			int(math.Trunc(c.x)),
			int(math.Trunc(c.y))+c.horizontalShift,
			int(math.Ceil(c.x+float64(c.width))),
			int(math.Ceil(c.y+float64(c.height))),
		)
	} else {
		c.updateMinimumArea()
	}
}

func (c *Cursor) updateMinimumArea() {
	var topleft, topright, topbottom *core.QPoint
	var bottomright, bottomleft, bottomtop *core.QPoint
	width := float64(c.width)
	height := float64(c.height)

	var poly *gui.QPolygon

	var top, left, right, bottom float64

	// [Make updating region]
	//
	//    <topleft>
	//    xprime,yprime                        x,y
	//              +---+\  <topright>           +---+\
	//              |   | \                      |   | \
	//              |   |  \                     |   |  \
	//              |   |   \                    |   |   \
	//  <topbottom> +---+    \                   +---+    \
	//               \        \                   \        \
	//                \        \                   \        \
	//                 \   x,y  \                   \ xprime,yprime
	//                  \    +---+ <bottomtop>       \    +---+
	//                   \   |   |                    \   |   |
	//                    \  |   |                     \  |   |
	//                     \ |   |                      \ |   |
	//                      \+---+                       \+---+
	//            <bottomleft>   <bottomright>

	padding := 1
	if c.xprime < c.x && c.yprime < c.y || c.xprime > c.x && c.yprime > c.y {
		if c.xprime < c.x {
			left = c.xprime
			right = c.x
		} else {
			left = c.x
			right = c.xprime
		}
		if c.yprime < c.y {
			top = c.yprime
			bottom = c.y
		} else {
			top = c.y
			bottom = c.yprime
		}

		topleft = core.NewQPoint2(
			int(math.Trunc(left)),
			int(math.Trunc(top)),
		)
		topright = core.NewQPoint2(
			int(math.Trunc(left+width))+padding,
			int(math.Trunc(top)),
		)
		topbottom = core.NewQPoint2(
			int(math.Trunc(left))-padding,
			int(math.Trunc(top+height)),
		)
		bottomright = core.NewQPoint2(
			int(math.Trunc(right+width)),
			int(math.Trunc(bottom+height)),
		)
		bottomleft = core.NewQPoint2(
			int(math.Trunc(right))-padding,
			int(math.Trunc(bottom+height)),
		)
		bottomtop = core.NewQPoint2(
			int(math.Trunc(right+width))+padding,
			int(math.Trunc(bottom)),
		)

		poly = gui.NewQPolygon3(
			[]*core.QPoint{
				topbottom, topleft, topright,
				bottomtop, bottomright, bottomleft,
				topbottom,
			},
		)
	} else {

		//           <topleft>
		//                  x,y                         xprime,yprime
		//                   /+---+ <topright>                /+---+
		//                  / |   |                          / |   |
		//                 /  |   |                         /  |   |
		//                /   |   |                        /   |   |
		//               /    +---+ <topbottom>           /    +---+
		//              /        /                       /        /
		// <bottomtop> /        /                       /        /
		//    xprime,yprime    /                   x,y /        /
		//           +---+    /                       +---+    /
		//           |   |   /                        |   |   /
		//           |   |  /                         |   |  /
		//           |   | /                          |   | /
		//           +---+/                           +---+/
		// <bottomleft>   <bottomright>

		if c.xprime < c.x {
			left = c.xprime
			right = c.x
		} else {
			left = c.x
			right = c.xprime
		}
		if c.yprime < c.y {
			top = c.yprime
			bottom = c.y
		} else {
			top = c.y
			bottom = c.yprime
		}

		topleft = core.NewQPoint2(
			int(math.Trunc(right))-padding,
			int(math.Trunc(top)),
		)
		topright = core.NewQPoint2(
			int(math.Trunc(right+width)),
			int(math.Trunc(top)),
		)
		topbottom = core.NewQPoint2(
			int(math.Trunc(right+width))+padding,
			int(math.Trunc(top+height)),
		)
		bottomright = core.NewQPoint2(
			int(math.Trunc(left+width))+padding,
			int(math.Trunc(bottom+height)),
		)
		bottomleft = core.NewQPoint2(
			int(math.Trunc(left)),
			int(math.Trunc(bottom+height)),
		)
		bottomtop = core.NewQPoint2(
			int(math.Trunc(left))-padding,
			int(math.Trunc(bottom)),
		)

		poly = gui.NewQPolygon3(
			[]*core.QPoint{
				topleft, topright, topbottom,
				bottomright, bottomleft, bottomtop,
				topleft,
			},
		)
	}

	rgn := gui.NewQRegion4(
		poly,
		core.Qt__OddEvenFill,
	)
	c.Update4(rgn)
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

		c.updateRegion()
		if v == 1.0 {
			c.getSnapshot()
		}

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

func (c *Cursor) raise() {
	c.Raise()
	c.Hide()
	c.Show()
}

package editor

import (
	"fmt"
	"math"

	"github.com/akiyosi/goneovim/util"
	"github.com/therecipe/qt/widgets"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
)

// ScrollBar is
type ScrollBar struct {
	ws        *Workspace
	widget    *widgets.QWidget
	thumb     *widgets.QWidget
	pos       int
	height    int
	isPressed bool
	beginPosY int
}

func newScrollBar() *ScrollBar {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetFixedWidth(10)
	thumb := widgets.NewQWidget(widget, 0)
	thumb.SetFixedWidth(6)

	scrollBar := &ScrollBar{
		widget: widget,
		thumb:  thumb,
	}
	scrollBar.widget.Hide()

	scrollBar.thumb.ConnectMousePressEvent(scrollBar.mousePress)
	scrollBar.thumb.ConnectMouseMoveEvent(scrollBar.mouseScroll)
	scrollBar.thumb.ConnectMouseReleaseEvent(scrollBar.mouseRelease)
	scrollBar.thumb.ConnectEnterEvent(scrollBar.mouseEnter)
	scrollBar.thumb.ConnectLeaveEvent(scrollBar.mouseLeave)

	return scrollBar
}

func (s *ScrollBar) mouseEnter(e *core.QEvent) {
	color := editor.colors.selectedBg.String()
	s.thumb.SetStyleSheet(fmt.Sprintf(" * { background: %s;}", color))
}

func (s *ScrollBar) mouseLeave(e *core.QEvent) {
	color := editor.colors.scrollBarFg.String()
	s.thumb.SetStyleSheet(fmt.Sprintf(" * { background: %s;}", color))
}

func (s *ScrollBar) mousePress(e *gui.QMouseEvent) {
	s.beginPosY = e.GlobalPos().Y()
	s.isPressed = true
}

func (s *ScrollBar) mouseScroll(e *gui.QMouseEvent) {
	win, ok := s.ws.screen.getWindow(s.ws.cursor.gridid)
	if !ok {
		return
	}
	font := win.getFont()

	ratio := float64(s.ws.maxLine * font.lineHeight) / float64(s.widget.Height())
	v := s.beginPosY - e.GlobalPos().Y()
	if v == 0 {
		return
	}
	v2 := int(math.Ceil(float64(v) * ratio))
	s.scroll(v2, 0)
	s.beginPosY = e.GlobalPos().Y()

	s.update()

}

func (s *ScrollBar) mouseRelease(e *gui.QMouseEvent) {
	s.isPressed = false
	s.scroll(0, 0)
}

// for smooth scroll, but it has some probrem
func (s *ScrollBar) scroll(v, h int) {
	var vert int
	var vertKey string

	win, ok := s.ws.screen.getWindow(s.ws.cursor.gridid)
	if !ok {
		return
	}
	font := win.getFont()

	isStopScroll := !s.isPressed

	if int(math.Abs(float64(v))) >= font.lineHeight {
		vert = v / font.lineHeight
	} else {
		vert, _ = win.smoothUpdate(v, h, isStopScroll)
	}

	if vert == 0 {
		return
	}
	if vert > 0 {
		vertKey = "Up"
	} else {
		vertKey = "Down"
	}

	// Detect current mode
	mode := win.s.ws.mode
	if mode != "normal" {
		win.s.ws.nvim.Input(win.s.ws.escKeyInInsert)
	} else if mode == "terminal-input" {
		win.s.ws.nvim.Input(`<C-\><C-n>`)
	}

	if win.s.ws.isMappingScrollKey {
		if vert != 0 {
			win.s.ws.nvim.Input(fmt.Sprintf("<ScrollWheel%s>", vertKey))
		}
	} else {
		if vert > 0 {
			win.s.ws.nvim.Input(fmt.Sprintf("%v<C-y>", int(math.Abs(float64(vert)))))
		} else if vert < 0 {
			win.s.ws.nvim.Input(fmt.Sprintf("%v<C-e>", int(math.Abs(float64(vert)))))
		}
	}
}

func (s *ScrollBar) setColor() {
	fg := editor.colors.scrollBarFg.String()
	s.thumb.SetStyleSheet(fmt.Sprintf(" * { background: %s;}", fg))
	s.widget.SetStyleSheet(" * { background: rgba(0, 0, 0, 0);}")
}

func (s *ScrollBar) update() {
	win, ok := s.ws.screen.getWindow(s.ws.cursor.gridid)
	if !ok {
		return
	}
	top := win.scrollRegion[0]
	bot := win.scrollRegion[1]
	if top == 0 && bot == 0 {
		top = 0
		bot = s.ws.rows - 1
	}
	relativeCursorY := int(float64(s.ws.cursor.y) / float64(s.ws.font.lineHeight))
	if s.ws.maxLine == 0 {
		//s.ws.nvim.Eval("line('$')", &s.ws.maxLine)
		lnITF, err := s.ws.nvimEval("line('$')")
		if err != nil {
			s.ws.maxLine = 0
		} else {
			s.ws.maxLine = util.ReflectToInt(lnITF)
		}

	}

	if s.ws.maxLine > bot-top {
		s.height = int(float64(bot-top) / float64(s.ws.maxLine) * float64(s.ws.screen.widget.Height()))
		height := s.height
		if s.height < 20 {
			height = 20
		}
		s.thumb.SetFixedHeight(height)
		s.pos = int(float64(s.ws.curLine-relativeCursorY) / float64(s.ws.maxLine) * float64(s.ws.screen.widget.Height()))
		s.thumb.Move2(0, s.pos)
		s.widget.Show()
	} else {
		s.widget.Hide()
	}
}

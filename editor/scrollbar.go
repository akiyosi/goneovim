package editor

import (
	"fmt"
	"math"
	"sync"

	"github.com/akiyosi/goneovim/util"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/widgets"
)

// ScrollBar is
type ScrollBar struct {
	mu sync.Mutex

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
	thumb.SetFixedWidth(8)

	scrollBar := &ScrollBar{
		widget: widget,
		thumb:  thumb,
	}

	scrollBar.thumb.ConnectMousePressEvent(scrollBar.thumbPress)
	scrollBar.thumb.ConnectMouseMoveEvent(scrollBar.thumbScroll)
	scrollBar.thumb.ConnectMouseReleaseEvent(scrollBar.thumbRelease)
	scrollBar.thumb.ConnectEnterEvent(scrollBar.thumbEnter)
	scrollBar.thumb.ConnectLeaveEvent(scrollBar.thumbLeave)

	scrollBar.widget.Hide()
	return scrollBar
}

func (s *ScrollBar) thumbEnter(e *core.QEvent) {
	color := editor.config.SideBar.AccentColor
	s.thumb.SetStyleSheet(fmt.Sprintf(" * { background: %s;}", color))
}

func (s *ScrollBar) thumbLeave(e *core.QEvent) {
	color := editor.colors.scrollBarFg.String()
	s.thumb.SetStyleSheet(fmt.Sprintf(" * { background: %s;}", color))
}

func (s *ScrollBar) thumbPress(e *gui.QMouseEvent) {
	switch e.Button() {
	case core.Qt__LeftButton:
		s.beginPosY = e.GlobalPos().Y()
		s.isPressed = true
	default:
	}
}

func (s *ScrollBar) thumbScroll(e *gui.QMouseEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	win, ok := s.ws.screen.getWindow(s.ws.cursor.gridid)
	if !ok {
		return
	}
	font := win.getFont()

	thumbHeight := s.height
	if s.height < 20 {
		thumbHeight = 20
	}
	ratio := float64((s.ws.maxLine*font.lineHeight)+thumbHeight) / float64(s.widget.Height())
	v := s.beginPosY - e.GlobalPos().Y()
	if v == 0 {
		return
	}
	v2 := int(math.Ceil(float64(v) * ratio))
	s.beginPosY = e.GlobalPos().Y()
	s.scroll(v2, 0)

	s.update()

}

func (s *ScrollBar) thumbRelease(e *gui.QMouseEvent) {
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

	// Detect current mode
	mode := win.s.ws.mode
	if mode == "terminal-input" {
		win.s.ws.nvim.Input(`<C-\><C-n>`)
	} else if mode != "normal" {
		win.s.ws.nvim.Input(win.s.ws.escKeyInInsert)
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
	font := win.getFont()
	relativeCursorY := int(float64(s.ws.cursor.y) / float64(font.lineHeight))
	if s.ws.maxLine == 0 {
		lnITF, err := s.ws.nvimEval("line('$')")
		if err != nil {
			s.ws.maxLine = 0
		} else {
			s.ws.maxLine = util.ReflectToInt(lnITF)
		}

	}

	if s.ws.maxLine > bot-top {
		s.height = int(float64(bot-top) / float64(s.ws.maxLine) * float64(s.ws.screen.widget.Height()))
		thumbHeight := s.height
		if s.height < 20 {
			thumbHeight = 20
		}
		s.thumb.SetFixedHeight(thumbHeight)
		s.pos = int(float64(s.ws.curLine-relativeCursorY) / float64(s.ws.maxLine) * float64(s.ws.screen.widget.Height()))
		s.thumb.Move2(0, s.pos)
		s.widget.Show()
	} else {
		s.widget.Hide()
	}
}

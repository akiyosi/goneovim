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
	fg        *RGBA
	ws        *Workspace
	widget    *widgets.QWidget
	thumb     *widgets.QWidget
	pos       int
	height    int
	beginPosY int
	mu        sync.Mutex
	isPressed bool
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
	shift := -40
	if s.ws.screenbg == "light" {
		shift = 40
	}
	color := warpColor(s.fg, shift)
	s.thumb.SetStyleSheet(fmt.Sprintf(" * { background: %s;}", color))
}

func (s *ScrollBar) thumbLeave(e *core.QEvent) {
	color := s.fg
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
	s.fg = editor.colors.scrollBarFg
	if editor.config.ScrollBar.Color != "" {
		s.fg = hexToRGBA(editor.config.ScrollBar.Color)
	}
	s.thumb.SetStyleSheet(fmt.Sprintf(" * { background: %s;}", s.fg.String()))
	s.widget.SetStyleSheet(" * { background: rgba(0, 0, 0, 0);}")
}

func (s *ScrollBar) update() {
	win, ok := s.ws.screen.getWindow(s.ws.cursor.gridid)
	if !ok {
		return
	}
	rows := win.rows
	if s.ws.maxLine == 0 {
		lnITF, err := s.ws.nvimEval("line('$')")
		if err != nil {
			s.ws.maxLine = 0
		} else {
			s.ws.maxLine = util.ReflectToInt(lnITF)
		}
	}

	if s.ws.maxLine > rows {
		s.height = int(float64(rows) / float64(s.ws.maxLine) * float64(s.ws.screen.widget.Height()))
		thumbHeight := s.height
		if s.height < 20 {
			thumbHeight = 20
		}
		s.thumb.SetFixedHeight(thumbHeight)
		top := 0
		top = s.ws.viewport[0] - 1
		editor.putLog("scrollbar: debug::", top, s.ws.maxLine)
		s.pos = int(float64(top) / float64(s.ws.maxLine) * float64(s.ws.screen.widget.Height()))
		s.thumb.Move2(0, s.pos)
		s.widget.Show()
	} else {
		s.widget.Hide()
	}
}

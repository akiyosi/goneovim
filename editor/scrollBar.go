package editor

import (
	"sync"

	"github.com/therecipe/qt/widgets"
)

// ScrollBar is
type ScrollBar struct {
	ws     *Workspace
	widget *widgets.QWidget
	thumb  *widgets.QWidget
	pos    int
	height int
	mu     sync.Mutex
	isMove int
}

func newScrollBar() *ScrollBar {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetFixedWidth(5)
	thumb := widgets.NewQWidget(widget, 0)
	thumb.SetFixedWidth(5)

	scrollBar := &ScrollBar{
		widget: widget,
		thumb:  thumb,
	}
	scrollBar.widget.Hide()

	return scrollBar
}

func (s *ScrollBar) update() {
	if s.isMove > 0 {
		return
	}
	s.mu.Lock()

	defer s.mu.Unlock()
	defer func() { s.isMove = 0 }()
	s.isMove++

	top := s.ws.screen.scrollRegion[0]
	bot := s.ws.screen.scrollRegion[1]
	if top == 0 && bot == 0 {
		return
	}
	if s.ws.maxLine > bot-top {
		s.height = int(float64(bot-top) / float64(s.ws.maxLine) * float64(s.ws.screen.widget.Height()))
		s.thumb.SetFixedHeight(s.height)
		s.pos = int(float64(s.ws.statusline.pos.ln) / float64(s.ws.maxLine) * float64(s.ws.screen.widget.Height()))
		s.move()
		s.widget.Show()
	} else {
		s.widget.Hide()
	}
}

func (s *ScrollBar) move() {
	var pos int
	pos = s.pos - s.height/2
	screenHeight := s.ws.screen.widget.Height()
	if pos < 0 {
		pos = 0
	} else if pos > screenHeight-s.height {
		pos = screenHeight - s.height
	}
	s.thumb.Move2(0, pos)
}

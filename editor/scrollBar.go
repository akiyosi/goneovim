package editor

import (
	"fmt"

	"github.com/therecipe/qt/widgets"
)

// ScrollBar is
type ScrollBar struct {
	ws     *Workspace
	widget *widgets.QWidget
	thumb  *widgets.QWidget
	pos    int
	height int
}

func newScrollBar() *ScrollBar {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetFixedWidth(10)
	thumb := widgets.NewQWidget(widget, 0)
	thumb.SetFixedWidth(5)

	scrollBar := &ScrollBar{
		widget: widget,
		thumb:  thumb,
	}
	scrollBar.widget.Hide()

	return scrollBar
}

func (s *ScrollBar) setColor() {
	fg := editor.colors.scrollBarFg.String()
	bg := editor.colors.scrollBarBg.StringTransparent()
	s.thumb.SetStyleSheet(fmt.Sprintf(" * { background: %s;}", fg))
	s.widget.SetStyleSheet(fmt.Sprintf(" * { background: %s;}", bg))
}

func (s *ScrollBar) update() {
	top := s.ws.screen.scrollRegion[0]
	bot := s.ws.screen.scrollRegion[1]
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
			switch lnITF.(type) {
			case uint64:
				s.ws.maxLine = int(lnITF.(uint64))
			case uint:
				s.ws.maxLine = int(lnITF.(uint))
			case int64:
				s.ws.maxLine = int(lnITF.(int64))
			case int:
				s.ws.maxLine = lnITF.(int)
			}
		}

	}
	if s.ws.maxLine > bot-top {
		s.height = int(float64(bot-top) / float64(s.ws.maxLine) * float64(s.ws.screen.widget.Height()))
		if s.height < 20 {
			s.height = 20
		}
		s.thumb.SetFixedHeight(s.height)
		s.pos = int(float64(s.ws.curLine-relativeCursorY) / float64(s.ws.maxLine) * float64(s.ws.screen.widget.Height()))
		s.thumb.Move2(0, s.pos)
		s.widget.Show()
	} else {
		s.widget.Hide()
	}
}

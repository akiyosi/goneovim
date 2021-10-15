package editor

import (

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/widgets"
)


// IMETooltip is the tooltip for Input Method Editor
type IMETooltip struct {
	widgets.QWidget

	s          *Screen
	text       string
	font       *Font
	widthSlice []float64
}

func (i *IMETooltip) paint(event *gui.QPaintEvent) {
	p := gui.NewQPainter2(i)
	font := i.font
	if i.s.ws.palette.widget.IsVisible() {
		p.SetFont(
			gui.NewQFont2(editor.extFontFamily, editor.extFontSize, 1, false),
		)
	} else {
		p.SetFont(font.fontNew)
	}

	p.SetPen2(i.s.ws.foreground.QColor())
	if i.text != "" {
		r := []rune(i.text)
		var x float64
		var shift int
		for k := 0; k < len(r); k++ {
			var width float64
			fontMetrics := gui.NewQFontMetricsF(gui.NewQFont2(editor.extFontFamily, editor.extFontSize, 1, false))
			if i.s.ws.palette.widget.IsVisible() {
				width = fontMetrics.HorizontalAdvance(string(r[k]), -1)
				shift = int(fontMetrics.Ascent())
			} else {
				width = i.widthSlice[k]
				shift = font.shift
			}
			x += width
			if i.s.ws.palette.widget.IsVisible() {
				shift = int(fontMetrics.Ascent())
			} else {
				shift = font.shift
			}
			pos := core.NewQPointF3(
				float64(x),
				float64(shift),
			)
			p.DrawText(pos, string(r[k]))

		}
	}

	p.DestroyQPainter()
}


func (i *IMETooltip) pos() (int, int, int, int) {
	var x, y, candX, candY int
	ws := i.s.ws
	s := i.s
	if s.lenWindows() == 0 {
		return 0, 0, 0, 0
	}
	if ws.palette == nil {
		return 0, 0, 0, 0
	}
	if ws.palette.widget.IsVisible() {
		i.SetParent(s.ws.palette.widget)
		// font := gui.NewQFont2(editor.extFontFamily, editor.extFontSize, 1, false)
		// i.SetFont(font)
		x = ws.palette.cursorX + ws.palette.patternPadding
		candX = x + ws.palette.widget.Pos().X()
		y = ws.palette.patternPadding + ws.palette.padding
		candY = y + ws.palette.widget.Pos().Y()
	} else {
		win, ok := s.getWindow(s.ws.cursor.gridid)
		if !ok {
			return 0, 0, 0, 0
		}
		font := win.getFont()
		i.setFont(font)
		row := s.cursor[0]
		col := s.cursor[1]
		x = int(float64(col) * font.truewidth)
		y = row * font.lineHeight

		candX = int(float64(col+win.pos[0]) * font.truewidth)
		tablineMarginTop := 0
		if ws.tabline != nil {
			tablineMarginTop = ws.tabline.marginTop
		}
		tablineHeight := 0
		if ws.tabline != nil {
			tablineHeight = ws.tabline.height
		}
		tablineMarginBottom := 0
		if ws.tabline != nil {
			tablineMarginBottom = ws.tabline.marginBottom
		}
		candY = (row+win.pos[1])*font.lineHeight + tablineMarginTop + tablineHeight + tablineMarginBottom
	}
	return x, y, candX, candY
}

func (i *IMETooltip) move(x int, y int) {
	if i.s.ws.palette == nil {
		return
	}
	padding := 0
	if i.s.ws.palette.widget.IsVisible() {
		padding = i.s.ws.palette.padding
	}
	i.Move(core.NewQPoint2(x+padding, y))
}

func (i *IMETooltip) setFont(font *Font) {
	i.SetFont(font.fontNew)
	i.font = font
}

func (i *IMETooltip) show() {
	if i.s.ws.palette == nil {
		return
	}
	if !i.s.ws.palette.widget.IsVisible() {
		win, ok := i.s.getWindow(i.s.ws.cursor.gridid)
		if ok {
			i.SetParent(win)
		}
	}
	i.Show()
}

func (i *IMETooltip) update(text string) {
	if i.font == nil {
		return
	}
	font := i.font
	i.text = text
	r := []rune(i.text)
	var wSlice []float64
		wSlice = append(wSlice, 0.0)
	for k := 0; k < len(r); k++ {
		width := font.truewidth
		for {
			cwidth := font.fontMetrics.HorizontalAdvance(string(r[k]), -1)
			if cwidth <= width { break }
			width += font.truewidth
		}
		wSlice = append(wSlice, width)
	}
	i.widthSlice = wSlice
	var tooltipWidth float64
	for _, w := range i.widthSlice {
		tooltipWidth += w
	}
	i.SetFixedWidth(
		int(tooltipWidth),
	)
	i.Update()

	s := i.s
	row := s.cursor[0]
	col := s.cursor[1]
	c := s.ws.cursor
	c.x = float64(col)*s.font.truewidth + float64(s.tooltip.Width())
	c.y = float64(row * s.font.lineHeight)

	c.move(nil)
	i.show()
	i.Raise()
}

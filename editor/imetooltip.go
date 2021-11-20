package editor

import (
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
)


// IMETooltip is the tooltip for Input Method Editor
type IMETooltip struct {
	Tooltip
}

func (i *IMETooltip) setQpainterFont(p *gui.QPainter) {
	if i.s.ws.palette.widget.IsVisible() {
		p.SetFont(
			gui.NewQFont2(editor.extFontFamily, editor.extFontSize, 1, false),
		)
	} else {
		p.SetFont(i.font.fontNew)
	}
}

func (i *IMETooltip) getNthWidthAndShift(n int) (float64, int) {
	var width float64
	var shift int
	r := []rune(i.text)

	if i.s.ws.palette.widget.IsVisible() {
		fontMetrics := gui.NewQFontMetricsF(gui.NewQFont2(editor.extFontFamily, editor.extFontSize, 1, false))
		width = fontMetrics.HorizontalAdvance(string(r[n]), -1)
		if n == 0 { width = 0 }
		shift = int(fontMetrics.Ascent())
	} else {
		width = i.widthSlice[n]
		shift = i.font.shift
	}

	return width, shift
}

func (i *IMETooltip) drawForeground(p *gui.QPainter) {
	p.SetPen2(i.s.ws.foreground.QColor())

	if i.text != "" {
		r := []rune(i.text)
		var x float64
		for k := 0; k < len(r); k++ {

			width, shift := i.getNthWidthAndShift(k)
			x += width

			p.DrawText(
				core.NewQPointF3(
					float64(x),
					float64(shift),
				),
				string(r[k]),
			)

		}
	}
}

func (i *IMETooltip) paint(event *gui.QPaintEvent) {
	p := gui.NewQPainter2(i)

	i.setQpainterFont(p)
	i.drawForeground(p)

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

func (i *IMETooltip) show() {
	if i.s.ws.palette == nil {
		return
	}
	if !i.s.ws.palette.widget.IsVisible() {
		win, ok := i.s.getWindow(i.s.ws.cursor.gridid)
		if ok {
			i.SetParent(win)
		}
	} else {
		i.SetParent(i.s.ws.palette.widget)
	}

	i.Show()
	i.Raise()
}


func (i *IMETooltip) updateText(text string) {
	if i.font == nil {
		return
	}
	font := i.font

	// update text in struct
	i.text = text

	// rune text
	r := []rune(i.text)

	// init slice
	var wSlice []float64
	wSlice = append(wSlice, 0.0)


	var width float64
	var fontMetrics *gui.QFontMetricsF
	if i.s.ws.palette.widget.IsVisible() {
		fontMetrics = gui.NewQFontMetricsF(gui.NewQFont2(editor.extFontFamily, editor.extFontSize, 1, false))
		width = fontMetrics.HorizontalAdvance("w", -1)
	} else {
		fontMetrics = font.fontMetrics
		width = font.truewidth
	}

	for k := 0; k < len(r); k++ {
		w := width
		for {
			cwidth := fontMetrics.HorizontalAdvance(string(r[k]), -1)
			if cwidth <= w { break }
			w += width
		}
		wSlice = append(wSlice, w)
	}

	i.widthSlice = wSlice
}

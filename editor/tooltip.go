package editor

import (
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/widgets"
)

// Tooltip is the tooltip
type Tooltip struct {
	widgets.QWidget

	s          *Screen
	text       string
	font       *Font
	widthSlice []float64
}

func (t *Tooltip) getNthWidthAndShift(n int) (float64, int) {
	if t.font == nil {
		return 0.0, 0
	}
	if len(t.widthSlice) <= n {
		return 0.0, 0
	}

	var width float64
	var shift int

	width = t.widthSlice[n]
	shift = t.font.shift

	return width, shift
}

func (t *Tooltip) drawForeground(p *gui.QPainter, f func(*gui.QPainter), g func(int) (float64, int)) {
	f(p)

	p.SetPen2(t.s.ws.foreground.QColor())

	if t.text != "" {
		r := []rune(t.text)
		var x float64
		for k := 0; k < len(r); k++ {

			width, shift := g(k)
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

func (t *Tooltip) setQpainterFont(p *gui.QPainter) {
	p.SetFont(t.font.fontNew)
}

func (t *Tooltip) paint(event *gui.QPaintEvent) {
	p := gui.NewQPainter2(t)

	t.drawForeground(p, t.setQpainterFont, t.getNthWidthAndShift)

	p.DestroyQPainter()
}

func (t *Tooltip) move(x int, y int) {
	t.Move(core.NewQPoint2(x, y))
}

func (t *Tooltip) setFont(font *Font) {
	t.font = font
}

func (t *Tooltip) updateText(text string) {
	if t.font == nil {
		return
	}
	font := t.font

	// update text in struct
	t.text = text

	// rune text
	r := []rune(t.text)

	// init slice
	var wSlice []float64
	wSlice = append(wSlice, 0.0)

	for k := 0; k < len(r); k++ {
		w := font.truewidth
		for {
			cwidth := font.fontMetrics.HorizontalAdvance(string(r[k]), -1)
			if cwidth <= w {
				break
			}
			w += font.truewidth
		}
		wSlice = append(wSlice, w)
	}

	t.widthSlice = wSlice
}

func (t *Tooltip) show() {
	if t.s != nil {
		t.SetAutoFillBackground(true)
		p := gui.NewQPalette()
		p.SetColor2(gui.QPalette__Background, t.s.ws.background.QColor())
		t.SetPalette(p)
	}

	t.Show()
	t.Raise()
}

func (t *Tooltip) update() {
	// detect width
	var tooltipWidth float64
	if len(t.widthSlice) == 0 {
		return
	}
	for _, w := range t.widthSlice {
		tooltipWidth += w
	}

	// update widget size
	t.SetFixedSize2(
		int(tooltipWidth),
		t.font.lineHeight,
	)

	t.Update()
}

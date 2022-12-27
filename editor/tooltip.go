package editor

import (
	"bytes"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/widgets"
)

type ColorStr struct {
	hl    *Highlight
	str   string
	scale int
}

// Tooltip is the tooltip
type Tooltip struct {
	widgets.QWidget

	s          *Screen
	text       []*ColorStr
	font       *Font
	widthSlice []float64
}

func (t *Tooltip) drawContent(p *gui.QPainter, f func(*gui.QPainter)) {
	f(p)
	font := p.Font()

	p.SetPen2(t.s.ws.foreground.QColor())

	if t.text == nil {
		return
	}

	height := float64(t.s.tooltip.font.height) * 1.1
	lineHeight := float64(t.s.tooltip.font.lineHeight)
	if height > float64(lineHeight) {
		height = float64(lineHeight)
	}
	shift := t.font.shift
	var x float64
	var y float64 = float64(lineHeight-height) / 2

	for _, chunk := range t.text {

		fg := chunk.hl.fg()
		bg := chunk.hl.bg()
		// bold := chunk.hl.bold
		underline := chunk.hl.underline
		// undercurl := chunk.hl.undercurl
		// strikethrough := chunk.hl.strikethrough
		italic := chunk.hl.italic

		// set foreground color
		p.SetPen2(fg.QColor())

		r := []rune(chunk.str)
		for _, rr := range r {
			// draw background
			if !bg.equals(t.s.ws.background) {
				p.FillRect4(
					core.NewQRectF4(
						x,
						y,
						float64(chunk.scale)*t.font.cellwidth,
						height,
					),
					bg.QColor(),
				)
			}

			// set italic
			if italic {
				font.SetItalic(true)
			} else {
				font.SetItalic(false)
			}

			p.DrawText(
				core.NewQPointF3(
					float64(x),
					float64(shift),
				),
				string(rr),
			)

			// draw underline
			if underline {
				var underlinePos float64 = 1
				if t.s.ws.palette.widget.IsVisible() {
					underlinePos = 2
				}

				// draw underline
				p.FillRect4(
					core.NewQRectF4(
						x,
						y+height-underlinePos,
						float64(chunk.scale)*t.font.cellwidth,
						underlinePos,
					),
					fg.QColor(),
				)
			}

			x += float64(chunk.scale) * t.font.cellwidth
		}
	}
}

func (t *Tooltip) setQpainterFont(p *gui.QPainter) {
	if p == nil {
		return
	}
	if t.font == nil {
		return
	}
	p.SetFont(t.font.fontNew)
}

func (t *Tooltip) paint(event *gui.QPaintEvent) {
	p := gui.NewQPainter2(t)

	t.drawContent(p, t.setQpainterFont)

	p.DestroyQPainter()
}

func (t *Tooltip) move(x int, y int) {
	t.Move(core.NewQPoint2(x, y))
}

func (t *Tooltip) setFont(font *Font) {
	t.font = font
}

func (t *Tooltip) clearText() {
	var newText []*ColorStr
	t.text = newText
}

func (t *Tooltip) updateText(hl *Highlight, str string) {
	if t.font == nil {
		return
	}

	font := t.font

	// rune text
	r := []rune(str)

	var preScale int
	var preStrWidth float64
	var buffer bytes.Buffer
	for k, rr := range r {

		// detect char width based cell width
		w := font.cellwidth
		scale := 1
		for {
			cwidth := font.fontMetrics.HorizontalAdvance(string(rr), -1)
			if cwidth <= w {
				break
			}
			w += font.cellwidth
			scale++
		}
		if preStrWidth == 0 {
			preStrWidth = w
			preScale = scale
		}

		if preStrWidth == w {
			buffer.WriteString(string(rr))
			if k < len(r)-1 {
				continue
			}
		}

		if buffer.Len() != 0 {

			t.text = append(t.text, &ColorStr{
				hl:  hl,
				str: buffer.String(),
				// width: preStrWidth,
				scale: preScale,
			})

			buffer.Reset()
			buffer.WriteString(string(rr))

			if preStrWidth != w && k == len(r)-1 {
				t.text = append(t.text, &ColorStr{
					hl:  hl,
					str: buffer.String(),
					// width: w,
					scale: scale,
				})
			}

			preStrWidth = w
			preScale = scale
		}

	}
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
	if len(t.text) == 0 {
		return
	}
	for _, chunk := range t.text {
		r := []rune(chunk.str)
		for _, _ = range r {
			tooltipWidth += float64(chunk.scale) * t.font.cellwidth
		}
	}

	font := t.font
	if font == nil {
		font = t.s.ws.font
	}

	// update widget size
	t.SetFixedSize2(
		int(tooltipWidth),
		t.font.lineHeight,
	)

	t.SetAutoFillBackground(true)
	p := gui.NewQPalette()
	p.SetColor2(gui.QPalette__Background, t.s.ws.background.QColor())
	t.SetPalette(p)
	t.Update()
}

package editor

import (
	"bytes"

	"github.com/akiyosi/qt/core"
	"github.com/akiyosi/qt/gui"
	"github.com/akiyosi/qt/widgets"
)

type ColorStr struct {
	hl    *Highlight
	str   string
	width float64
}

// Tooltip is the tooltip
type Tooltip struct {
	widgets.QWidget

	s             *Screen
	text          []*ColorStr
	font          *Font
	fallbackfonts []*Font
	widthSlice    []float64
}

func resolveMetricsInFontFallback(font *Font, fallbackfonts []*Font, char string) float64 {
	if len(fallbackfonts) == 0 {
		return font.fontMetrics.HorizontalAdvance(char, -1)
	}

	hasGlyph := font.hasGlyph(char)
	if hasGlyph {
		return font.fontMetrics.HorizontalAdvance(char, -1)
	} else {
		for _, ff := range fallbackfonts {
			hasGlyph = ff.hasGlyph(char)
			if hasGlyph {
				return ff.fontMetrics.HorizontalAdvance(char, -1)
			}
		}
	}

	return font.fontMetrics.HorizontalAdvance(char, -1)
}

func (t *Tooltip) drawContent(p *gui.QPainter, getFont func() *Font) {
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
		sp := chunk.hl.special
		if sp == nil {
			sp = fg
		}

		bold := chunk.hl.bold
		italic := chunk.hl.italic
		underline := chunk.hl.underline
		undercurl := chunk.hl.undercurl
		underdouble := chunk.hl.underdouble
		underdotted := chunk.hl.underdotted
		underdashed := chunk.hl.underdashed
		strikethrough := chunk.hl.strikethrough

		// set foreground color
		p.SetPen2(fg.QColor())

		r := []rune(chunk.str)
		for _, rr := range r {
			p.SetFont(resolveFontFallback(getFont(), t.fallbackfonts, string(rr)).qfont)
			font := p.Font()

			// draw background
			p.FillRect4(
				core.NewQRectF4(
					x,
					y,
					chunk.width,
					height,
				),
				bg.QColor(),
			)

			// set italic, bold
			font.SetItalic(italic)
			font.SetBold(bold)

			p.DrawText(
				core.NewQPointF3(
					float64(x),
					float64(shift),
				),
				string(rr),
			)

			//
			// set text decoration
			//

			if strikethrough {
				drawStrikethrough(p, t.font, sp.QColor(), 0, x, x+chunk.width, 0, 0)
			}

			if underline {
				drawUnderline(p, t.font, sp.QColor(), 0, x, x+chunk.width, 0, 0)
			}

			if undercurl {
				drawUndercurl(p, t.font, sp.QColor(), 0, x, x+chunk.width, 0, 0)
			}

			if underdouble {
				drawUnderdouble(p, t.font, sp.QColor(), 0, x, x+chunk.width, 0, 0)
			}

			if underdotted {
				drawUnderdotted(p, t.font, sp.QColor(), 0, x, x+chunk.width, 0, 0)
			}

			if underdashed {
				drawUnderdashed(p, t.font, sp.QColor(), 0, x, x+chunk.width, 0, 0)
			}

			x += chunk.width
		}
	}
}

func (t *Tooltip) getFont() *Font {
	return t.font
}

func (t *Tooltip) paint(event *gui.QPaintEvent) {
	p := gui.NewQPainter2(t)

	t.drawContent(p, t.getFont)

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

func (t *Tooltip) updateText(hl *Highlight, str string, letterspace int, font *gui.QFont) {
	if t.font == nil {
		return
	}

	fontMetrics := gui.NewQFontMetricsF(font)
	cellwidth := fontMetrics.HorizontalAdvance("w", -1) + float64(letterspace)

	// rune text
	r := []rune(str)

	var preStrWidth float64
	var buffer bytes.Buffer
	for k, rr := range r {

		// detect char width based cell width
		w := cellwidth
		for {
			// cwidth := fontMetrics.HorizontalAdvance(string(rr), -1)
			cwidth := resolveMetricsInFontFallback(t.font, t.fallbackfonts, string(rr))
			if cwidth <= w {
				break
			}
			w += cellwidth
		}
		if preStrWidth == 0 {
			preStrWidth = w
		}

		if preStrWidth == w {
			buffer.WriteString(string(rr))
			if k < len(r)-1 {
				continue
			}
		}

		if buffer.Len() != 0 {

			t.text = append(t.text, &ColorStr{
				hl:    hl,
				str:   buffer.String(),
				width: preStrWidth,
			})

			buffer.Reset()
			buffer.WriteString(string(rr))

			if preStrWidth != w && k == len(r)-1 {
				t.text = append(t.text, &ColorStr{
					hl:    hl,
					str:   buffer.String(),
					width: w,
				})
			}

			preStrWidth = w
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
			tooltipWidth += chunk.width
		}
	}

	// update widget size
	t.SetFixedSize2(
		int(tooltipWidth),
		t.font.lineHeight,
	)

	t.Update()
}

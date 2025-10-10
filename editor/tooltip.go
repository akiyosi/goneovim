package editor

import (
	"bytes"
	"math"
	"strings"

	"github.com/akiyosi/qt/core"
	"github.com/akiyosi/qt/gui"
	"github.com/akiyosi/qt/widgets"
)

type ColorStr struct {
	hl        *Highlight
	str       string
	width     float64
	cellwidth float64
}

// Tooltip is the tooltip
type Tooltip struct {
	widgets.QWidget

	s             *Screen
	text          []*ColorStr
	font          *Font
	fallbackfonts []*Font
	widthSlice    []float64

	textWidth  float64
	msgHeight  float64
	background *RGBA

	pathWidth float64
	xPadding  float64
	yPadding  float64
	xRadius   float64
	yRadius   float64

	maximumWidth int
	fixedWidth   bool

	backgrond *RGBA
}

func (t *Tooltip) drawContent(p *gui.QPainter, getFont func() *Font) {

	if t.text == nil {
		return
	}

	screenWidth := t.s.widget.Width()
	if t.maximumWidth == 0 {
		t.maximumWidth = screenWidth
	}
	height := float64(t.font.height) * 1.1
	lineHeight := float64(t.font.lineHeight)
	if height > float64(lineHeight) {
		height = float64(lineHeight)
	}
	shift := t.font.shift
	var x float64
	var y float64 = float64(lineHeight-height) / 2

	color := t.background
	if color == nil {
		if t.s != nil {
			color = t.s.ws.background
		}
	}

	if t.xRadius == 0 && t.yRadius == 0 {
		p.FillRect4(
			core.NewQRectF4(
				0,
				0,
				t.textWidth+t.xPadding*2.0,
				t.msgHeight+t.yPadding*2.0,
			),
			color.QColor(),
		)

	} else {

		outlineColor := warpColor(color, -20)
		p.SetPen(
			gui.NewQPen4(
				gui.NewQBrush3(
					gui.NewQColor3(
						outlineColor.R,
						outlineColor.G,
						outlineColor.B,
						255,
					),
					core.Qt__BrushStyle(1),
				),
				t.pathWidth,
				core.Qt__SolidLine,
				core.Qt__FlatCap,
				core.Qt__MiterJoin,
			),
		)

		width := t.textWidth + t.xPadding*2.0
		if width > float64(t.maximumWidth)-t.pathWidth*2 {
			width = float64(t.maximumWidth) - t.pathWidth*2
		}
		if t.fixedWidth {
			width = float64(t.maximumWidth) - t.pathWidth*2
		}

		path := gui.NewQPainterPath()
		path.AddRoundedRect(
			core.NewQRectF4(
				t.pathWidth,
				t.pathWidth,
				width,
				t.msgHeight+t.yPadding*2.0,
			),
			t.xRadius,
			t.yRadius,
			core.Qt__AbsoluteSize,
		)
		p.DrawPath(path)

		p.FillPath(
			path,
			gui.NewQBrush3(
				gui.NewQColor3(
					color.R,
					color.G,
					color.B,
					255,
				),
				core.Qt__BrushStyle(1),
			),
		)

	}

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

		for i, s := range strings.Split(chunk.str, "\n") {
			if i > 0 {
				x = 0
				y += lineHeight
			}

			// set foreground color
			p.SetPen2(fg.QColor())

			r := []rune(s)

			for _, rr := range r {

				if x+t.xPadding*2 >= float64(t.maximumWidth) {
					x = 0
					y += lineHeight
				}

				p.SetFont(resolveFontFallback(getFont(), t.fallbackfonts, string(rr)).qfont)
				font := p.Font()
				cwidth := gui.NewQFontMetricsF(t.font.qfont).HorizontalAdvance(string(rr), -1)
				w := chunk.cellwidth
				for {
					if cwidth <= w {
						break
					}
					w += chunk.cellwidth
				}

				// draw background
				p.FillRect4(
					core.NewQRectF4(
						x+t.xPadding,
						y+t.yPadding,
						w,
						height,
					),
					bg.QColor(),
				)

				// set italic, bold
				font.SetItalic(italic)
				font.SetBold(bold)

				p.DrawText(
					core.NewQPointF3(
						x+t.xPadding,
						y+float64(shift)+t.yPadding,
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

				x += w
			}
		}
	}
}

func (t *Tooltip) getFont() *Font {
	return t.font
}

func (t *Tooltip) setBgColor(color *RGBA) {
	t.background = color
}

func (t *Tooltip) setPadding(xv, yv float64) {
	t.xPadding = xv
	t.yPadding = yv
}

func (t *Tooltip) setRadius(xr, yr float64) {
	t.xRadius = xr
	t.yRadius = yr
}

func (t *Tooltip) setQpainterFont(p *gui.QPainter) {
	p.SetFont(t.font.qfont)
}

func (t *Tooltip) paint(event *gui.QPaintEvent) {
	p := gui.NewQPainter2(t)

	t.drawContent(p, t.getFont)

	p.DestroyQPainter()
}

func (t *Tooltip) setFont(font *Font) {
	t.font = font
}

func (t *Tooltip) clearText() {
	var newText []*ColorStr
	t.text = newText
}

// func (t *Tooltip) updateText(hl *Highlight, str string, letterspace float64, font *gui.QFont) {
func (t *Tooltip) updateText(hl *Highlight, str string, letterspace float64, font *gui.QFont) {
	if t.font == nil {
		return
	}

	fontMetrics := gui.NewQFontMetricsF(font)
	cellwidth := fontMetrics.HorizontalAdvance("w", -1) + letterspace

	// rune text
	r := []rune(str)

	var width float64
	var maxWidth float64
	var widthList []float64
	var buffer bytes.Buffer
	for _, rr := range r {
		buffer.WriteString(string(rr))

		// detect char width based cell width
		w := cellwidth
		cwidth := fontMetrics.HorizontalAdvance(string(rr), -1)

		if string(rr) == "\n" {
			widthList = append(widthList, width)
			width = 0
			continue
		}

		for {
			if cwidth <= w {
				break
			}
			w += cellwidth
		}

		width += w
	}
	widthList = append(widthList, width)

	for _, v := range widthList {
		if maxWidth < v {
			maxWidth = v
		}
	}

	t.text = append(t.text, &ColorStr{
		hl:        hl,
		str:       buffer.String(),
		width:     maxWidth,
		cellwidth: cellwidth,
	})

}

func (t *Tooltip) show() {
	if t == nil {
		return
	}

	t.Show()
	t.Raise()
}

func (t *Tooltip) update() {
	// detect width
	var textWidth float64
	var msgHeight float64
	screenWidth := t.s.widget.Width()
	screenHeight := t.s.widget.Height()

	if t.maximumWidth == 0 {
		t.maximumWidth = screenWidth
	}

	if len(t.text) == 0 {
		return
	}
	var width float64
	var height int
	for i, chunk := range t.text {
		if i == 0 {
			msgHeight += float64(t.font.lineHeight)
		}

		width += chunk.width
		for j, s := range strings.Split(chunk.str, "\n") {
			if j > 0 {
				msgHeight += float64(t.font.lineHeight)
				width = 0
			}
			r := []rune(s)
			for k, _ := range r {
				if width > float64(t.maximumWidth)-t.xPadding*2-t.pathWidth*2 {
					height = int(math.Floor(width)) / int(math.Ceil((float64(t.maximumWidth) - t.xPadding*2 - t.pathWidth*2)))
					residue := int(math.Floor(width)) % int(math.Ceil((float64(t.maximumWidth) - t.xPadding*2 - t.pathWidth*2)))
					if height > 0 && residue == 0 {
						height -= 1
					}
				} else if textWidth < width {
					textWidth = width
				}

				if k == len(r)-1 && textWidth < width {
					textWidth = width
				}

			}

		}

	}
	if height > 0 {
		msgHeight += float64(height * t.font.lineHeight)
	}

	t.textWidth = textWidth
	t.msgHeight = msgHeight

	var tooltipwidth float64
	if t.fixedWidth {
		tooltipwidth = float64(t.maximumWidth) + t.pathWidth*2
	} else {
		tooltipwidth = t.textWidth + t.xPadding*2.0 + t.pathWidth*2
		if tooltipwidth > float64(t.maximumWidth) {
			tooltipwidth = float64(t.maximumWidth) + t.pathWidth*2
		}
	}

	tooltipheight := t.msgHeight + t.yPadding*2.0 + t.pathWidth*2
	if tooltipheight > float64(screenHeight) {
		tooltipheight = float64(screenHeight)
	}

	// update widget size
	t.SetFixedSize2(
		int(tooltipwidth),
		int(tooltipheight),
	)

	t.Update()
}

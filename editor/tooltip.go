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

	textWidth  float64
	msgHeight  float64
	background *RGBA

	pathWidth float64
	xPadding  float64
	yPadding  float64
	xMargin   float64
	yMargin   float64
	xRadius   float64
	yRadius   float64

	maximumWidth int
	fixedWidth   bool

	backgrond *RGBA
}

func (t *Tooltip) drawContent(p *gui.QPainter, getFont func() *Font) {
	// setFont(p)
	p.SetFont(getFont().qfont)
	font := p.Font()

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
		if t.fixedWidth {
			width = float64(t.maximumWidth) + t.xPadding*2.0
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

		for i, s := range strings.Split(chunk.str, "\n") {
			if i > 0 {
				x = 0
				y += lineHeight
			}

			fg := chunk.hl.fg()
			bg := chunk.hl.bg()
			// bold := chunk.hl.bold
			underline := chunk.hl.underline
			// undercurl := chunk.hl.undercurl
			// strikethrough := chunk.hl.strikethrough
			italic := chunk.hl.italic

			// set foreground color
			p.SetPen2(fg.QColor())

			r := []rune(s)
			for _, rr := range r {
				if x+t.xPadding*2 >= float64(t.maximumWidth-int(t.xMargin)*2) {
					x = 0
					y += lineHeight
				}
				// draw background
				p.FillRect4(
					core.NewQRectF4(
						x+t.xPadding,
						y+t.yPadding,
						chunk.width,
						height,
					),
					bg.QColor(),
				)

				// set italic
				if italic {
					font.SetItalic(true)
				} else {
					font.SetItalic(false)
				}

				p.DrawText(
					core.NewQPointF3(
						x+t.xPadding,
						y+float64(shift)+t.yPadding,
					),
					string(rr),
				)

				// draw underline
				if underline {
					var underlinePos float64 = 1
					if t.s.ws.palette != nil && t.s.ws.palette.widget.IsVisible() {
						underlinePos = 2
					}

					// draw underline
					p.FillRect4(
						core.NewQRectF4(
							x+t.xPadding,
							y+height-underlinePos+t.yPadding,
							chunk.width,
							underlinePos,
						),
						fg.QColor(),
					)
				}

				x += chunk.width
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

func (t *Tooltip) setMargin(xv, yv float64) {
	t.xMargin = xv
	t.yMargin = yv
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

// func (t *Tooltip) updateText(hl *Highlight, str string, letterspace float64, font *gui.QFont) {
func (t *Tooltip) updateText(hl *Highlight, str string, letterspace float64, font *gui.QFont) {
	if t.font == nil {
		return
	}

	fontMetrics := gui.NewQFontMetricsF(font)
	cellwidth := fontMetrics.HorizontalAdvance("w", -1) + letterspace

	// rune text
	r := []rune(str)

	var preStrWidth float64
	var buffer bytes.Buffer
	for k, rr := range r {

		// detect char width based cell width
		w := cellwidth
		for {

			cwidth := fontMetrics.HorizontalAdvance(string(rr), -1)
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

		for j, s := range strings.Split(chunk.str, "\n") {
			if j > 0 {
				msgHeight += float64(t.font.lineHeight)
				width = 0
			}
			r := []rune(s)
			for k, _ := range r {
				if width > float64(t.maximumWidth)-t.xMargin*2-t.xPadding*2-t.pathWidth*2 {
					height = int(math.Floor(width)) / int(math.Ceil((float64(t.maximumWidth) - t.xMargin*2 - t.xPadding*2 - t.pathWidth*2)))
					residue := int(math.Floor(width)) % int(math.Ceil((float64(t.maximumWidth) - t.xMargin*2 - t.xPadding*2 - t.pathWidth*2)))
					if height > 0 && residue == 0 {
						height -= 1
					}
				} else if textWidth < width {
					textWidth = width
				}

				width += chunk.width
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
		tooltipwidth = float64(t.maximumWidth) - t.xMargin*2 + t.pathWidth*2
	} else {
		tooltipwidth = t.textWidth + t.xPadding*2.0 + t.pathWidth*2
		if tooltipwidth > float64(t.maximumWidth)-t.xMargin*2 {
			tooltipwidth = float64(t.maximumWidth) - t.xMargin*2 + t.pathWidth*2
		}
	}

	tooltipheight := t.msgHeight + t.yPadding*2.0 + t.pathWidth*2
	if tooltipheight > float64(screenHeight)-t.yMargin*2 {
		tooltipheight = float64(screenHeight) - t.yMargin*2
	}

	// update widget size
	t.SetFixedSize2(
		int(tooltipwidth),
		int(tooltipheight),
	)

	t.Update()
}

package gonvim

import (
	"github.com/dzhou121/ui"
)

// SpanHandler is
type SpanHandler struct {
	AreaHandler
	text          string
	bg            *RGBA
	color         *RGBA
	font          *ui.Font
	paddingLeft   int
	paddingRight  int
	paddingTop    int
	paddingBottom int
}

// Draw the span
func (s *SpanHandler) Draw(a *ui.Area, dp *ui.AreaDrawParams) {
	if s.bg == nil {
		return
	}
	bg := s.bg
	p := ui.NewPath(ui.Winding)
	p.AddRectangle(dp.ClipX, dp.ClipY, dp.ClipWidth, dp.ClipHeight)
	p.End()

	bottomBg := newRGBA(14, 17, 18, 1)
	dp.Context.Fill(p, &ui.Brush{
		Type: ui.Solid,
		R:    bottomBg.R,
		G:    bottomBg.G,
		B:    bottomBg.B,
		A:    bottomBg.A,
	})
	dp.Context.Fill(p, &ui.Brush{
		Type: ui.Solid,
		R:    bg.R,
		G:    bg.G,
		B:    bg.B,
		A:    bg.A,
	})
	p.Free()

	textLayout := s.getTextLayout()
	dp.Context.Text(
		float64(s.paddingLeft),
		float64(s.paddingTop),
		textLayout,
	)
	textLayout.Free()
}

// SetColor sets the color
func (s *SpanHandler) SetColor(rgba *RGBA) {
	s.color = rgba
}

// SetBackground sets the color
func (s *SpanHandler) SetBackground(rgba *RGBA) {
	s.bg = rgba
}

// SetFont sets the font
func (s *SpanHandler) SetFont(font *ui.Font) {
	s.font = font
}

// SetText sets the text
func (s *SpanHandler) SetText(text string) {
	s.text = text
}

func (s *SpanHandler) getTextLayout() *ui.TextLayout {
	textLayout := ui.NewTextLayout(s.text, s.font, -1)
	fg := s.color
	textLayout.SetColor(0, len(s.text), fg.R, fg.G, fg.B, fg.A)
	return textLayout
}

func (s *SpanHandler) getSize() (int, int) {
	width := editor.fontWidth*len(s.text) + s.paddingLeft + s.paddingRight
	height := editor.fontHeight + s.paddingTop + s.paddingBottom
	return width, height
}

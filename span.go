package gonvim

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dzhou121/ui"
)

// Border is the border of span
type Border struct {
	color *RGBA
	width int
}

// SpanHandler is
type SpanHandler struct {
	AreaHandler
	match         string
	matchColor    *RGBA
	matchIndex    []int
	text          string
	bg            *RGBA
	color         *RGBA
	font          *ui.Font
	paddingLeft   int
	paddingRight  int
	paddingTop    int
	paddingBottom int
	textType      string
	borderTop     *Border
	borderRight   *Border
	borderLeft    *Border
	borderBottom  *Border
	width         int
	height        int
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

	s.drawBorder(dp)

	if s.text == "" {
		return
	}
	textLayout := s.getTextLayout()
	dp.Context.Text(
		float64(s.paddingLeft),
		float64(s.paddingTop),
		textLayout,
	)
	textLayout.Free()
}

func (s *SpanHandler) drawBorder(dp *ui.AreaDrawParams) {
	if s.borderBottom != nil {
		s.drawRect(dp, 0, s.height-s.borderBottom.width, s.width, s.borderBottom.width, s.borderBottom.color)
	}
	if s.borderRight != nil {
		s.drawRect(dp, s.width-s.borderRight.width, 0, s.borderRight.width, s.height, s.borderRight.color)
	}
}

func (s *SpanHandler) drawRect(dp *ui.AreaDrawParams, x, y, width, height int, color *RGBA) {
	p := ui.NewPath(ui.Winding)
	p.AddRectangle(float64(x), float64(y), float64(width), float64(height))
	p.End()
	dp.Context.Fill(p, &ui.Brush{
		Type: ui.Solid,
		R:    color.R,
		G:    color.G,
		B:    color.B,
		A:    color.A,
	})
	p.Free()
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
	text := s.text
	matchIndex := s.matchIndex
	var textLayout *ui.TextLayout
	shift := map[int]int{}
	indent := 0
	if s.textType == "file" || s.textType == "dir" || s.textType == "ag_file" {
		dir := filepath.Dir(s.text)
		if dir == "." {
			dir = ""
		}

		base := filepath.Base(s.text)
		if dir != "" {
			i := strings.Index(s.text, dir)
			if i != -1 {
				for j := range dir {
					shift[j+i] = len(base) + 1 + j
				}
			}
		}
		if base != "" {
			i := strings.LastIndex(s.text, base)
			if i != -1 {
				for j := range base {
					shift[j+i] = j
				}
			}
		}

		baseLen := len(base)

		text = fmt.Sprintf("%s %s", base, dir)
		if s.textType == "ag_file" {
			text = "- " + text
			baseLen += 2
			indent = 2
		}
		textLayout = ui.NewTextLayout(text, s.font, -1)
		fg := newRGBA(131, 131, 131, 1)
		textLayout.SetColor(0, len(text), fg.R, fg.G, fg.B, fg.A)
		fg = s.color
		textLayout.SetColor(0, baseLen, fg.R, fg.G, fg.B, fg.A)
	} else if s.textType == "line" {
		i := strings.Index(s.text, "\t")
		textLayout = ui.NewTextLayout(text, s.font, -1)
		fg := s.color
		textLayout.SetColor(0, len(text), fg.R, fg.G, fg.B, fg.A)

		fg = newRGBA(131, 131, 131, 1)
		textLayout.SetColor(0, i, fg.R, fg.G, fg.B, fg.A)
	} else if s.textType == "ag_line" {
		text = "    " + text
		indent = 4
		textLayout = ui.NewTextLayout(text, s.font, -1)
		fg := s.color
		textLayout.SetColor(0, len(text), fg.R, fg.G, fg.B, fg.A)
	} else {
		textLayout = ui.NewTextLayout(text, s.font, -1)
		fg := s.color
		textLayout.SetColor(0, len(text), fg.R, fg.G, fg.B, fg.A)
	}

	if s.matchColor != nil {
		if len(matchIndex) > 0 {
			for _, i := range matchIndex {
				j, ok := shift[i]
				if ok {
					i = j
				}
				textLayout.SetColor(i+indent, i+indent+1, s.matchColor.R, s.matchColor.G, s.matchColor.B, s.matchColor.A)
			}
		} else if s.match != "" {
			for _, c := range s.match {
				i := strings.Index(text, string(c))
				if i != -1 {
					textLayout.SetColor(i, i+1, s.matchColor.R, s.matchColor.G, s.matchColor.B, s.matchColor.A)
				}
			}
		}
	}
	return textLayout
}

func (s *SpanHandler) setSize(width, height int) {
	s.width = width
	s.height = height
	ui.QueueMain(func() {
		s.span.SetSize(width, height)
	})
}

func (s *SpanHandler) getSize() (int, int) {
	width := editor.font.width*len(s.text) + s.paddingLeft + s.paddingRight
	height := editor.font.height + s.paddingTop + s.paddingBottom
	return width, height
}

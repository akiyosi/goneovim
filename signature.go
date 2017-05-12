package gonvim

import (
	"strings"

	"github.com/dzhou121/ui"
)

// Signature is
type Signature struct {
	box   *ui.Box
	span  *SpanHandler
	cusor []int
	comma int
}

func initSignature() *Signature {
	box := ui.NewHorizontalBox()
	handler := &SpanHandler{}
	span := ui.NewArea(handler)
	handler.area = span
	box.Append(span, false)
	return &Signature{
		box:   box,
		span:  handler,
		cusor: []int{0, 0},
	}
}

func (s *Signature) show(args []interface{}) {
	text := args[0].(string)
	cursor := args[1].([]interface{})
	s.comma = reflectToInt(args[2])
	s.cusor[0] = reflectToInt(cursor[0])
	s.cusor[1] = reflectToInt(cursor[1])
	font := editor.font

	s.span.SetFont(font)
	s.span.SetText(text)
	s.span.setSize(s.span.getSize())
	s.span.SetBackground(newRGBA(30, 30, 30, 1))
	s.span.SetColor(newRGBA(205, 211, 222, 1))
	border := &Border{
		width: 1,
		color: newRGBA(0, 0, 0, 1),
	}
	s.span.borderBottom = border
	s.span.borderTop = border
	s.span.borderLeft = border
	s.span.borderRight = border
	s.span.paddingTop = font.shift * 2
	s.span.paddingLeft = int(font.truewidth)
	s.span.paddingRight = s.span.paddingLeft
	s.span.paddingBottom = s.span.paddingTop
	s.span.setSize(s.span.getSize())
	s.move()
	s.underline()
	ui.QueueMain(func() {
		s.box.Show()
		s.box.SetSize(s.span.getSize())
	})
}

func (s *Signature) pos(args []interface{}) {
	s.comma = reflectToInt(args[0])
	s.underline()
}

func (s *Signature) underline() {
	text := s.span.text
	left := strings.Index(text, "(")
	right := strings.Index(text, ")")
	n := 0
	i := left + 1
	start := i
	for ; i < right; i++ {
		if string(text[i]) == "," {
			n++
			if n > s.comma {
				break
			}
			start = i
		}
	}
	for ; start < i; start++ {
		t := string(text[start])
		if t == "," || t == " " {
			continue
		} else {
			break
		}
	}
	i--
	s.span.underline = []int{}
	for j := start; j <= i; j++ {
		s.span.underline = append(s.span.underline, j)
	}
	ui.QueueMain(func() {
		s.span.area.QueueRedrawAll()
	})
}

func (s *Signature) move() {
	row := editor.screen.cursor[0] + s.cusor[0]
	col := editor.screen.cursor[1] + s.cusor[1]
	_, h := s.span.getSize()
	i := strings.Index(s.span.text, "(")
	if i >= 0 {
		col -= i
	}
	ui.QueueMain(func() {
		s.box.SetPosition(int(float64(col)*editor.font.truewidth), row*editor.font.lineHeight-h)
	})
}

func (s *Signature) hide() {
	ui.QueueMain(func() {
		s.box.Hide()
	})
}

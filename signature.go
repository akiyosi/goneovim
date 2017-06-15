package gonvim

import (
	"fmt"
	"strings"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/widgets"
)

// Signature is
type Signature struct {
	cusor         []int
	comma         int
	formattedText string
	text          string
	widget        *widgets.QWidget
	label         *widgets.QLabel
	height        int
	x             int
	y             int
}

func initSignature() *Signature {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(8, 8, 8, 8)
	widget.SetStyleSheet(`
	.QWidget {
		border: 1px solid #000;
	}
	QWidget {
		background-color: rgba(30, 30, 30, 1);
	}
	* {
		color: rgba(205, 211, 222, 1);
	}
	`)
	layout := widgets.NewQVBoxLayout()
	layout.SetContentsMargins(0, 0, 0, 0)

	label := widgets.NewQLabel(nil, 0)
	layout.AddWidget(label, 0, 0)
	widget.SetLayout(layout)
	widget.Hide()
	signature := &Signature{
		cusor:  []int{0, 0},
		widget: widget,
		label:  label,
		height: widget.SizeHint().Height(),
	}
	widget.ConnectCustomEvent(func(event *core.QEvent) {
		switch event.Type() {
		case core.QEvent__Move:
			widget.Move2(signature.x, signature.y)
		case core.QEvent__Show:
			widget.Show()
		case core.QEvent__Hide:
			widget.Hide()
		}
	})
	label.ConnectCustomEvent(func(event *core.QEvent) {
		switch event.Type() {
		case core.QEvent__UpdateRequest:
			label.SetText(signature.formattedText)
		}
	})
	return signature
}

func (s *Signature) showItem(args []interface{}) {
	text := args[0].(string)
	s.text = text
	cursor := args[1].([]interface{})
	s.comma = reflectToInt(args[2])
	s.cusor[0] = reflectToInt(cursor[0])
	s.cusor[1] = reflectToInt(cursor[1])
	s.update()
	s.move()
	s.hide()
	s.show()
}

func (s *Signature) pos(args []interface{}) {
	s.comma = reflectToInt(args[0])
	s.update()
	// s.underline()
}

func (s *Signature) update() {
	text := s.text
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
	formattedText := fmt.Sprintf("%s<font style=\"text-decoration:underline\";>%s</font>%s", text[:start], text[start:i], text[i:])
	s.formattedText = formattedText
	s.label.CustomEvent(core.NewQEvent(core.QEvent__UpdateRequest))
}

func (s *Signature) move() {
	text := s.text
	row := editor.screen.cursor[0] + s.cusor[0]
	col := editor.screen.cursor[1] + s.cusor[1]
	i := strings.Index(text, "(")
	x := float64(col) * editor.font.truewidth
	if i > -1 {
		x -= editor.font.defaultFontMetrics.Width(string(text[:i]))
	}
	s.x = int(x)
	s.y = row*editor.font.lineHeight - s.height
	s.widget.CustomEvent(core.NewQEvent(core.QEvent__Move))
}

func (s *Signature) show() {
	s.widget.CustomEvent(core.NewQEvent(core.QEvent__Show))
}

func (s *Signature) hide() {
	s.widget.CustomEvent(core.NewQEvent(core.QEvent__Hide))
}

package editor

import (
	"fmt"
	"strings"

	"github.com/akiyosi/goneovim/util"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

// Signature is
type Signature struct {
	ws            *Workspace
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
	layout := widgets.NewQHBoxLayout()
	layout.SetContentsMargins(0, 0, 0, 0)

	icon := svg.NewQSvgWidget(nil)
	icon.SetFixedWidth(editor.iconSize)
	icon.SetFixedHeight(editor.iconSize)
	icon.SetContentsMargins(0, 0, 0, 0)
	svgContent := editor.getSvg("flag", hexToRGBA(editor.config.SideBar.AccentColor))
	icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))

	label := widgets.NewQLabel(nil, 0)

	layout.AddWidget(icon, 0, 0)
	layout.AddWidget(label, 0, 0)

	widget.SetLayout(layout)
	signature := &Signature{
		cusor:  []int{0, 0},
		widget: widget,
		label:  label,
	}

	widget.SetGraphicsEffect(util.DropShadow(-2, 6, 40, 200))

	return signature
}

func (s *Signature) setColor() {
	fg := editor.colors.widgetFg.String()
	bg := editor.colors.widgetBg.String()
	s.widget.SetStyleSheet(fmt.Sprintf(".QWidget { border: 1px solid %s; } QWidget { background-color: %s; } * { color: %s; }", bg, bg, fg))
}

func (s *Signature) showItem(args []interface{}) {
	text := args[0].(string)
	s.text = text
	cursor := args[1].([]interface{})
	s.comma = util.ReflectToInt(args[2])
	s.cusor[0] = util.ReflectToInt(cursor[0])
	s.cusor[1] = util.ReflectToInt(cursor[1])
	s.update()
	s.move()
	s.hide()
	s.show()
}

func (s *Signature) pos(args []interface{}) {
	s.comma = util.ReflectToInt(args[0])
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
	s.label.SetText(formattedText)
}

func (s *Signature) move() {
	text := s.text
	row := s.ws.screen.cursor[0] + s.cusor[0]
	col := s.ws.screen.cursor[1] + s.cusor[1]
	i := strings.Index(text, "(")

	x, y, _, _ := s.ws.getPointInWidget(col, row, s.ws.cursor.gridid)
	if i > -1 {
		y -= s.widget.Height()
	}
	s.x = x
	s.y = y
	s.widget.Move2(s.x, s.y)
}

func (s *Signature) show() {
	if s.height == 0 {
		s.height = s.widget.SizeHint().Height()
	}
	s.widget.Show()
	s.widget.Raise()
}

func (s *Signature) hide() {
	s.widget.Hide()
}

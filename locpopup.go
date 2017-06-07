package gonvim

import (
	"github.com/dzhou121/ui"
)

// Locpopup is the location popup
type Locpopup struct {
	box     *ui.Box
	locType *SpanHandler
	text    *SpanHandler
}

func initLocpopup() *Locpopup {
	box := ui.NewHorizontalBox()

	typeHandler := &SpanHandler{}
	typeSpan := ui.NewArea(typeHandler)
	typeHandler.area = typeSpan

	textHandler := &SpanHandler{}
	textSpan := ui.NewArea(textHandler)
	textHandler.area = textSpan

	box.Append(textSpan, false)
	box.Append(typeSpan, false)
	box.Hide()

	return &Locpopup{
		box:     box,
		locType: typeHandler,
		text:    textHandler,
	}
}

func (l *Locpopup) show(loc map[string]interface{}) {
	return
	font := editor.font
	smallerFont := editor.smallerFont
	locType := loc["type"].(string)
	typePadding := smallerFont.shift
	typeMargin := int(float64(smallerFont.shift) * 1.4)
	heightDiff := (font.height - smallerFont.height) / 2
	textPadding := typePadding + typeMargin - heightDiff
	l.locType.SetFont(smallerFont)
	switch locType {
	case "E":
		l.locType.SetText("Error")
		l.locType.SetBackground(newRGBA(204, 62, 68, 1))
		l.locType.SetColor(newRGBA(255, 255, 255, 1))
	case "W":
		l.locType.SetText("Warning")
		l.locType.SetBackground(newRGBA(203, 203, 65, 1))
		l.locType.SetColor(newRGBA(255, 255, 255, 1))
	}
	l.locType.area.SetPosition(typeMargin, typeMargin)
	l.locType.paddingTop = typePadding
	l.locType.paddingLeft = typePadding
	l.locType.paddingRight = l.locType.paddingLeft
	l.locType.paddingBottom = l.locType.paddingTop
	w, _ := l.locType.getSize()
	l.locType.setSize(l.locType.getSize())

	text := loc["text"].(string)
	l.text.SetText(text)
	l.text.SetFont(font)
	l.text.SetColor(newRGBA(14, 17, 18, 1))
	l.text.SetBackground(newRGBA(212, 215, 214, 1))
	l.text.paddingLeft = w + typeMargin*2
	l.text.paddingRight = textPadding
	l.text.paddingTop = textPadding
	l.text.paddingBottom = l.text.paddingTop
	l.text.setSize(l.text.getSize())
	l.move()
	ui.QueueMain(func() {
		l.locType.area.Show()
		l.text.area.Show()
		l.locType.area.QueueRedrawAll()
		l.text.area.QueueRedrawAll()
		l.box.SetSize(l.text.getSize())
		l.box.Show()
	})
}

func (l *Locpopup) move() {
	row := editor.screen.cursor[0]
	col := editor.screen.cursor[1]
	ui.QueueMain(func() {
		l.box.SetPosition(int(float64(col)*editor.font.truewidth), (row+1)*editor.font.lineHeight)
	})
}

func (l *Locpopup) hide() {
	ui.QueueMain(func() {
		l.box.Hide()
	})
}

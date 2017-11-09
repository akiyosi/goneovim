package editor

import (
	"fmt"
	"time"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

// Message isj
type Message struct {
	kind    string
	width   int
	widget  *widgets.QWidget
	layout  *widgets.QGridLayout
	items   []*MessageItem
	expires int
}

// MessageItem is
type MessageItem struct {
	active  bool
	kind    string
	text    string
	hideAt  time.Time
	expired bool
	icon    *svg.QSvgWidget
	label   *widgets.QLabel
	widget  *widgets.QWidget
}

func initMessage() *Message {
	width := 250
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	layout := widgets.NewQGridLayout2()
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetSpacing(0)
	layout.SetSizeConstraint(widgets.QLayout__SetMinAndMaxSize)
	widget.SetLayout(layout)
	widget.SetStyleSheet("* {background-color: rgba(24, 29, 34, 1); color: rgba(205, 211, 222, 1);}")

	items := []*MessageItem{}
	for i := 0; i < 10; i++ {
		w := widgets.NewQWidget(nil, 0)
		w.SetContentsMargins(0, 0, 0, 0)
		w.SetStyleSheet(".QWidget {border-left: 1px solid #000; border-bottom: 1px solid #000;}")
		w.SetFixedWidth(34)
		icon := svg.NewQSvgWidget(nil)
		icon.SetFixedSize2(14, 14)
		icon.SetParent(w)
		icon.Move2(10, 10)
		l := widgets.NewQLabel(nil, 0)
		l.SetContentsMargins(10, 10, 10, 10)
		l.SetStyleSheet("border-bottom: 1px solid #000; border-left: 1px solid #000; border-right: 1px solid #000;")
		l.SetWordWrap(true)
		l.SetText(fmt.Sprintf("ldksj sdlkfjd  lkdsj sdlkfj sdlfkj lsdlfj  dslfjdsf sdfdslfkjdsf lksdjf sdklfj sldfkj sldkfj sdlfkj sdlkfjsdf test%d", i))
		layout.AddWidget(w, i, 0, 0)
		layout.AddWidget(l, i, 1, 0)
		w.Hide()
		l.Hide()
		items = append(items, &MessageItem{
			label:  l,
			icon:   icon,
			widget: w,
		})
	}
	widget.Show()
	return &Message{
		width:   width,
		widget:  widget,
		layout:  layout,
		items:   items,
		expires: 10,
	}
}

func (m *Message) subscribe() {
	editor.signal.ConnectMessageSignal(func() {
		m.update()
	})
}

func (m *Message) update() {
	now := time.Now()
	hasExpired := false
	for _, item := range m.items {
		if !item.active {
			break
		}
		if item.active && item.hideAt.Before(now) {
			item.expired = true
			hasExpired = true
		}
	}
	if !hasExpired {
		return
	}
	i := 0
	for _, item := range m.items {
		if item.active && !item.expired {
			m.items[i].copy(item)
			item.expired = true
			i++
		}
	}
	for _, item := range m.items {
		if item.active && item.expired {
			item.hide()
		}
	}
}

func (m *Message) resize() {
	m.width = editor.screen.width / 4
	m.widget.Move2(editor.screen.width-m.width-34, 0)
	m.widget.Resize2(m.width+34, 0)
	for _, item := range m.items {
		item.label.SetMinimumHeight(0)
		item.label.SetMinimumHeight(item.label.HeightForWidth(m.width))
	}
}

func (m *Message) chunk(args []interface{}) {
	text := ""
	for _, arg := range args {
		chunk, ok := arg.([]interface{})
		if !ok {
			continue
		}
		if len(chunk) != 2 {
			continue
		}
		msg, ok := chunk[0].(string)
		if !ok {
			continue
		}
		text += msg
	}
	if text == "" {
		return
	}
	for text[len(text)-1] == '\n' {
		text = string(text[:len(text)-1])
		if text == "" {
			return
		}
	}
	var item *MessageItem
	allActive := true
	for _, item = range m.items {
		if !item.active {
			allActive = false
			break
		}
	}
	if allActive {
		for i := 0; i < len(m.items)-1; i++ {
			m.items[i].copy(m.items[i+1])
		}
	}

	item.hideAt = time.Now().Add(time.Duration(m.expires) * time.Second)
	time.AfterFunc(time.Duration(m.expires+1)*time.Second, func() {
		editor.signal.MessageSignal()
	})
	item.setKind(m.kind)
	item.setText(text)
	item.show()
	m.widget.Resize2(m.width+34, 0)
}

func (i *MessageItem) setText(text string) {
	i.text = text
	label := i.label
	label.SetMinimumHeight(0)
	// label.SetMaximumHeight(0)
	label.SetText(text)
	height := label.HeightForWidth(editor.message.width)
	label.SetMinimumHeight(height)
	label.SetMaximumHeight(height)
	i.widget.SetMinimumHeight(height)
	i.widget.SetMaximumHeight(height)
}

func (i *MessageItem) copy(item *MessageItem) {
	i.hideAt = item.hideAt
	i.setKind(item.kind)
	i.setText(item.text)
	i.show()
}

func (i *MessageItem) hide() {
	i.expired = false
	if !i.active {
		return
	}
	i.active = false
	i.widget.Hide()
	i.icon.Hide()
	i.label.Hide()
}

func (i *MessageItem) show() {
	i.expired = false
	if i.active {
		return
	}
	i.active = true
	i.label.Show()
	i.icon.Show()
	i.widget.Show()
}

func (i *MessageItem) setKind(kind string) {
	if kind == i.kind {
		return
	}
	i.kind = kind
	style := "border-bottom: 1px solid #000; border-left: 1px solid #000; border-right: 1px solid #000;"
	switch i.kind {
	case "emsg":
		style += "color: rgba(204, 62, 68, 1);"
		svgContent := getSvg("fire", newRGBA(204, 62, 68, 1))
		i.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	default:
		style += "color: rgba(81, 154, 186, 1);"
		svgContent := getSvg("comment", nil)
		i.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	}
	i.label.SetStyleSheet(style)
}

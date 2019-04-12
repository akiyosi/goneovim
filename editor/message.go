package editor

import (
	"fmt"
	"time"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"

	"github.com/akiyosi/gonvim/util"
)

// Message isj
type Message struct {
	ws      *Workspace
	kind    string
	width   int
	widget  *widgets.QWidget
	layout  *widgets.QGridLayout
	items   []*MessageItem
	expires int
}

// MessageItem is
type MessageItem struct {
	m       *Message
	active  bool
	kind    string
	attrId  int
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

	m := &Message{
		width:   width,
		widget:  widget,
		layout:  layout,
		expires: 10,
	}

	items := []*MessageItem{}
	for i := 0; i < 10; i++ {
		w := widgets.NewQWidget(nil, 0)
		w.SetContentsMargins(0, 0, 0, 0)
		w.SetStyleSheet("* { background-color: rgba(0, 0, 0, 0); border: 0px solid #000;}")
		w.SetFixedWidth(editor.iconSize)
		icon := svg.NewQSvgWidget(nil)
		icon.SetStyleSheet("* { background-color: rgba(0, 0, 0, 0); border: 0px solid #000;}")
		icon.SetFixedSize2(editor.iconSize, editor.iconSize)
		icon.SetParent(w)
		icon.Move2(10, 10)
		l := widgets.NewQLabel(nil, 0)
		l.SetStyleSheet("* { background-color: rgba(0, 0, 0, 0); border: 0px solid #000;}")
		l.SetContentsMargins(10, 10, 10, 10)
		l.SetWordWrap(true)
		l.SetText("dummy text")
		layout.AddWidget(w, i, 0, 0)
		layout.AddWidget(l, i, 1, 0)
		w.Hide()
		l.Hide()
		items = append(items, &MessageItem{
			m:      m,
			label:  l,
			icon:   icon,
			widget: w,
		})
	}
	widget.Show()
	m.items = items
	return m
}

func (m *Message) setColor() {
	fg := editor.colors.widgetFg.String()
	bg := editor.colors.widgetBg
	// transparent := editor.config.Editor.Transparent / 2.0
	transparent := transparent()
	m.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: rgba(%d, %d, %d, %f);  color: %s; }", bg.R, bg.G, bg.B, transparent, fg))
}

func (m *Message) subscribe() {
	m.ws.signal.ConnectMessageSignal(func() {
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
	m.width = m.ws.width / 4
	m.widget.Move2(m.ws.width-m.width-editor.iconSize, 0)
	m.widget.Resize2(m.width+editor.iconSize, 0)
	for _, item := range m.items {
		item.label.SetMinimumHeight(0)
		item.label.SetMinimumHeight(item.label.HeightForWidth(m.width))
	}
}

func (m *Message) msgShow(args []interface{}) {
	for _, arg := range args {
		kind, ok := arg.([]interface{})[0].(string)
		if !ok {
			continue
		}

		text := ""
		var attrId int
		for _, tupple := range arg.([]interface{})[1].([]interface{}) {
			chunk, ok := tupple.([]interface{})
			if !ok {
				continue
			}
			if len(chunk) != 2 {
				continue
			}
			attrId = util.ReflectToInt(chunk[0])
			if !ok {
				continue
			}
			msg, ok := chunk[1].(string)
			if !ok {
				continue
			}
			text += msg
		}

		replaceLast := false
		if len(arg.([]interface{})) > 2 {
			replaceLast, ok = arg.([]interface{})[2].(bool)
		}

		m.makeMessage(kind, attrId, text, replaceLast)
	}
}

func (m *Message) makeMessage(kind string, attrId int, text string, replaceLast  bool) {
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

	if replaceLast {
		for _, i := range m.items {
			i.hide()
		}
	}

	item.hideAt = time.Now().Add(time.Duration(m.expires) * time.Second)
	time.AfterFunc(time.Duration(m.expires+1)*time.Second, func() {
		m.ws.signal.MessageSignal()
	})
	item.attrId = attrId
	item.setKind(kind)
	item.setText(text)
	item.show()
	m.widget.Resize2(m.width+editor.iconSize, 0)
}

func (m *Message) msgClear() {
	for _, item := range m.items {
		item.hide()
	}
}

func (m *Message) msgHistoryShow(args []interface{}) {
	for _, arg := range args {
		m.msgShow((arg.([]interface{})[0]).([]interface{}))
	}
}


func (i *MessageItem) setText(text string) {
	i.text = text
	label := i.label
	label.SetMinimumHeight(0)
	// label.SetMaximumHeight(0)
	label.SetText(text)
	height := label.HeightForWidth(i.m.width)
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
	i.m.widget.Raise()
	i.widget.Raise()
}

func (i *MessageItem) setKind(kind string) {
	if kind == i.kind {
		return
	}
	i.kind = kind
	var style string
	var color *RGBA
	switch i.kind {
	case "emsg", "echo", "echomsg", "echoerr", "return_prompt", "quickfix" :
		color = (i.m.ws.screen.highAttrDef[i.attrId]).foreground
		style = fmt.Sprintf("* { border: 0px solid #000; background-color: rgba(0, 0, 0 ,0); color: %s;}", color.String())
		svgContent := editor.getSvg(i.kind, color)
		i.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	default:
		color = (i.m.ws.screen.highAttrDef[i.attrId]).foreground
		style = fmt.Sprintf("* { border: 0px solid #000; background-color: rgba(0, 0, 0 ,0); color: %s;}", color.String())
		svgContent := editor.getSvg("echo", nil)
		i.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	}
	i.label.SetStyleSheet(style)
}

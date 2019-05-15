package editor

import (
	"fmt"
	"time"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"

	"github.com/akiyosi/gonvim/util"
)

// Message is
type Message struct {
	ws      *Workspace
	kind    string
	width   int
	widget  *widgets.QWidget
	layout  *widgets.QGridLayout
	items   []*MessageItem
	expires int
	pos     *core.QPoint
	isDrag  bool
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

	margin := 5

	items := []*MessageItem{}
	for i := 0; i < 10; i++ {
	 	w := widgets.NewQWidget(m.widget, 0)
	 	w.SetContentsMargins(0, 0, 0, 0)
	 	w.SetStyleSheet("* { background-color: rgba(0, 0, 0, 0); border: 0px solid #000;}")
	 	w.SetFixedWidth(editor.iconSize)
	 	icon := svg.NewQSvgWidget(nil)
	 	icon.SetStyleSheet("* { background-color: rgba(0, 0, 0, 0); border: 0px solid #000;}")
	 	icon.SetFixedSize2(editor.iconSize, editor.iconSize)
	 	icon.SetParent(w)
	 	icon.Move2(margin, margin)
	 	l := widgets.NewQLabel(nil, 0)
	 	l.SetStyleSheet("* { background-color: rgba(0, 0, 0, 0); border: 0px solid #000;}")
	 	l.SetContentsMargins(margin, margin, margin, margin)
	 	l.SetWordWrap(true)
	 	l.SetText("dummy text")
	 	layout.AddWidget(w, i, 0, 0)
		layout.SetAlignment(w, core.Qt__AlignTop)
	 	layout.AddWidget(l, i, 1, 0)
		layout.SetAlignment(l, core.Qt__AlignTop)
		item := &MessageItem{
			m:      m,
			label:  l,
			icon:   icon,
			widget: w,
			active: true,
			expired: true,
		}
		items = append(items, item)
	}
	m.items = items

	// Enable widget dragging
	m.widget.ConnectMousePressEvent(func(event *gui.QMouseEvent) {
		m.widget.Raise()
		m.isDrag = true
		m.pos = event.Pos()
	})
	m.widget.ConnectMouseReleaseEvent(func(*gui.QMouseEvent) {
		m.isDrag = false
	})
	m.widget.ConnectMouseMoveEvent(func(event *gui.QMouseEvent) {
		if m.isDrag == true {
			x := event.Pos().X() - m.pos.X()
			y := event.Pos().Y() - m.pos.Y()
			newPos := core.NewQPoint2(x, y)
			trans := m.widget.MapToParent(newPos)
			m.widget.Move(trans)
		}
	})

	// Drop shadow to widget
	go func() {
		shadow := widgets.NewQGraphicsDropShadowEffect(nil)
		shadow.SetBlurRadius(40)
		shadow.SetColor(gui.NewQColor3(0, 0, 0, 200))
		shadow.SetOffset3(-2, 4)
		m.widget.SetGraphicsEffect(shadow)
	}()
	m.widget.Hide()

	// hide messgaes at startup.
	m.update()

	return m
}

func (m *Message) setColor() {
	fg := editor.colors.widgetFg.String()
	bg := editor.colors.widgetBg
	transparent := transparent()
	m.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: rgba(%d, %d, %d, %f);  color: %s; }", bg.R, bg.G, bg.B, transparent, fg))
	for _, item := range m.items {
	 	item.icon.SetFixedSize2(m.ws.font.height, m.ws.font.height)
		item.label.SetFont(m.ws.font.fontNew)
	}
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
	m.width = m.ws.screen.widget.Width() / 3
	m.widget.Move2(m.ws.width-m.width-editor.iconSize-m.ws.scrollBar.widget.Width()-12, 6+m.ws.tabline.widget.Height())
	height := 0
	width := m.ws.font.width
	posChange := false
	for _, item := range m.items {
		item.label.SetMinimumHeight(0)
		item.label.SetMinimumHeight(item.label.HeightForWidth(m.width))
		item.label.SetAlignment(core.Qt__AlignTop)
		if !item.active {
			continue
		}
		if m.width < width * len(item.label.Text()) {
			posChange = true
		}
		height += item.widget.Height()
	}

	// if message is too long, message widget move to bottom half of screen
	if posChange {
		m.width = m.ws.screen.widget.Width() - m.ws.scrollBar.widget.Width() - 12
		m.widget.Move2(10, m.ws.screen.widget.Height() - height - m.ws.statusline.widget.Height())
	}

	m.widget.Resize2(m.width+editor.iconSize, height)
}

func (m *Message) msgShow(args []interface{}) {
	prevKind := ""
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

		if kind == prevKind {
			// Do not show message icon if the same kind as the previous kind
			m.makeMessage("_dup", attrId, text, replaceLast)
		} else {
			m.makeMessage(kind, attrId, text, replaceLast)
		}
		prevKind = kind
	}
}

func (m *Message) makeMessage(kind string, attrId int, text string, replaceLast  bool) {
	defer m.resize()
	if text == "" {
		return
	}
	for text[len(text)-1] == '\n' {
		text = string(text[:len(text)-1])
		if text == "" {
			break
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
	i.widget.SetMinimumWidth(editor.iconSize+5)
	i.widget.SetMaximumWidth(editor.iconSize+5)
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
	i.m.widget.Show()
	i.m.widget.Raise()
	i.widget.Raise()
}

func (i *MessageItem) setKind(kind string) {
	i.kind = kind
	var style string
	var color *RGBA
	if i.m.ws == nil {
		return
	}
	if i.m.ws.screen.highAttrDef[i.attrId] == nil {
		return
	}
	switch i.kind {
	case "emsg", "echo", "echomsg", "echoerr", "return_prompt", "quickfix" :
		color = (i.m.ws.screen.highAttrDef[i.attrId]).foreground
		style = fmt.Sprintf("* { border: 0px solid #000; background-color: rgba(0, 0, 0 ,0); color: %s;}", color.String())
		svgContent := editor.getSvg(i.kind, color)
		i.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	case "_dup":
		color = (i.m.ws.screen.highAttrDef[i.attrId]).foreground
		style = fmt.Sprintf("* { border: 0px solid #000; background-color: rgba(0, 0, 0 ,0); color: %s;}", color.String())
		// hide 
		svgContent := editor.getSvg("echo", editor.colors.widgetBg)
		i.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	default:
		color = (i.m.ws.screen.highAttrDef[i.attrId]).foreground
		style = fmt.Sprintf("* { border: 0px solid #000; background-color: rgba(0, 0, 0 ,0); color: %s;}", color.String())
		svgContent := editor.getSvg("echo", color)
		i.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	}
	i.label.SetStyleSheet(style)
}

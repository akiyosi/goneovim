package editor

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"

	"github.com/akiyosi/goneovim/util"
)

const DummyText = "dummy text"

// Message is
type Message struct {
	ws       *Workspace
	width    int
	widget   *widgets.QWidget
	layout   *widgets.QGridLayout
	items    []*MessageItem
	expires  int
	pos      *core.QPoint
	isDrag   bool
	isExpand bool
}

// MessageItem is
type MessageItem struct {
	m          *Message
	active     bool
	kind       string
	attrId     int
	text       string
	hideAt     time.Time
	expired    bool
	icon       *svg.QSvgWidget
	label      *widgets.QLabel
	widget     *widgets.QWidget
	textLength int
}

func initMessage() *Message {
	width := 250
	widget := widgets.NewQWidget(nil, 0)
	widget.SetObjectName("message")
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
		expires: 120,
	}

	margin := 5

	items := []*MessageItem{}
	for i := 0; i < 10; i++ {
		w := widgets.NewQWidget(m.widget, 0)
		w.SetContentsMargins(0, 0, 0, 0)
		w.SetStyleSheet(" * { background-color: rgba(0, 0, 0, 0); border: 0px solid #000;}")
		w.SetFixedWidth(editor.iconSize)
		icon := svg.NewQSvgWidget(nil)
		// icon.SetStyleSheet("* { background-color: rgba(0, 0, 0, 0); border: 0px solid #000;}")
		icon.SetFixedSize2(editor.iconSize, editor.iconSize)
		icon.SetParent(w)
		icon.Move2(margin, margin)
		l := widgets.NewQLabel(nil, 0)
		// l.SetStyleSheet("* { background-color: rgba(0, 0, 0, 0); border: 0px solid #000;}")
		l.SetContentsMargins(margin, margin, margin, margin)
		l.SetWordWrap(true)
		l.SetText(DummyText)
		item := &MessageItem{
			m:       m,
			text:    DummyText,
			label:   l,
			icon:    icon,
			widget:  w,
			active:  true,
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
		if m.isDrag {
			x := event.Pos().X() - m.pos.X()
			y := event.Pos().Y() - m.pos.Y()
			newPos := core.NewQPoint2(x, y)
			trans := m.widget.MapToParent(newPos)
			m.widget.Move(trans)
		}
	})

	// layout message item
	for i, item := range items {
		layout.AddWidget2(item.widget, i, 0, 0)
		layout.SetAlignment(item.widget, core.Qt__AlignTop)
		layout.AddWidget2(item.label, i, 1, 0)
		layout.SetAlignment(item.label, core.Qt__AlignTop)
	}
	// Drop shadow to widget
	m.widget.SetGraphicsEffect(util.DropShadow(-2, 4, 40, 200))
	m.widget.Hide()

	// hide messgaes at startup.
	m.update()

	return m
}

func (m *Message) setColor() {
	fg := editor.colors.fg.String()
	bg := warpColor(editor.colors.bg, -15)
	transparent := transparent() * transparent()
	if editor.config.Message.Transparent < 1.0 {
		transparent = editor.config.Message.Transparent
	}
	m.widget.SetStyleSheet(fmt.Sprintf(
		" #message { background-color: rgba(%d, %d, %d, %f);  color: %s; }",
		bg.R,
		bg.G,
		bg.B,
		transparent,
		fg,
	))
}

func (m *Message) updateFont() {
	margin := m.ws.font.height / 3
	for _, item := range m.items {
		item.widget.SetFixedSize2(m.ws.font.height*5/3, m.ws.font.height*5/3)
		item.icon.SetFixedSize2(m.ws.font.height, m.ws.font.height)
		item.label.SetContentsMargins(margin/2, margin, margin, margin)
		item.icon.Move2(margin*5/4, margin)
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
	if m.ws == nil {
		return
	}
	if m.ws.screen == nil {
		return
	}

	leftPadding := 0
	if editor.wsSide != nil {
		if editor.wsSide.scrollarea != nil {
			leftPadding = editor.wsSide.scrollarea.Width()
		}
	}

	scrollbarwidth := 0
	if editor.config.ScrollBar.Visible {
		scrollbarwidth = m.ws.scrollBar.widget.Width()
	}

	var x, y int
	var ok bool
	if !m.isExpand {
		m.width = m.ws.screen.widget.Width() / 3
		ok = m.resizeMessages()
		if !ok {
			m.isExpand = true
			m.width = m.ws.screen.widget.Width() - scrollbarwidth - 12
			_ = m.resizeMessages()
		}
	}
	m.widget.Resize2(m.width+editor.iconSize, 0)
	if !ok {
		x = 10 + leftPadding
		// y = m.ws.widget.Height() - m.ws.statusline.widget.Height() - m.widget.Height()
		y = m.ws.widget.Height() - m.widget.Height()
		if m.ws.statusline != nil {
			y -= m.ws.statusline.widget.Height()
		}
	} else {
		x = m.ws.width + leftPadding - m.width - editor.iconSize - scrollbarwidth - 12
		// y = 6 + m.ws.tabline.widget.Height()
		y = 6
		if m.ws.tabline != nil {
			y += m.ws.tabline.widget.Height()
		}
	}
	m.widget.Move2(x, y)
}

func (m *Message) resizeMessages() bool {
	ok := true
	width := m.ws.font.truewidth
	for _, item := range m.items {
		item.widget.SetFixedSize2(m.ws.font.height*5/3, m.ws.font.height*5/3)
		item.label.SetMinimumHeight(0)
		item.label.SetMinimumHeight(item.label.HeightForWidth(m.width))
		item.label.SetAlignment(core.Qt__AlignTop)
		if !item.active {
			continue
		}

		labelWidth := m.width - item.widget.Width()
		messageWidth := int(width * float64(item.textLength))

		if labelWidth < messageWidth {
			ok = false
		}
	}

	return ok
}

func (m *Message) msgShow(args []interface{}) {
	prevKind := ""
	isActiveState := editor.window.IsActiveWindow()
	notifyText := ""

	for _, arg := range args {
		kind, ok := arg.([]interface{})[0].(string)
		if !ok {
			continue
		}
		// text := ""
		var buffer bytes.Buffer
		length := 0
		lineLen := 0
		scrollbarwidth := 0
		if editor.config.ScrollBar.Visible {
			scrollbarwidth = m.ws.scrollBar.widget.Width()
		}
		maxLen := m.ws.screen.widget.Width() - scrollbarwidth - 12
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
			var color *RGBA
			if m.ws.screen.hlAttrDef[attrId] != nil {
				color = (m.ws.screen.hlAttrDef[attrId]).foreground
			} else {
				color = m.ws.foreground
			}
			if msg == "" || msg == "\n" || msg == "\r\n" {
				continue
			}

			// If window is minimize, then message notified as a desktop notifications
			if !isActiveState {
				notifyText += msg
			}

			length += len(msg)

			if !strings.Contains(msg, "\n") {
				var cBuffer bytes.Buffer
				for _, c := range msg {
					lineLen += len(string(c))
					msgLen := int(m.ws.font.truewidth * float64(lineLen))
					if msgLen >= maxLen {
						cBuffer.WriteString(`<br>`)
						lineLen = 0
					}
					cBuffer.WriteString(string(c))
				}
				msg = cBuffer.String()
			}

			msg = strings.Replace(msg, "\r\n", `<br>`, -1)
			msg = strings.Replace(msg, "\n", `<br>`, -1)
			msg = strings.Replace(msg, " ", `&nbsp;`, -1)
			formattedMsg := fmt.Sprintf("<font color='%s'>%s</font>", warpColor(color, -20).Hex(), msg)
			buffer.WriteString(formattedMsg)
		}

		// If window is minimize, then message notified as a desktop notifications
		if !isActiveState && notifyText != "" {
			editor.sysTray.ShowMessage("GoNeovim", notifyText, widgets.QSystemTrayIcon__NoIcon, 2000)
			return
		}

		replaceLast := false
		if len(arg.([]interface{})) > 2 {
			replaceLast, _ = arg.([]interface{})[2].(bool)
		}
		if kind == prevKind {
			// Do not show message icon if the same kind as the previous kind
			m.makeMessage("_dup", attrId, buffer.String(), length, replaceLast)
		} else {
			m.makeMessage(kind, attrId, buffer.String(), length, replaceLast)
		}
		prevKind = kind
	}
	m.isExpand = false
}

func (m *Message) makeMessage(kind string, attrId int, text string, length int, replaceLast bool) {
	defer m.resize()
	var item *MessageItem
	allActive := true
	for _, item = range m.items {
		item.textLength = length
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

	MessageSignal := m.ws.signal.MessageSignal

	item.hideAt = time.Now().Add(time.Duration(m.expires) * time.Second)
	time.AfterFunc(time.Duration(m.expires+1)*time.Second, func() {
		MessageSignal()
	})
	item.attrId = attrId
	item.setKind(kind)
	if item.text == DummyText {
		item.show()
	}
	item.setText(text)
	item.show()
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
	i.label.SetMinimumHeight(0)
	i.label.SetText(text)
	height := i.label.HeightForWidth(i.m.width)
	i.label.SetMinimumHeight(height)
	i.label.SetMaximumHeight(height)
	i.widget.SetMinimumHeight(height)
	i.widget.SetMaximumHeight(height)
	i.widget.SetFixedSize2(i.m.ws.font.height*5/3, i.m.ws.font.height*5/3)
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
	var color *RGBA
	if i.m.ws == nil {
		return
	}
	if i.m.ws.screen.hlAttrDef[i.attrId] == nil {
		return
	}
	color = warpColor((i.m.ws.screen.hlAttrDef[i.attrId]).foreground, -15)
	switch i.kind {
	case "emsg", "echo", "echomsg", "echoerr", "return_prompt", "quickfix":
		svgContent := editor.getSvg(i.kind, color)
		i.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	case "_dup":
		// hide
		svgContent := editor.getSvg("echo", warpColor(editor.colors.bg, -15))
		i.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	default:
		svgContent := editor.getSvg("echo", color)
		i.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	}
}

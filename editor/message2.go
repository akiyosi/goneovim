package editor

import (
	"fmt"

	"github.com/akiyosi/goneovim/util"
)

const (
	MARGINOFMESSAGEPOS = 10
)

type Messages struct {
	ws *Workspace

	msgs []*Message
}

type Message struct {
	Tooltip

	m *Messages

	kind        string
	replaceLast bool

	expires  int
	isDrag   bool
	isExpand bool
}

func initMessages() *Messages {
	return &Messages{
		msgs: []*Message{},
	}
}

func (m *Messages) msgClear() {
	for _, msg := range m.msgs {
		msg.Hide()
	}

	m.msgs = []*Message{}
}

func (m *Messages) msgShow(args []interface{}, bulkmsg bool) {
	var msg *Message
	for _, arg := range args {

		// kind
		var ok bool
		var kind string
		kind, ok = arg.([]interface{})[0].(string)

		// replaceLast
		var replaceLast bool
		if len(arg.([]interface{})) > 2 {
			replaceLast, ok = arg.([]interface{})[2].(bool)
			if !ok {
				continue
			}
		}

		if replaceLast {
			// m.msgs[len(m.msgs)-2].Hide()

			for _, mm := range m.msgs {
				mm.Hide()
			}
		}

		maximumWidth := m.ws.screen.widget.Width() / 2
		fixedWidth := false
		if bulkmsg {
			maximumWidth = int(float64(m.ws.screen.widget.Width()) * 0.85)
			fixedWidth = true
		}

		var emmitNewMsg bool = true
		if len(m.msgs) == 0 || replaceLast {
			msg = m.newMessage(maximumWidth, fixedWidth)
		} else if len(m.msgs) > 0 && kind != "" && m.msgs[len(m.msgs)-1].kind != kind && !bulkmsg {
			msg = m.newMessage(maximumWidth, fixedWidth)
		} else {
			msg = m.msgs[len(m.msgs)-1]
			emmitNewMsg = false
		}

		// content
		for _, c := range arg.([]interface{})[1].([]interface{}) {
			tuple, ok := c.([]interface{})
			if !ok {
				continue
			}

			attrId, textChunk := parseContent(tuple)
			hl, ok := msg.m.ws.screen.hlAttrDef[attrId]
			if !ok {
				continue
			}

			fmt.Println(textChunk)
			if !emmitNewMsg {
				msg.updateText(
					hl,
					"\n",
					0,
					msg.font.qfont,
				)
			}

			msg.updateText(
				hl,
				textChunk,
				float64(m.ws.screen.font.letterSpace),
				resolveFontFallback(
					m.ws.screen.font,
					m.ws.screen.fallbackfonts,
					textChunk,
				).qfont,
			)
		}

		msg.kind = kind
		msg.replaceLast = replaceLast
		msg.update()

		msg.emmit()
	}

	return
}

func (m *Messages) msgHistoryShow(entries []interface{}) {
	for _, entrie := range entries {
		m.msgShow((entrie.([]interface{})[0]).([]interface{}), true)
	}
}

func parseContent(tuple []interface{}) (attrId int, textChunk string) {
	if len(tuple) != 2 {
		return
	}

	attrId = util.ReflectToInt(tuple[0])

	var ok bool
	textChunk, ok = tuple[1].(string)
	if !ok {
		return
	}

	return
}

func (m *Messages) newMessage(maximumWidth int, fixedWidth bool) *Message {
	msg := NewMessage(m.ws.screen.widget, 0)
	msg.s = m.ws.screen
	msg.setPadding(20, 10)
	msg.setRadius(14, 10)
	msg.maximumWidth = maximumWidth
	msg.pathWidth = 2
	msg.setBgColor(m.ws.background)
	msg.setFont(m.ws.font)
	msg.fixedWidth = fixedWidth
	msg.ConnectPaintEvent(msg.paint)

	m.msgs = append(m.msgs, msg)
	msg.m = m

	return msg
}

func (msg *Message) emmit() {
	// TODO: stacked positioning
	stackedMsgHeight := 0
	for _, mm := range msg.m.msgs {
		if !mm.IsVisible() {
			continue
		}
		if mm == msg {
			break
		}
		stackedMsgHeight += mm.Height() + MARGINOFMESSAGEPOS
	}
	msg.show()
	msg.Move2(
		msg.m.ws.screen.widget.Width()-msg.Width()-MARGINOFMESSAGEPOS,
		stackedMsgHeight,
	)
	msg.SetGraphicsEffect(util.DropShadow(-1, 16, 130, 180))
}

func (m *Messages) updateFont() {

	var newMsgs []*Message

	for _, msg := range m.msgs {
		oldText := msg.text
		var newText []*ColorStr
		msg.text = newText

		for _, colorStr := range oldText {
			hl := colorStr.hl
			text := colorStr.str
			msg.updateText(
				hl,
				text,
				float64(m.ws.screen.font.letterSpace),
				resolveFontFallback(
					m.ws.screen.font,
					m.ws.screen.fallbackfonts,
					text,
				).qfont,
			)
			msg.update()
			newMsgs = append(newMsgs, msg)
		}
	}

	m.msgs = newMsgs
}

package gonvim

import (
	"github.com/dzhou121/ui"
	"github.com/neovim/go-client/nvim"
)

// Tabline of the editor
type Tabline struct {
	bg        *SpanHandler
	box       *ui.Box
	CurrentID int
	Tabs      []*Tab
	height    int
}

// Tab in the tabline
type Tab struct {
	ID      int
	Name    string
	current bool
	span    *SpanHandler
	width   int
}

func initTabline(width int, height int) *Tabline {
	box := ui.NewHorizontalBox()
	box.SetSize(width, height)
	handler := &SpanHandler{}
	bgSpan := ui.NewArea(handler)
	handler.span = bgSpan
	handler.SetBackground(newRGBA(24, 29, 34, 1))
	handler.borderBottom = &Border{
		width: 2,
		color: newRGBA(0, 0, 0, 1),
	}
	handler.setSize(width, height)
	ui.QueueMain(func() {
		box.Show()
		box.Append(bgSpan, false)
	})
	return &Tabline{
		box:    box,
		height: height,
		bg:     handler,
	}
}

func (t *Tabline) resize(width int, height int) {
	t.box.SetSize(width, height)
	t.bg.setSize(width, height)
}

func (t *Tabline) update(args []interface{}) {
	arg := args[0].([]interface{})
	t.CurrentID = int(arg[0].(nvim.Tabpage))
	tabs := arg[1].([]interface{})
	for i, tabInterface := range tabs {
		tabMap, ok := tabInterface.(map[string]interface{})
		if !ok {
			continue
		}
		if i > len(t.Tabs)-1 {
			handler := &SpanHandler{}
			tabSpan := ui.NewArea(handler)
			handler.span = tabSpan
			tab := &Tab{
				span:  handler,
				width: 200,
			}
			t.Tabs = append(t.Tabs, tab)
			fg := newRGBA(212, 215, 214, 1)
			padding := (t.height - editor.font.height - 2) / 2
			handler.SetColor(fg)
			handler.SetBackground(newRGBA(0, 0, 0, 1))
			handler.SetFont(editor.font)
			handler.setSize(tab.width, t.height)
			handler.paddingTop = padding
			handler.paddingLeft = int(float64(padding) * 1.5)
			handler.borderRight = &Border{
				width: 1,
				color: newRGBA(0, 0, 0, 1),
			}
			handler.borderBottom = &Border{
				width: 2,
				color: newRGBA(0, 0, 0, 1),
			}
			ui.QueueMain(func() {
				t.box.Append(tabSpan, false)
				tabSpan.SetPosition(i*tab.width, 0)
			})
		}
		tab := t.Tabs[i]
		tab.ID = int(tabMap["tab"].(nvim.Tabpage))
		if tab.ID == t.CurrentID {
			tab.current = true
			tab.span.borderBottom.color = newRGBA(81, 154, 186, 1)
			tab.span.SetBackground(newRGBA(0, 0, 0, 1))
			tab.span.SetColor(newRGBA(212, 215, 214, 1))
		} else {
			tab.current = false
			tab.span.borderBottom.color = newRGBA(0, 0, 0, 1)
			tab.span.SetBackground(newRGBA(24, 29, 34, 1))
			tab.span.SetColor(&editor.Foreground)
		}
		tab.Name = tabMap["name"].(string)
		tab.span.SetText(tab.Name)
		ui.QueueMain(func() {
			tab.span.span.Show()
			tab.span.span.QueueRedrawAll()
		})
	}

	for i := len(tabs); i < len(t.Tabs); i++ {
		tab := t.Tabs[i]
		tab.current = false
		ui.QueueMain(func() {
			tab.span.span.Hide()
		})
	}
}

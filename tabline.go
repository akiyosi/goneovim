package gonvim

import (
	"path/filepath"
	"strings"

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
	box      *ui.Box
	ID       int
	Name     string
	current  bool
	span     *SpanHandler
	bg       *SpanHandler
	width    int
	chars    int
	cross    *Svg
	fileicon *Svg
}

func initTabline(width int, height int) *Tabline {
	box := ui.NewHorizontalBox()
	box.SetSize(width, height)
	handler := &SpanHandler{}
	bgSpan := ui.NewArea(handler)
	handler.area = bgSpan
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
			box := ui.NewHorizontalBox()
			handler := &SpanHandler{}
			tabSpan := ui.NewArea(handler)
			handler.area = tabSpan
			bgHandler := &SpanHandler{}
			bgSpan := ui.NewArea(bgHandler)
			bgHandler.area = bgSpan
			padding := (t.height - editor.font.height - 2) / 2
			paddingLeft := editor.font.width * 2
			chars := 21
			fileiconWidth := editor.font.width * 2
			cross := newSvg("cross", editor.font.width*2, editor.font.width*2, newRGBA(255, 255, 255, 1), newRGBA(0, 0, 0, 1))
			fileicon := newSvg("default", fileiconWidth, fileiconWidth, nil, nil)
			tab := &Tab{
				box:      box,
				span:     handler,
				width:    editor.font.width*chars + fileiconWidth + 3*paddingLeft + editor.font.width,
				chars:    chars,
				bg:       bgHandler,
				cross:    cross,
				fileicon: fileicon,
			}
			t.Tabs = append(t.Tabs, tab)
			fg := newRGBA(212, 215, 214, 1)
			handler.SetColor(fg)
			handler.SetBackground(newRGBA(0, 0, 0, 1))
			handler.SetFont(editor.font)
			handler.setSize(tab.width-paddingLeft*3, t.height-2)
			box.SetSize(tab.width, t.height)
			bgHandler.setSize(tab.width, t.height)
			bgHandler.SetBackground(newRGBA(0, 0, 0, 1))
			handler.paddingTop = padding
			bgHandler.borderRight = &Border{
				width: 1,
				color: newRGBA(0, 0, 0, 1),
			}
			bgHandler.borderBottom = &Border{
				width: 2,
				color: newRGBA(0, 0, 0, 1),
			}

			ui.QueueMain(func() {
				box.Append(bgSpan, false)
				box.Append(tabSpan, false)
				box.Append(cross.area, false)
				box.Append(fileicon.area, false)
				t.box.Append(box, false)
				box.SetPosition(i*tab.width, 0)
				cross.area.SetPosition(tab.width-paddingLeft-editor.font.width, (t.height-cross.height)/2)
				fileicon.setPosition(paddingLeft, (t.height-fileicon.height)/2)
				tabSpan.SetPosition(paddingLeft+fileiconWidth+editor.font.width, 0)
			})
		}
		tab := t.Tabs[i]
		tab.ID = int(tabMap["tab"].(nvim.Tabpage))
		tab.Name = tabMap["name"].(string)
		text := tab.Name
		fileType := getFileType(text)
		tab.fileicon.name = fileType
		if len(text) > tab.chars {
			text = text[len(text)-tab.chars+3 : len(text)]
			text = "..." + text
		}
		tab.span.SetText(text)
		if tab.ID == t.CurrentID {
			tab.current = true
			tab.bg.borderBottom.color = newRGBA(81, 154, 186, 1)
			tab.bg.SetBackground(newRGBA(0, 0, 0, 1))
			tab.span.SetBackground(newRGBA(0, 0, 0, 1))
			tab.span.SetColor(newRGBA(212, 215, 214, 1))
			tab.cross.color = newRGBA(212, 215, 214, 1)
			tab.cross.bg = newRGBA(0, 0, 0, 1)
		} else {
			tab.current = false
			tab.bg.borderBottom.color = newRGBA(0, 0, 0, 1)
			tab.bg.SetBackground(newRGBA(24, 29, 34, 1))
			tab.span.SetBackground(newRGBA(24, 29, 34, 1))
			tab.span.SetColor(&editor.Foreground)
			tab.cross.color = &editor.Foreground
			tab.cross.bg = newRGBA(24, 29, 34, 1)
		}
		ui.QueueMain(func() {
			tab.box.Show()
			tab.span.area.QueueRedrawAll()
			tab.bg.area.QueueRedrawAll()
			tab.cross.area.QueueRedrawAll()
			tab.fileicon.area.QueueRedrawAll()
		})
	}

	for i := len(tabs); i < len(t.Tabs); i++ {
		tab := t.Tabs[i]
		tab.current = false
		ui.QueueMain(func() {
			tab.box.Hide()
		})
	}
}

func getFileType(text string) string {
	if strings.HasPrefix(text, "term://") {
		return "sh"
	}
	base := filepath.Base(text)
	if strings.Index(base, ".") >= 0 {
		parts := strings.Split(base, ".")
		return parts[len(parts)-1]
	}
	return "default"
}

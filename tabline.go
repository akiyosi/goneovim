package gonvim

import (
	"path/filepath"
	"strings"

	"github.com/dzhou121/ui"
	"github.com/neovim/go-client/nvim"
)

// Tabline of the editor
type Tabline struct {
	AreaHandler
	box       *ui.Box
	CurrentID int
	Tabs      []*Tab
}

// Tab in the tabline
type Tab struct {
	SpanHandler
	box      *ui.Box
	ID       int
	Name     string
	current  bool
	width    int
	chars    int
	cross    *Svg
	fileicon *Svg
}

func initTabline(width int, height int) *Tabline {
	box := ui.NewHorizontalBox()
	tabline := &Tabline{
		box: box,
	}
	tabline.area = ui.NewArea(tabline)
	tabline.bg = newRGBA(24, 29, 34, 1)
	tabline.borderBottom = &Border{
		width: 2,
		color: newRGBA(0, 0, 0, 1),
	}
	box.SetSize(width, height)
	box.Append(tabline.area, false)
	tabline.setSize(width, height)
	return tabline
}

func (t *Tabline) resize(width int, height int) {
	t.box.SetSize(width, height)
	t.setSize(width, height)
	for _, tab := range t.Tabs {
		tab.setSize(tab.width, height)
	}
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
			chars := 21
			fileiconWidth := editor.font.width * 2
			padding := (t.height - editor.font.height - 2) / 2
			paddingLeft := editor.font.width * 2
			cross := newSvg("cross", editor.font.width*2, editor.font.width*2, newRGBA(255, 255, 255, 1), newRGBA(0, 0, 0, 1))
			fileicon := newSvg("default", fileiconWidth, fileiconWidth, nil, nil)

			box := ui.NewHorizontalBox()
			tab := &Tab{
				box:      box,
				width:    editor.font.width*chars + fileiconWidth + 3*paddingLeft + editor.font.width,
				chars:    chars,
				cross:    cross,
				fileicon: fileicon,
			}
			tab.area = ui.NewArea(tab)
			tab.font = editor.font
			tab.paddingTop = padding
			tab.paddingLeft = paddingLeft + fileiconWidth + editor.font.width
			t.Tabs = append(t.Tabs, tab)
			tab.borderRight = &Border{
				width: 1,
				color: newRGBA(0, 0, 0, 1),
			}
			tab.borderBottom = &Border{
				width: 2,
				color: newRGBA(0, 0, 0, 1),
			}

			ui.QueueMain(func() {
				t.box.Append(box, false)
				box.Append(tab.area, false)
				box.Append(cross.area, false)
				box.Append(fileicon.area, false)
				box.SetSize(tab.width, t.height)
				tab.setSize(tab.width, t.height)
				box.SetPosition(i*tab.width, 0)
				cross.area.SetPosition(tab.width-paddingLeft-editor.font.width, (t.height-cross.height)/2)
				fileicon.setPosition(paddingLeft, (t.height-fileicon.height)/2)
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
		tab.text = text
		if tab.ID == t.CurrentID {
			tab.current = true
			tab.borderBottom.color = newRGBA(81, 154, 186, 1)
			tab.bg = newRGBA(0, 0, 0, 1)
			tab.color = newRGBA(212, 215, 214, 1)
			tab.cross.color = newRGBA(212, 215, 214, 1)
			tab.cross.bg = newRGBA(0, 0, 0, 1)
		} else {
			tab.current = false
			tab.borderBottom.color = newRGBA(0, 0, 0, 1)
			tab.bg = newRGBA(24, 29, 34, 1)
			tab.color = editor.Foreground
			tab.cross.color = editor.Foreground
			tab.cross.bg = newRGBA(24, 29, 34, 1)
		}
		ui.QueueMain(func() {
			tab.box.Show()
			tab.area.QueueRedrawAll()
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

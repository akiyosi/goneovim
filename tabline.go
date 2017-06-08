package gonvim

import (
	"path/filepath"
	"strings"

	"github.com/neovim/go-client/nvim"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

// Tabline of the editor
type Tabline struct {
	widget    *widgets.QWidget
	layout    *widgets.QLayout
	CurrentID int
	Tabs      []*Tab
}

// Tab in the tabline
type Tab struct {
	widget    *widgets.QWidget
	layout    *widgets.QHBoxLayout
	ID        int
	Name      string
	current   bool
	width     int
	chars     int
	cross     *Svg
	fileicon  *Svg
	fileIcon  *svg.QSvgWidget
	closeIcon *svg.QSvgWidget
	file      *widgets.QLabel
}

func newVFlowLayout(spacing int) *widgets.QLayout {
	layout := widgets.NewQLayout2()
	items := []*widgets.QLayoutItem{}
	layout.ConnectSizeHint(func() *core.QSize {
		size := core.NewQSize()
		for _, item := range items {
			size = size.ExpandedTo(item.MinimumSize())
		}
		return size
	})
	layout.ConnectAddItem(func(item *widgets.QLayoutItem) {
		items = append(items, item)
	})
	layout.ConnectSetGeometry(func(r *core.QRect) {
		x := 0
		sizes := [][]int{}
		maxHeight := 0
		for _, item := range items {
			sizeHint := item.SizeHint()
			width := sizeHint.Width()
			height := sizeHint.Height()
			size := []int{width, height}
			sizes = append(sizes, size)
			if height > maxHeight {
				maxHeight = height
			}
		}
		for i, item := range items {
			size := sizes[i]
			width := size[0]
			height := size[1]
			y := 0
			if height != maxHeight {
				y = (maxHeight - height) / 2
			}
			item.SetGeometry(core.NewQRect4(x, y, width, height))
			x += width + spacing
		}
	})
	layout.ConnectItemAt(func(index int) *widgets.QLayoutItem {
		if index < len(items) {
			return items[index]
		}
		return nil
	})
	layout.ConnectTakeAt(func(index int) *widgets.QLayoutItem {
		if index < len(items) {
			return items[index]
		}
		return nil
	})
	return layout
}

func initTablineNew(height int) *Tabline {
	width := 210
	widget := widgets.NewQWidget(nil, 0)
	layout := widgets.NewQLayout2()
	layout.SetSpacing(0)
	layout.SetContentsMargins(0, 0, 0, 0)
	items := []*widgets.QLayoutItem{}
	layout.ConnectSizeHint(func() *core.QSize {
		return core.NewQSize2(width, 0)
	})
	layout.ConnectAddItem(func(item *widgets.QLayoutItem) {
		items = append(items, item)
	})
	layout.ConnectSetGeometry(func(r *core.QRect) {
		for i := 0; i < len(items); i++ {
			items[i].SetGeometry(core.NewQRect4(width*i, 0, width, r.Height()))
		}
	})
	layout.ConnectItemAt(func(index int) *widgets.QLayoutItem {
		if index < len(items) {
			return items[index]
		}
		return nil
	})
	layout.ConnectTakeAt(func(index int) *widgets.QLayoutItem {
		if index < len(items) {
			return items[index]
		}
		return nil
	})
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetLayout(layout)
	widget.SetFixedHeight(height)
	widget.SetStyleSheet(`
	QWidget {
		color: rgba(147, 161, 161, 1);
	}
	.QWidget {
		border-bottom: 2px solid rgba(0, 0, 0, 1);
		border-right: 1px solid rgba(0, 0, 0, 1);
		background-color: rgba(24, 29, 34, 1);
	}
	`)
	widget.ConnectPaintEvent(func(event *gui.QPaintEvent) {
		rect := event.M_rect()
		width := rect.Width()
		height := rect.Height()
		p := gui.NewQPainter2(widget)
		p.FillRect5(
			0,
			0,
			width,
			height,
			gui.NewQColor3(24, 29, 34, 255),
		)
		p.FillRect5(
			0,
			height-2,
			width,
			2,
			gui.NewQColor3(0, 0, 0, 255),
		)
		p.DestroyQPainter()
	})

	tabs := []*Tab{}
	for i := 0; i < 10; i++ {
		w := widgets.NewQWidget(nil, 0)
		w.SetContentsMargins(10, 0, 10, 0)
		l := widgets.NewQHBoxLayout()
		l.SetContentsMargins(0, 0, 0, 0)
		l.SetSpacing(10)
		fileIcon := svg.NewQSvgWidget(nil)
		fileIcon.SetFixedWidth(14)
		fileIcon.SetFixedHeight(14)
		file := widgets.NewQLabel(nil, 0)
		closeIcon := svg.NewQSvgWidget(nil)
		closeIcon.SetFixedWidth(14)
		closeIcon.SetFixedHeight(14)
		l.AddWidget(fileIcon, 0, 0)
		l.AddWidget(file, 1, 0)
		l.AddWidget(closeIcon, 0, 0)
		w.SetLayout(l)
		tab := &Tab{
			widget:    w,
			layout:    l,
			file:      file,
			fileIcon:  fileIcon,
			closeIcon: closeIcon,
		}
		tabs = append(tabs, tab)
		layout.AddWidget(w)
	}

	return &Tabline{
		widget: widget,
		layout: layout,
		Tabs:   tabs,
	}
}

func initTabline(width int, height int) *Tabline {
	// box := ui.NewHorizontalBox()
	tabline := &Tabline{}
	// tabline.area = ui.NewArea(tabline)
	// tabline.bg = newRGBA(24, 29, 34, 1)
	// tabline.borderBottom = &Border{
	// 	width: 2,
	// 	color: newRGBA(0, 0, 0, 1),
	// }
	// box.SetSize(width, height)
	// box.Append(tabline.area, false)
	// tabline.setSize(width, height)
	return tabline
}

func (t *Tabline) resize(width int, height int) {
	// t.box.SetSize(width, height)
	// t.setSize(width, height)
	// for _, tab := range t.Tabs {
	// 	tab.setSize(tab.width, height)
	// }
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
			return
		}
		tab := t.Tabs[i]
		tab.ID = int(tabMap["tab"].(nvim.Tabpage))
		text := tabMap["name"].(string)
		svgContent := getSvg(getFileType(text), nil)
		tab.fileIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		text = editor.font.defaultFontMetrics.ElidedText(text, core.Qt__ElideLeft, float64(tab.file.Width()), 0)
		tab.file.SetText(text)
		if tab.ID == t.CurrentID {
			tab.widget.SetStyleSheet(".QWidget {border-bottom: 2px solid rgba(81, 154, 186, 1); background-color: rgba(0, 0, 0, 1); } QWidget{color: rgba(212, 215, 214, 1);} ")
			svgContent = getSvg("cross", newRGBA(212, 215, 214, 1))
			tab.closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		} else {
			tab.widget.SetStyleSheet("")
			svgContent = getSvg("cross", newRGBA(147, 161, 161, 1))
			tab.closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		}
		tab.widget.Show()
	}
	for i := len(tabs); i < len(t.Tabs); i++ {
		tab := t.Tabs[i]
		tab.current = false
		tab.widget.Hide()
	}
	// 		widget := widgets.NewQWidget(nil, 0)
	// 		layout := widgets.NewQHBoxLayout2(widget)
	// 		tab := &Tab{
	// 			widget: widget,
	// 			layout: layout,
	// 		}
	// 		t.layout.AddWidget(widget, 0, 0)
	// 		t.Tabs = append(t.Tabs, tab)
	// 	}
	// arg := args[0].([]interface{})
	// t.CurrentID = int(arg[0].(nvim.Tabpage))
	// tabs := arg[1].([]interface{})
	// for i, tabInterface := range tabs {
	// 	tabMap, ok := tabInterface.(map[string]interface{})
	// 	if !ok {
	// 		continue
	// 	}
	// 	if i > len(t.Tabs)-1 {
	// 		chars := 21
	// 		fileiconWidth := editor.font.width * 2
	// 		padding := (t.height - editor.font.height - 2) / 2
	// 		paddingLeft := editor.font.width * 2
	// 		cross := newSvg("cross", editor.font.width*2, editor.font.width*2, newRGBA(255, 255, 255, 1), newRGBA(0, 0, 0, 1))
	// 		fileicon := newSvg("default", fileiconWidth, fileiconWidth, nil, nil)

	// 		box := ui.NewHorizontalBox()
	// 		tab := &Tab{
	// 			box:      box,
	// 			width:    editor.font.width*chars + fileiconWidth + 3*paddingLeft + editor.font.width,
	// 			chars:    chars,
	// 			cross:    cross,
	// 			fileicon: fileicon,
	// 		}
	// 		tab.area = ui.NewArea(tab)
	// 		tab.font = editor.font
	// 		tab.paddingTop = padding
	// 		tab.paddingLeft = paddingLeft + fileiconWidth + editor.font.width
	// 		t.Tabs = append(t.Tabs, tab)
	// 		tab.borderRight = &Border{
	// 			width: 1,
	// 			color: newRGBA(0, 0, 0, 1),
	// 		}
	// 		tab.borderBottom = &Border{
	// 			width: 2,
	// 			color: newRGBA(0, 0, 0, 1),
	// 		}

	// 		ui.QueueMain(func() {
	// 			t.box.Append(box, false)
	// 			box.Append(tab.area, false)
	// 			box.Append(cross.area, false)
	// 			box.Append(fileicon.area, false)
	// 			box.SetSize(tab.width, t.height)
	// 			tab.setSize(tab.width, t.height)
	// 			box.SetPosition(i*tab.width, 0)
	// 			cross.area.SetPosition(tab.width-paddingLeft-editor.font.width, (t.height-cross.height)/2)
	// 			fileicon.setPosition(paddingLeft, (t.height-fileicon.height)/2)
	// 		})
	// 	}
	// 	tab := t.Tabs[i]
	// 	tab.ID = int(tabMap["tab"].(nvim.Tabpage))
	// 	tab.Name = tabMap["name"].(string)
	// 	text := tab.Name
	// 	fileType := getFileType(text)
	// 	tab.fileicon.name = fileType
	// 	if len(text) > tab.chars {
	// 		text = text[len(text)-tab.chars+3 : len(text)]
	// 		text = "..." + text
	// 	}
	// 	tab.text = text
	// 	if tab.ID == t.CurrentID {
	// 		tab.current = true
	// 		tab.borderBottom.color = newRGBA(81, 154, 186, 1)
	// 		tab.bg = newRGBA(0, 0, 0, 1)
	// 		tab.color = newRGBA(212, 215, 214, 1)
	// 		tab.cross.color = newRGBA(212, 215, 214, 1)
	// 		tab.cross.bg = newRGBA(0, 0, 0, 1)
	// 	} else {
	// 		tab.current = false
	// 		tab.borderBottom.color = newRGBA(0, 0, 0, 1)
	// 		tab.bg = newRGBA(24, 29, 34, 1)
	// 		tab.color = editor.Foreground
	// 		tab.cross.color = editor.Foreground
	// 		tab.cross.bg = newRGBA(24, 29, 34, 1)
	// 	}
	// 	ui.QueueMain(func() {
	// 		tab.box.Show()
	// 		tab.area.QueueRedrawAll()
	// 		tab.cross.area.QueueRedrawAll()
	// 		tab.fileicon.area.QueueRedrawAll()
	// 	})
	// }

	// for i := len(tabs); i < len(t.Tabs); i++ {
	// 	tab := t.Tabs[i]
	// 	tab.current = false
	// 	ui.QueueMain(func() {
	// 		tab.box.Hide()
	// 	})
	// }
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

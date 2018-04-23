package editor

import (
 "fmt"
	"path/filepath"
	"strings"

	"github.com/neovim/go-client/nvim"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

// Tabline of the editor
type Tabline struct {
	ws            *Workspace
	widget        *widgets.QWidget
	layout        *widgets.QLayout
	CurrentID     int
	Tabs          []*Tab
	marginDefault int
	marginTop     int
	marginBottom  int
	height        int
}

// Tab in the tabline
type Tab struct {
	t         *Tabline
	widget    *widgets.QWidget
	layout    *widgets.QHBoxLayout
	ID        int
	active    bool
	Name      string
	width     int
	chars     int
	fileIcon  *svg.QSvgWidget
	fileType  string
	closeIcon *svg.QSvgWidget
	file      *widgets.QLabel
	fileText  string
	hidden    bool
}

func (t *Tabline) subscribe() {
	if !t.ws.drawTabline {
		t.widget.Hide()
		return
	}
}

func newHFlowLayout(spacing int, padding int, paddingTop int, rightIdex int, width int) *widgets.QLayout {
	layout := widgets.NewQLayout2()
	items := []*widgets.QLayoutItem{}
	rect := core.NewQRect()
	layout.ConnectSizeHint(func() *core.QSize {
		size := core.NewQSize()
		for _, item := range items {
			size = size.ExpandedTo(item.MinimumSize())
		}
		return size
	})
	if width > 0 {
		layout.ConnectMinimumSize(func() *core.QSize {
			size := core.NewQSize()
			for _, item := range items {
				size = size.ExpandedTo(item.MinimumSize())
			}
			if size.Width() > width {
				size.SetWidth(width)
			}
			// size.SetWidth(0)
			return size
		})
		layout.ConnectMaximumSize(func() *core.QSize {
			size := core.NewQSize()
			for _, item := range items {
				size = size.ExpandedTo(item.MinimumSize())
			}
			// size.SetWidth(width)
			return size
		})
	}
	layout.ConnectAddItem(func(item *widgets.QLayoutItem) {
		items = append(items, item)
	})
	layout.ConnectSetGeometry(func(r *core.QRect) {
		sizes := [][]int{}
		maxWidth := 0
		for _, item := range items {
			sizeHint := item.SizeHint()
			width := sizeHint.Width()
			height := sizeHint.Height()
			size := []int{width, height}
			sizes = append(sizes, size)
			if width > maxWidth {
				maxWidth = width
			}
		}
		y := 0
		for i, item := range items {
			size := sizes[i]
			height := size[1]
			rect.SetRect(0, y, maxWidth, height)
			item.SetGeometry(rect)
			y += height
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

func newVFlowLayout(spacing int, padding int, paddingTop int, rightIdex int, width int) *widgets.QLayout {
	layout := widgets.NewQLayout2()
	items := []*widgets.QLayoutItem{}
	rect := core.NewQRect()
	layout.ConnectSizeHint(func() *core.QSize {
		size := core.NewQSize()
		for _, item := range items {
			size = size.ExpandedTo(item.MinimumSize())
		}
		return size
	})
	if width > 0 {
		layout.ConnectMinimumSize(func() *core.QSize {
			size := core.NewQSize()
			for _, item := range items {
				size = size.ExpandedTo(item.MinimumSize())
			}
			if size.Width() > width {
				size.SetWidth(width)
			}
			size.SetWidth(0)
			return size
		})
		layout.ConnectMaximumSize(func() *core.QSize {
			size := core.NewQSize()
			for _, item := range items {
				size = size.ExpandedTo(item.MinimumSize())
			}
			size.SetWidth(width)
			return size
		})
	}
	layout.ConnectAddItem(func(item *widgets.QLayoutItem) {
		items = append(items, item)
	})
	layout.ConnectSetGeometry(func(r *core.QRect) {
		x := padding
		right := padding
		sizes := [][]int{}
		maxHeight := 0
		totalWidth := r.Width()
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
			y := paddingTop
			if height != maxHeight {
				y = (maxHeight-height)/2 + paddingTop
			}

			if rightIdex > 0 && i >= rightIdex {
				rect.SetRect(totalWidth-width-right, y, width, height)
				item.SetGeometry(rect)
				if width > 0 {
					right += width + spacing
				}
			} else {
				if x+width+padding > totalWidth {
					width = totalWidth - x - padding
					rect.SetRect(x, y, width, height)
					item.SetGeometry(rect)
					break
				}
				rect.SetRect(x, y, width, height)
				item.SetGeometry(rect)
				if width > 0 {
					x += width + spacing
				}
			}
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

func newTabline() *Tabline {
	width := 210
	widget := widgets.NewQWidget(nil, 0)
	layout := widgets.NewQLayout2()
	layout.SetSpacing(0)
	layout.SetContentsMargins(0, 0, 0, 0)
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


	marginDefault := 10
	marginTop := 10
	marginBottom := 10
	tabline := &Tabline{
		widget:        widget,
		layout:        layout,
		marginDefault: marginDefault,
		marginTop:     marginTop,
		marginBottom:  marginBottom,
	}

	tabs := []*Tab{}
	for i := 0; i < 10; i++ {
		w := widgets.NewQWidget(nil, 0)
		w.SetContentsMargins(10, 0, 10, 0)
		l := widgets.NewQHBoxLayout()
		l.SetContentsMargins(0, 0, 0, 0)
		l.SetSpacing(10)
		fileIcon := svg.NewQSvgWidget(nil)
		fileIcon.SetFixedWidth(12)
		fileIcon.SetFixedHeight(12)
		file := widgets.NewQLabel(nil, 0)
		file.SetContentsMargins(0, marginTop, 0, marginBottom)
		closeIcon := svg.NewQSvgWidget(nil)
		closeIcon.SetFixedWidth(14)
		closeIcon.SetFixedHeight(14)
		l.AddWidget(fileIcon, 0, 0)
		l.AddWidget(file, 1, 0)
		l.AddWidget(closeIcon, 0, 0)
		w.SetLayout(l)
		tab := &Tab{
			t:         tabline,
			widget:    w,
			layout:    l,
			file:      file,
			fileIcon:  fileIcon,
			closeIcon: closeIcon,
		}
		tabs = append(tabs, tab)
		layout.AddWidget(w)
		if i > 0 {
			tab.hide()
		}
	}
	tabline.Tabs = tabs
	return tabline
}

func (t *Tab) updateActive() {
 bg := t.t.ws.background
 fg := t.t.ws.foreground
	if t.active {
		t.widget.SetStyleSheet(fmt.Sprintf(".QWidget {border-bottom: 0px solid; background-color: rgba(%d, %d, %d, 1); } QWidget{color: rgba(%d, %d, %d, 1);} ", bg.R, bg.G, bg.B, fg.R, fg.G, fg.B))
		svgContent := t.t.ws.getSvg("cross", newRGBA(fg.R, fg.G, fg.B, 1))
		t.closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	} else {
		t.widget.SetStyleSheet("")
		svgContent := t.t.ws.getSvg("cross", newRGBA(gradColor(fg).R, gradColor(fg).G, gradColor(fg).B, 0.6))
		t.closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	}
}

func (t *Tab) show() {
	if !t.hidden {
		return
	}
	t.hidden = false
	t.widget.Show()
}

func (t *Tab) hide() {
	if t.hidden {
		return
	}
	t.hidden = true
	t.widget.Hide()
}

func (t *Tab) setActive(active bool) {
	if t.active == active {
		return
	}
	t.active = active
	t.updateActive()
}

func (t *Tab) updateFileText() {
	text := t.t.ws.font.defaultFontMetrics.ElidedText(t.fileText, core.Qt__ElideLeft, float64(t.file.Width()), 0)
	t.file.SetText(text)
}

func (t *Tab) updateFileIcon() {
	svgContent := t.t.ws.getSvg(t.fileType, nil)
	t.fileIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
}

func (t *Tabline) updateMargin() {
	for _, tab := range t.Tabs {
		tab.file.SetContentsMargins(0, t.marginTop, 0, t.marginBottom)
		if !tab.hidden {
			tab.hide()
			tab.show()
		}
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
			return
		}
		tab := t.Tabs[i]
		tab.ID = int(tabMap["tab"].(nvim.Tabpage))
		text := tabMap["name"].(string)

		fileType := getFileType(text)
		if fileType != tab.fileType {
			tab.fileType = fileType
			tab.updateFileIcon()
		}

		if text != tab.fileText {
			tab.fileText = text
			tab.updateFileText()
		}

		tab.setActive(tab.ID == t.CurrentID)
		tab.show()
	}
	for i := len(tabs); i < len(t.Tabs); i++ {
		tab := t.Tabs[i]
		tab.setActive(false)
		tab.hide()
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

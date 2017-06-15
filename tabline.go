package gonvim

import (
	"path/filepath"
	"strings"

	"github.com/neovim/go-client/nvim"
	"github.com/therecipe/qt/core"
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

func newVFlowLayout(spacing int, padding int, paddingTop int, rightIdex int) *widgets.QLayout {
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

func initTablineNew() *Tabline {
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
		file.SetContentsMargins(0, 10, 0, 10)
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

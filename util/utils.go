package util

import (
	"strings"
	"os/user"
	"path/filepath"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/widgets"
)

// ReflectToInt converts interface{} to int
func ReflectToInt(iface interface{}) int {
	i, ok := iface.(int64)
	if ok {
		return int(i)
	}
	j, ok := iface.(uint64)
	if ok {
		return int(j)
	}
	k, ok := iface.(int)
	if ok {
		return int(k)
	}
	l, ok := iface.(uint)
	if ok {
		return int(l)
	}
	return 0
}

// ReflectToFloat converts interface{} to float64
func ReflectToFloat(iface interface{}) float64 {
	i, ok := iface.(float64)
	if ok {
		return i
	}
	u, ok := iface.(float32)
	if ok {
		return float64(u)
	}
	return 0
}
// IsZero determines if the value of interface{} is zero
func IsZero(d interface{}) bool {
	if d == nil {
		return false
	}
	switch a := d.(type) {
	case int64:
		if a == 0 {
			return true
		}
	case uint64:
		if a == 0 {
			return true
		}
	}
	return false
}

// IsTrue determines the truth of the value of interface{}
func IsTrue(d interface{}) bool {
	if d == nil {
		return false
	}
	switch a := d.(type) {
	case int64:
		if a == 1 {
			return true
		}
	case uint64:
		if a == 1 {
			return true
		}
	}
	return false
}

// SplitVimscript splits Vimscript read as a character string with line breaks and converts it to a list format character string
func SplitVimscript(s string) string {
	if string(s[0]) == "\n" {
		s = strings.TrimPrefix(s, string("\n"))
	}
	listLines := "["
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		listLines = listLines + `'` + line + `'`
		if i == len(lines)-1 {
			listLines = listLines + "]"
		} else {
			listLines = listLines + ","
		}
	}

	return listLines
}

// DropShadow drops a shadow
func DropShadow(x, y, radius float64, alpha int) *widgets.QGraphicsDropShadowEffect {
	shadow := widgets.NewQGraphicsDropShadowEffect(nil)
	shadow.SetBlurRadius(radius)
	shadow.SetColor(gui.NewQColor3(0, 0, 0, alpha))
	shadow.SetOffset2(x, y)

	return shadow
}

// NewHFlowLayout is a horizontal flow layout. See https://doc.qt.io/qt-5.9/qtwidgets-layouts-flowlayout-example.html
func NewHFlowLayout(spacing int, padding int, paddingTop int, rightIdex int, width int) *widgets.QLayout {
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

// NewVFlowLayout is a vertical flow layout. See https://doc.qt.io/qt-5.9/qtwidgets-layouts-flowlayout-example.html
func NewVFlowLayout(spacing int, padding int, paddingTop int, rightIdex int, width int) *widgets.QLayout {
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

// ExpandTildeToHomeDirectory is a function that expand '~' to absolute home directory path
func ExpandTildeToHomeDirectory(path string) (string, error) {
	if len(path) == 0 || path[0] != '~' {
		return path, nil
	}

	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return filepath.Join(usr.HomeDir, path[1:]), nil
}

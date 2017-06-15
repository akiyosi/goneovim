package gonvim

import (
	"fmt"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/widgets"
)

// PopupMenu is the popupmenu
type PopupMenu struct {
	widget       *widgets.QWidget
	layout       *widgets.QGridLayout
	items        []*PopupItem
	rawItems     []interface{}
	total        int
	showTotal    int
	selected     int
	hidden       bool
	top          int
	scrollBar    *widgets.QWidget
	scrollBarPos int
	scrollCol    *widgets.QWidget
	x            int
	y            int
}

// PopupItem is
type PopupItem struct {
	kindLable *widgets.QLabel
	menuLable *widgets.QLabel
	selected  bool
	hidden    bool
}

func initPopupmenuNew(font *Font) *PopupMenu {
	layout := widgets.NewQGridLayout2()
	layout.SetSpacing(0)
	layout.SetContentsMargins(0, 0, 0, 0)
	scrollCol := widgets.NewQWidget(nil, 0)
	scrollCol.SetContentsMargins(0, 0, 0, 0)
	scrollCol.SetFixedWidth(5)
	scrollBar := widgets.NewQWidget(scrollCol, 0)
	scrollBar.SetFixedWidth(5)
	scrollBar.SetStyleSheet("background-color: #3c3c3c;")
	mainLayout := widgets.NewQHBoxLayout()
	mainLayout.AddLayout(layout, 0)
	mainLayout.AddWidget(scrollCol, 0, 0)
	mainLayout.SetContentsMargins(0, 0, 0, 0)
	mainLayout.SetSpacing(0)
	widget := widgets.NewQWidget(nil, 0)
	widget.SetLayout(mainLayout)
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetStyleSheet("background-color: rgba(14, 17, 18, 1); color: rgba(205, 211, 222, 1);")
	shadow := widgets.NewQGraphicsDropShadowEffect(nil)
	shadow.SetBlurRadius(20)
	shadow.SetColor(gui.NewQColor3(0, 0, 0, 255))
	shadow.SetOffset3(0, 2)
	widget.SetGraphicsEffect(shadow)
	max := 15
	var popupItems []*PopupItem
	for i := 0; i < max; i++ {
		kind := widgets.NewQLabel(nil, 0)
		kind.SetContentsMargins(8, 8, 8, 8)
		kind.SetFont(font.fontNew)
		menu := widgets.NewQLabel(nil, 0)
		menu.SetContentsMargins(8, 8, 8, 8)
		menu.SetFont(font.fontNew)
		layout.AddWidget(kind, i, 0, 0)
		layout.AddWidget(menu, i, 1, 0)

		popupItem := &PopupItem{
			kindLable: kind,
			menuLable: menu,
		}
		popupItems = append(popupItems, popupItem)
	}

	widget.Hide()
	popup := &PopupMenu{
		widget:    widget,
		layout:    layout,
		items:     popupItems,
		total:     max,
		scrollBar: scrollBar,
		scrollCol: scrollCol,
	}
	widget.ConnectCustomEvent(func(event *core.QEvent) {
		widget.Move2(popup.x, popup.y)
	})
	scrollBar.ConnectCustomEvent(func(event *core.QEvent) {
		scrollBar.Move2(0, popup.scrollBarPos)
	})
	return popup
}

func (p *PopupMenu) updateFont(font *Font) {
	for i := 0; i < p.total; i++ {
		popupItem := p.items[i]
		popupItem.kindLable.SetFont(font.fontNew)
		popupItem.menuLable.SetFont(font.fontNew)
	}
}

func (p *PopupMenu) show(args []interface{}) {
	p.hidden = false
	arg := args[0].([]interface{})
	items := arg[0].([]interface{})
	selected := reflectToInt(arg[1])
	row := reflectToInt(arg[2])
	col := reflectToInt(arg[3])
	p.rawItems = items
	p.selected = selected
	p.top = 0

	popupItems := p.items
	itemHeight := editor.font.height + 20
	heightLeft := editor.screen.height - (row+1)*editor.font.lineHeight
	total := heightLeft / itemHeight
	if total < p.total {
		p.showTotal = total
	} else {
		p.showTotal = p.total
	}

	for i := 0; i < p.total; i++ {
		popupItem := popupItems[i]
		if i >= len(items) || i >= total {
			popupItem.hide()
			continue
		}

		item := items[i].([]interface{})
		popupItem.setItem(item, selected == i)
		popupItem.show()
	}

	if len(items) > p.showTotal {
		p.scrollBar.SetFixedHeight(int(float64(p.showTotal) / float64(len(items)) * float64(itemHeight*p.showTotal)))
		p.scrollBarPos = 0
		p.scrollBar.CustomEvent(core.NewQEvent(core.QEvent__Move))
		p.scrollCol.Show()
	} else {
		p.scrollCol.Hide()
	}

	p.x = int(float64(col)*editor.font.truewidth) - popupItems[0].kindLable.Width() - 8
	p.y = (row + 1) * editor.font.lineHeight
	p.widget.CustomEvent(core.NewQEvent(core.QEvent__Move))
	p.widget.Show()
	p.widget.Hide()
	p.widget.Show()
}

func (p *PopupMenu) hide(args []interface{}) {
	p.hidden = true
	p.widget.Hide()
}

func (p *PopupMenu) selectItem(args []interface{}) {
	selected := reflectToInt(args[0].([]interface{})[0])
	if selected == -1 {
		p.scroll(-p.top)
	}
	if selected-p.top >= p.showTotal {
		p.scroll(selected - p.top - p.showTotal + 1)
	}
	if selected >= 0 && selected-p.top < 0 {
		p.scroll(-1)
	}
	for i := 0; i < p.showTotal; i++ {
		popupItem := p.items[i]
		if selected == i+p.top {
			if !popupItem.selected {
				popupItem.selected = true
				bg := editor.selectedBg
				popupItem.menuLable.SetStyleSheet(fmt.Sprintf("background-color: %s;", bg.String()))
			}
		} else {
			if popupItem.selected {
				popupItem.selected = false
				popupItem.menuLable.SetStyleSheet("")
			}
		}
	}
}

func (p *PopupMenu) scroll(n int) {
	// fmt.Println(len(p.rawItems), p.top, n)
	p.top += n
	items := p.rawItems
	popupItems := p.items
	for i := 0; i < p.showTotal; i++ {
		popupItem := popupItems[i]
		item := items[i+p.top].([]interface{})
		popupItem.setItem(item, false)
	}
	p.scrollBarPos = int((float64(p.top) / float64(len(items))) * float64(p.widget.Height()))
	p.scrollBar.CustomEvent(core.NewQEvent(core.QEvent__Move))
	p.widget.Hide()
	p.widget.Show()
}

func (p *PopupItem) setItem(item []interface{}, selected bool) {
	text := item[0].(string)
	kindText := item[1].(string)
	p.setKind(kindText, selected)

	// fg := newRGBA(205, 211, 222, 1)
	// bg := newRGBA(14, 17, 18, 1)
	if selected {
		if !p.selected {
			p.selected = true
			bg := editor.selectedBg
			p.menuLable.SetStyleSheet(fmt.Sprintf("background-color: %s;", bg.String()))
		}
	} else {
		if p.selected {
			p.selected = false
			p.menuLable.SetStyleSheet("")
		}
	}
	p.menuLable.SetText(text)
}

func (p *PopupItem) setKind(kindText string, selected bool) {
	color := newRGBA(151, 195, 120, 1)
	bg := newRGBA(151, 195, 120, 0.2)

	switch kindText {
	case "function", "func":
		kindText = "f"
		color = newRGBA(97, 174, 239, 1)
		bg = newRGBA(97, 174, 239, 0.2)
	case "var", "statement", "instance", "param", "import":
		kindText = "v"
		color = newRGBA(223, 106, 115, 1)
		bg = newRGBA(223, 106, 115, 0.2)
	case "const":
		kindText = "c"
		color = newRGBA(223, 106, 115, 1)
		bg = newRGBA(223, 106, 115, 0.2)
	case "class":
		kindText = "c"
		color = newRGBA(229, 193, 124, 1)
		bg = newRGBA(229, 193, 124, 0.2)
	case "type":
		kindText = "t"
		color = newRGBA(229, 193, 124, 1)
		bg = newRGBA(229, 193, 124, 0.2)
	case "module":
		kindText = "m"
		color = newRGBA(42, 161, 152, 1)
		bg = newRGBA(42, 161, 152, 0.2)
	case "keyword":
		kindText = "k"
		color = newRGBA(42, 161, 152, 1)
		bg = newRGBA(42, 161, 152, 0.2)
	case "package":
		kindText = "p"
		color = newRGBA(42, 161, 152, 1)
		bg = newRGBA(42, 161, 152, 0.2)
	default:
		kindText = "b"
	}
	p.kindLable.SetStyleSheet(fmt.Sprintf("background-color: %s; color: %s;", bg.String(), color.String()))
	p.kindLable.SetText(kindText)
}

func (p *PopupItem) hide() {
	if p.hidden {
		return
	}
	p.hidden = true
	p.kindLable.Hide()
	p.menuLable.Hide()
}

func (p *PopupItem) show() {
	if !p.hidden {
		return
	}
	p.hidden = false
	p.kindLable.Show()
	p.menuLable.Show()
}

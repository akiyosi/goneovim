package gonvim

import (
	"fmt"

	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/widgets"
)

// PopupMenu is the popupmenu
type PopupMenu struct {
	widget    *widgets.QWidget
	layout    *widgets.QGridLayout
	items     []*PopupItem
	rawItems  []interface{}
	total     int
	showTotal int
	selected  int
	hidden    bool
	top       int
	scrollBar *widgets.QWidget
	scrollCol *widgets.QWidget
}

// PopupItem is
type PopupItem struct {
	kindLable *widgets.QLabel
	menuLable *widgets.QLabel
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
	scrollBar.SetStyleSheet("background-color: rgba(255,255,255,0.5);")
	mainLayout := widgets.NewQHBoxLayout()
	mainLayout.AddLayout(layout, 0)
	mainLayout.AddWidget(scrollCol, 0, 0)
	mainLayout.SetContentsMargins(0, 0, 0, 0)
	mainLayout.SetSpacing(0)
	widget := widgets.NewQWidget(nil, 0)
	widget.SetLayout(mainLayout)
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetStyleSheet("background-color: rgba(14, 17, 18, 1);")
	shadow := widgets.NewQGraphicsDropShadowEffect(nil)
	shadow.SetBlurRadius(20)
	shadow.SetColor(gui.NewQColor3(0, 0, 0, 255))
	shadow.SetOffset3(0, 2)
	widget.SetGraphicsEffect(shadow)
	max := 15
	var popupItems []*PopupItem
	for i := 0; i < max; i++ {
		kind := widgets.NewQLabel(nil, 0)
		kind.SetContentsMargins(10, 10, 10, 10)
		kind.SetFont(font.fontNew)
		menu := widgets.NewQLabel(nil, 0)
		menu.SetContentsMargins(10, 10, 10, 10)
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
	return &PopupMenu{
		widget:    widget,
		layout:    layout,
		items:     popupItems,
		total:     max,
		scrollBar: scrollBar,
		scrollCol: scrollCol,
	}
}

func initPopupmenu() *PopupMenu {
	// total := 10
	// box := ui.NewHorizontalBox()
	// var popupItems []*PopupItem
	// for i := 0; i < total; i++ {
	// 	kindSpanHandler := &SpanHandler{}
	// 	kindSpan := ui.NewArea(kindSpanHandler)
	// 	kindSpanHandler.area = kindSpan

	// 	menuSpanHandler := &SpanHandler{}
	// 	menuSpan := ui.NewArea(menuSpanHandler)
	// 	menuSpanHandler.area = menuSpan

	// 	popupItem := &PopupItem{
	// 		kind: kindSpanHandler,
	// 		menu: menuSpanHandler,
	// 	}

	// 	popupItems = append(popupItems, popupItem)
	// 	box.Append(kindSpan, false)
	// 	box.Append(menuSpan, false)
	// }
	// box.SetShadow(0, 2, 0, 0, 0, 1, 4)
	// box.Hide()

	return &PopupMenu{
	// box:   box,
	// items: popupItems,
	// total: total,
	}
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
	// itemHeightReal := popupItems[0].menuLable.Height()
	// if itemHeightReal < itemHeight {
	// 	itemHeight = itemHeightReal
	// }
	// fmt.Println(itemHeight, itemHeightReal)
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
			popupItem.kindLable.Hide()
			popupItem.menuLable.Hide()
			continue
		}

		item := items[i].([]interface{})
		popupItem.setItem(item, selected == i)
		popupItem.kindLable.Show()
		popupItem.menuLable.Show()
	}

	p.widget.Move2(
		int(float64(col)*editor.font.truewidth)-popupItems[0].kindLable.Width()-10,
		(row+1)*editor.font.lineHeight,
	)
	p.widget.Show()

	if len(items) > p.showTotal {
		p.scrollBar.SetFixedHeight(int(float64(p.showTotal) / float64(len(items)) * float64(itemHeight*p.showTotal)))
		p.scrollBar.Move2(0, 0)
		p.scrollCol.Show()
	} else {
		p.scrollCol.Hide()
	}
	// popupItems := p.items
	// i := 0
	// kindWidth := 0
	// menuWidthMax := 0
	// heightSum := 0
	// height := 0
	// for i = 0; i < p.total; i++ {
	// 	popupItem := popupItems[i]
	// 	if i >= len(items) {
	// 		popupItem.hide()
	// 		continue
	// 	}

	// 	item := items[i].([]interface{})
	// 	popupItem.setItem(item, selected == i)

	// 	var menuWidth int
	// 	menuWidth, height = popupItem.menu.getSize()
	// 	kindWidth, height = popupItem.kind.getSize()

	// 	if menuWidth > menuWidthMax {
	// 		menuWidthMax = menuWidth
	// 	}
	// 	y := heightSum
	// 	heightSum += height
	// 	ui.QueueMain(func() {
	// 		popupItem.kind.area.SetPosition(0, y)
	// 		popupItem.menu.area.SetPosition(kindWidth, y)
	// 	})
	// }

	// for i = 0; i < p.total; i++ {
	// 	if i >= len(items) {
	// 		continue
	// 	}
	// 	popupItem := popupItems[i]
	// 	ui.QueueMain(func() {
	// 		popupItem.kind.area.SetSize(kindWidth, height)
	// 		popupItem.kind.area.Show()
	// 		popupItem.kind.area.QueueRedrawAll()
	// 		popupItem.menu.area.SetSize(menuWidthMax, height)
	// 		popupItem.menu.area.Show()
	// 		popupItem.menu.area.QueueRedrawAll()
	// 	})
	// }

	// ui.QueueMain(func() {
	// 	p.box.SetPosition(
	// 		int(float64(col)*editor.font.truewidth)-kindWidth-p.items[0].menu.paddingLeft,
	// 		(row+1)*editor.font.lineHeight,
	// 	)
	// 	p.box.SetSize(menuWidthMax+kindWidth, heightSum)
	// 	p.box.Show()
	// })
}

func (p *PopupMenu) hide(args []interface{}) {
	p.hidden = true
	p.widget.Hide()
}

func (p *PopupMenu) selectItem(args []interface{}) {
	selected := reflectToInt(args[0].([]interface{})[0])
	if selected == -1 {
		p.top = 0
	}
	if selected-p.top >= p.showTotal {
		p.scroll(selected - p.top - p.showTotal + 1)
	}
	if selected >= 0 && selected-p.top < 0 {
		p.scroll(-1)
	}
	fg := newRGBA(205, 211, 222, 1)
	for i := 0; i < p.showTotal; i++ {
		popupItem := p.items[i]
		bg := newRGBA(14, 17, 18, 1)
		if selected == i+p.top {
			bg = editor.selectedBg
		}
		popupItem.menuLable.SetStyleSheet(fmt.Sprintf("background-color: %s; color: %s;", bg.String(), fg.String()))
	}
}

func (p *PopupMenu) scroll(n int) {
	p.top += n
	items := p.rawItems
	popupItems := p.items
	for i := 0; i < p.showTotal; i++ {
		popupItem := popupItems[i]
		item := items[i+p.top].([]interface{})
		popupItem.setItem(item, false)
	}
	p.scrollBar.Move2(0, int((float64(p.top)/float64(len(items)))*float64(p.widget.Height())))
	p.widget.Hide()
	p.widget.Show()
}

func (p *PopupItem) setItem(item []interface{}, selected bool) {
	text := item[0].(string)
	kindText := item[1].(string)
	p.setKind(kindText, selected)

	fg := newRGBA(205, 211, 222, 1)
	bg := newRGBA(14, 17, 18, 1)
	if selected {
		bg = editor.selectedBg
	}
	p.menuLable.SetStyleSheet(fmt.Sprintf("background-color: %s; color: %s;", bg.String(), fg.String()))
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
	// ui.QueueMain(func() {
	// 	p.kind.area.Hide()
	// 	p.menu.area.Hide()
	// })
}

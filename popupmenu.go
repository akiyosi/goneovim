package gonvim

import "github.com/dzhou121/ui"

// PopupMenu is the popupmenu
type PopupMenu struct {
	box   *ui.Box
	items []*PopupItem
	total int
}

// PopupItem is
type PopupItem struct {
	kind *SpanHandler
	menu *SpanHandler
}

func initPopupmenu() *PopupMenu {
	total := 10
	box := ui.NewHorizontalBox()
	var popupItems []*PopupItem
	for i := 0; i < total; i++ {
		kindSpanHandler := &SpanHandler{}
		kindSpan := ui.NewArea(kindSpanHandler)
		kindSpanHandler.span = kindSpan

		menuSpanHandler := &SpanHandler{}
		menuSpan := ui.NewArea(menuSpanHandler)
		menuSpanHandler.span = menuSpan

		popupItem := &PopupItem{
			kind: kindSpanHandler,
			menu: menuSpanHandler,
		}

		popupItems = append(popupItems, popupItem)
		box.Append(kindSpan, false)
		box.Append(menuSpan, false)
	}
	box.Hide()

	return &PopupMenu{
		box:   box,
		items: popupItems,
		total: total,
	}
}

func (p *PopupMenu) show(args []interface{}) {
	arg := args[0].([]interface{})
	items := arg[0].([]interface{})
	selected := reflectToInt(arg[1])
	row := reflectToInt(arg[2])
	col := reflectToInt(arg[3])

	popupItems := p.items
	i := 0
	widthMax := 0
	heightSum := 0
	for i = 0; i < p.total; i++ {
		popupItem := popupItems[i]
		if i >= len(items) {
			popupItem.hide()
			continue
		}

		item := items[i].([]interface{})
		popupItem.setItem(item, selected == i)

		width, height := popupItem.menu.getSize()
		if width > widthMax {
			widthMax = width
		}
		y := heightSum
		heightSum += height
		ui.QueueMain(func() {
			popupItem.menu.span.SetPosition(0, y)
		})
	}

	for i = 0; i < p.total; i++ {
		if i >= len(items) {
			continue
		}
		popupItem := popupItems[i]
		_, height := popupItem.menu.getSize()
		ui.QueueMain(func() {
			popupItem.menu.span.SetSize(widthMax, height)
			popupItem.menu.span.Show()
			popupItem.menu.span.QueueRedrawAll()
		})
	}

	ui.QueueMain(func() {
		p.box.SetPosition(
			col*editor.fontWidth,
			(row+1)*editor.LineHeight,
		)
		p.box.SetSize(widthMax, heightSum)
		p.box.Show()
	})
}

func (p *PopupMenu) hide(args []interface{}) {
	ui.QueueMain(func() {
		p.box.Hide()
	})
}

func (p *PopupMenu) selectItem(args []interface{}) {
	selected := reflectToInt(args[0].([]interface{})[0])
	for i := 0; i < p.total; i++ {
		popupItem := p.items[i]
		if selected == i {
			popupItem.menu.SetBackground(newRGBA(81, 154, 186, 1))
			ui.QueueMain(func() {
				popupItem.menu.span.QueueRedrawAll()
			})
		} else {
			popupItem.menu.SetBackground(newRGBA(14, 17, 18, 1))
			ui.QueueMain(func() {
				popupItem.menu.span.QueueRedrawAll()
			})
		}
	}
}

func (p *PopupItem) setItem(item []interface{}, selected bool) {
	text := item[0].(string)
	kindText := item[1].(string)
	p.setKind(kindText, selected)

	fg := newRGBA(205, 211, 222, 1)
	p.menu.SetColor(fg)
	if selected {
		p.menu.SetBackground(newRGBA(81, 154, 186, 1))
	} else {
		p.menu.SetBackground(newRGBA(14, 17, 18, 1))
	}
	p.menu.SetFont(editor.font)
	p.menu.SetText(text)
	p.menu.paddingLeft = 10
	p.menu.paddingRight = 10
	p.menu.paddingTop = 10
	p.menu.paddingBottom = 10
}

func (p *PopupItem) setKind(kindText string, selected bool) {
	switch kindText {
	case "function":
		kindText = "f"
		p.kind.SetColor(newRGBA(97, 174, 239, 1))
		p.kind.SetBackground(newRGBA(97, 174, 239, 0.2))
	default:
		kindText = "b"
		p.kind.SetColor(newRGBA(151, 195, 120, 1))
		p.kind.SetBackground(newRGBA(151, 195, 120, 0.2))
	}
	p.kind.SetText(kindText)
}

func (p *PopupItem) hide() {
	ui.QueueMain(func() {
		p.kind.span.Hide()
		p.menu.span.Hide()
	})
}

package gonvim

import (
	"time"

	"github.com/dzhou121/ui"
)

// PopupMenu is the popupmenu
type PopupMenu struct {
	box    *ui.Box
	items  []*PopupItem
	total  int
	hidden bool
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
	box.SetShadow(0, 2, 0, 0, 0, 1, 4)
	box.Hide()

	return &PopupMenu{
		box:   box,
		items: popupItems,
		total: total,
	}
}

func (p *PopupMenu) show(args []interface{}) {
	p.hidden = false
	arg := args[0].([]interface{})
	items := arg[0].([]interface{})
	selected := reflectToInt(arg[1])
	row := reflectToInt(arg[2])
	col := reflectToInt(arg[3])

	popupItems := p.items
	i := 0
	kindWidth := 0
	menuWidthMax := 0
	heightSum := 0
	height := 0
	for i = 0; i < p.total; i++ {
		popupItem := popupItems[i]
		if i >= len(items) {
			popupItem.hide()
			continue
		}

		item := items[i].([]interface{})
		popupItem.setItem(item, selected == i)

		var menuWidth int
		menuWidth, height = popupItem.menu.getSize()
		kindWidth, height = popupItem.kind.getSize()

		if menuWidth > menuWidthMax {
			menuWidthMax = menuWidth
		}
		y := heightSum
		heightSum += height
		ui.QueueMain(func() {
			popupItem.kind.span.SetPosition(0, y)
			popupItem.menu.span.SetPosition(kindWidth, y)
		})
	}

	for i = 0; i < p.total; i++ {
		if i >= len(items) {
			continue
		}
		popupItem := popupItems[i]
		ui.QueueMain(func() {
			popupItem.kind.span.SetSize(kindWidth, height)
			popupItem.kind.span.Show()
			popupItem.kind.span.QueueRedrawAll()
			popupItem.menu.span.SetSize(menuWidthMax, height)
			popupItem.menu.span.Show()
			popupItem.menu.span.QueueRedrawAll()
		})
	}

	ui.QueueMain(func() {
		p.box.SetPosition(
			col*editor.font.width-kindWidth-p.items[0].menu.paddingLeft,
			(row+1)*editor.font.lineHeight,
		)
		p.box.SetSize(menuWidthMax+kindWidth, heightSum)
		p.box.Show()
	})
}

func (p *PopupMenu) hide(args []interface{}) {
	p.hidden = true

	time.AfterFunc(50*time.Millisecond, func() {
		if p.hidden {
			ui.QueueMain(func() {
				p.box.Hide()
			})
		}
	})
}

func (p *PopupMenu) selectItem(args []interface{}) {
	selected := reflectToInt(args[0].([]interface{})[0])
	for i := 0; i < p.total; i++ {
		popupItem := p.items[i]
		if selected == i {
			popupItem.menu.SetBackground(editor.selectedBg)
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
		p.menu.SetBackground(editor.selectedBg)
	} else {
		p.menu.SetBackground(newRGBA(14, 17, 18, 1))
	}
	p.menu.SetFont(editor.font.font)
	p.menu.SetText(text)

	p.menu.paddingLeft = 10
	p.menu.paddingRight = 10
	p.menu.paddingTop = 8
	p.menu.paddingBottom = 8

	p.kind.paddingLeft = 10
	p.kind.paddingRight = 10
	p.kind.paddingTop = 8
	p.kind.paddingBottom = 8
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
	p.kind.SetColor(color)
	p.kind.SetBackground(bg)
	p.kind.SetFont(editor.font.font)
	p.kind.SetText(kindText)
}

func (p *PopupItem) hide() {
	ui.QueueMain(func() {
		p.kind.span.Hide()
		p.menu.span.Hide()
	})
}

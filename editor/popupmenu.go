package editor

import (
	"errors"
	"fmt"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

// PopupMenu is the popupmenu
type PopupMenu struct {
	ws              *Workspace
	widget          *widgets.QWidget
	layout          *widgets.QGridLayout
	items           []*PopupItem
	rawItems        []interface{}
	total           int
	showTotal       int
	selected        int
	hidden          bool
	top             int
	scrollBar       *widgets.QWidget
	scrollBarPos    int
	scrollBarHeight int
	scrollCol       *widgets.QWidget
	x               int
	y               int
}

// PopupItem is
type PopupItem struct {
	kindLabel  *widgets.QLabel
	kindText   string
	kind       string
	kindWidget *widgets.QWidget
	kindIcon   *svg.QSvgWidget
	detailText string

	menuLabel       *widgets.QLabel
	menuText        string
	menuTextRequest string

	detailLabel       *widgets.QLabel
	detailTextRequest string

	selected        bool
	selectedRequest bool

	kindColor *RGBA
	kindBg    *RGBA
	hidden    bool
}

func initPopupmenuNew(font *Font) *PopupMenu {
	layout := widgets.NewQGridLayout2()
	layout.SetSpacing(0)
	layout.SetContentsMargins(0, editor.iconSize/5, 0, 0)
	scrollCol := widgets.NewQWidget(nil, 0)
	scrollCol.SetContentsMargins(0, 0, 0, 0)
	scrollCol.SetFixedWidth(5)
	scrollBar := widgets.NewQWidget(scrollCol, 0)
	scrollBar.SetFixedWidth(5)
	//scrollBar.SetStyleSheet("background-color: #3c3c3c;")
	mainLayout := widgets.NewQHBoxLayout()
	mainLayout.AddLayout(layout, 0)
	mainLayout.AddWidget(scrollCol, 0, 0)
	mainLayout.SetContentsMargins(0, 0, 0, 0)
	mainLayout.SetSpacing(0)
	widget := widgets.NewQWidget(nil, 0)
	widget.SetLayout(mainLayout)
	widget.SetContentsMargins(1, 1, 1, 1)
	//widget.SetStyleSheet("* {background-color: rgba(24, 29, 34, 1); color: rgba(205, 211, 222, 1);}")
	max := 15
	var popupItems []*PopupItem

	margin := editor.config.Editor.Linespace/2 + 2
	for i := 0; i < max; i++ {
		kindWidget := widgets.NewQWidget(nil, 0)
		kindlayout := widgets.NewQHBoxLayout()
		kindlayout.SetContentsMargins(editor.iconSize/2, 0, editor.iconSize/2, 0)
		kindWidget.SetLayout(kindlayout)
		kindIcon := svg.NewQSvgWidget(nil)
		kindIcon.SetFixedSize2(editor.iconSize, editor.iconSize)
		kindlayout.AddWidget(kindIcon, 0, 0)

		menu := widgets.NewQLabel(nil, 0)
		menu.SetContentsMargins(1, margin, margin, margin)
		menu.SetFont(font.fontNew)
		detail := widgets.NewQLabel(nil, 0)
		detail.SetContentsMargins(margin, margin, margin, margin)
		detail.SetFont(font.fontNew)
		detail.SetObjectName("detailpopup")

		layout.AddWidget(kindWidget, i, 0, 0)
		layout.AddWidget(menu, i, 1, 0)
		layout.AddWidget(detail, i, 2, 0)

		popupItem := &PopupItem{
			// kindLabel:   kind,
			kindWidget:  kindWidget,
			kindIcon:    kindIcon,
			menuLabel:   menu,
			detailLabel: detail,
		}
		popupItems = append(popupItems, popupItem)
	}

	popup := &PopupMenu{
		widget:    widget,
		layout:    layout,
		items:     popupItems,
		total:     max,
		scrollBar: scrollBar,
		scrollCol: scrollCol,
	}

	shadow := widgets.NewQGraphicsDropShadowEffect(nil)
	shadow.SetBlurRadius(38)
	shadow.SetColor(gui.NewQColor3(0, 0, 0, 200))
	shadow.SetOffset3(-2, 6)
	popup.widget.SetGraphicsEffect(shadow)

	return popup
}

func (p *PopupMenu) updateFont(font *Font) {
	for i := 0; i < p.total; i++ {
		popupItem := p.items[i]
		// popupItem.kindLabel.SetFont(font.fontNew)
		popupItem.menuLabel.SetFont(font.fontNew)
		popupItem.detailLabel.SetFont(font.fontNew)
	}
}

func (p *PopupMenu) setColor() {
	fg := editor.colors.widgetFg.String()
	inactiveFg := editor.colors.inactiveFg.String()
	bg := editor.colors.widgetBg.String()
	p.scrollBar.SetStyleSheet(fmt.Sprintf("background-color: %s;", inactiveFg))
	p.widget.SetStyleSheet(fmt.Sprintf("* {background-color: %s; color: %s;} #detailpopup { color: %s; }", bg, fg, inactiveFg))
}

func (p *PopupMenu) showItems(args []interface{}) {
	arg := args[0].([]interface{})
	items := arg[0].([]interface{})
	selected := reflectToInt(arg[1])
	row := reflectToInt(arg[2])
	col := reflectToInt(arg[3])
	p.rawItems = items
	p.selected = selected
	p.top = 0

	popupItems := p.items
	itemHeight := p.ws.font.height + 20
	heightLeft := p.ws.screen.height - (row+1)*p.ws.font.lineHeight
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
		p.scrollBarHeight = int(float64(p.showTotal) / float64(len(items)) * float64(itemHeight*p.showTotal))
		p.scrollBarPos = 0
		p.scrollBar.SetFixedHeight(p.scrollBarHeight)
		p.scrollBar.Move2(0, p.scrollBarPos)
		p.scrollCol.Show()
	} else {
		p.scrollCol.Hide()
	}

	xpos := int(float64(col) * p.ws.font.truewidth)
	popupWidth := editor.iconSize + popupItems[0].menuLabel.Width() // + popupItems[0].detailLabel.Width()
	if xpos+popupWidth >= p.ws.screen.widget.Width() {
		xpos = p.ws.screen.widget.Width() - popupWidth - 5
	}

	p.widget.Move2(
		//int(float64(col)*p.ws.font.truewidth)-popupItems[0].kindLabel.Width()-8,
		xpos,
		(row+1)*p.ws.font.lineHeight,
	)
	p.hide()
	p.show()
}

func (p *PopupMenu) show() {
	p.widget.Show()
}

func (p *PopupMenu) hide() {
	p.widget.Hide()
}

func (p *PopupMenu) selectItem(args []interface{}) {
	selected := reflectToInt(args[0].([]interface{})[0])
	if selected == -1 && p.top > 0 {
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
		popupItem.setSelected(selected == i+p.top)
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
	p.scrollBar.Move2(0, p.scrollBarPos)
	p.hide()
	p.show()
}

func (p *PopupItem) updateKind() {
	p.kindLabel.SetStyleSheet(fmt.Sprintf("background-color: %s; color: %s;", p.kindBg.String(), p.kindColor.String()))
	p.kindLabel.SetText(p.kindText)
	p.kindLabel.AdjustSize()
}

func (p *PopupItem) updateMenu() {
	if p.selected != p.selectedRequest {
		p.selected = p.selectedRequest
		if p.selected {
			p.kindWidget.SetStyleSheet(fmt.Sprintf("background-color: %s;", editor.colors.selectedBg.String()))
			p.menuLabel.SetStyleSheet(fmt.Sprintf("background-color: %s;", editor.colors.selectedBg.String()))
			p.detailLabel.SetStyleSheet(fmt.Sprintf("background-color: %s;", editor.colors.selectedBg.String()))
		} else {
			p.kindWidget.SetStyleSheet("")
			p.menuLabel.SetStyleSheet("")
			p.detailLabel.SetStyleSheet("")
		}
	}
	if p.menuTextRequest != p.menuText {
		p.menuText = p.menuTextRequest
		p.menuLabel.SetText(p.menuText)
		p.detailText = p.detailTextRequest
		p.detailLabel.SetText(p.detailText)
	}
	p.menuLabel.AdjustSize()
	p.detailLabel.AdjustSize()
}

func (p *PopupItem) setSelected(selected bool) {
	p.selectedRequest = selected
	p.updateMenu()
}

func (p *PopupItem) setItem(item []interface{}, selected bool) {
	text := item[0].(string)
	kindText := item[1].(string)
	detail := fmt.Sprintf("%s", item[2:])

	p.setKind(kindText, selected)
	p.menuTextRequest = text
	p.detailTextRequest = detail[1 : len(detail)-1] // cut "[" and "]"
	p.setSelected(selected)
}

func detectVimCompleteMode() (string, error) {
	w := editor.workspaces[editor.active]

	var isEnableCompleteMode int
	var enableCompleteMode interface{}
	var kind interface{}
	w.nvim.Eval("exists('*complete_mode')", &enableCompleteMode)
	switch enableCompleteMode.(type) {
	case int64:
		isEnableCompleteMode = int(enableCompleteMode.(int64))
	case uint64:
		isEnableCompleteMode = int(enableCompleteMode.(uint64))
	case uint:
		isEnableCompleteMode = int(enableCompleteMode.(uint))
	case int:
		isEnableCompleteMode = enableCompleteMode.(int)
	}
	if isEnableCompleteMode == 1 {
		w.nvim.Eval("complete_mode()", &kind)
		if kind != nil {
			return kind.(string), nil
		} else {
			return "", nil
		}
	}
	return "", errors.New("Does not exits complete_mode()")

}

func (p *PopupItem) setKind(kindText string, selected bool) {
	switch kindText {
	case "function", "func":
		icon := editor.getSvg("lsp_function", nil)
		p.kindIcon.Load2(core.NewQByteArray2(icon, len(icon)))
	case "var", "statement", "instance", "param", "import":
		icon := editor.getSvg("lsp_variable", nil)
		p.kindIcon.Load2(core.NewQByteArray2(icon, len(icon)))
	case "class", "type", "struct":
		icon := editor.getSvg("lsp_class", nil)
		p.kindIcon.Load2(core.NewQByteArray2(icon, len(icon)))
	case "const","module", "keyword", "package":
		icon := editor.getSvg("lsp_"+kindText, nil)
		p.kindIcon.Load2(core.NewQByteArray2(icon, len(icon)))
	default:
		kind, err := detectVimCompleteMode()
		if err == nil {
			p.kind = kind
		}
		switch p.kind {
		case "keyword",
			"whole_line",
			"files",
			"tags",
			"path_defines",
			"path_patterns",
			"path_dictionary",
			"path_thesaurus",
			"path_cmdline",
			"path_function",
			"path_omni",
			"path_spell",
			"path_eval":
			icon := editor.getSvg("vim_"+p.kind, nil)
			p.kindIcon.Load2(core.NewQByteArray2(icon, len(icon)))
		default:
			icon := editor.getSvg("vim_unknown", nil)
			p.kindIcon.Load2(core.NewQByteArray2(icon, len(icon)))
		}
	}
	// if kindText != p.kindText {
	// 	p.kindText = kindText
	// 	p.kindColor = color
	// 	p.kindBg = bg
	// 	p.updateKind()
	// }
}

func (p *PopupItem) hide() {
	if p.hidden {
		return
	}
	p.hidden = true
	//p.kindLabel.Hide()
	p.kindIcon.Hide()
	p.menuLabel.Hide()
	p.detailLabel.Hide()
}

func (p *PopupItem) show() {
	if !p.hidden {
		return
	}
	p.hidden = false
	//p.kindLabel.Show()
	p.kindIcon.Show()
	p.menuLabel.Show()
	p.detailLabel.Show()
}

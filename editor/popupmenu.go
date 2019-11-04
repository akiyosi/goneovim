package editor

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"

	"github.com/akiyosi/goneovim/util"
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

	detailWidget      *widgets.QWidget
	detailLabel       *widgets.QLabel
	detailTextRequest string

	selected        bool
	selectedRequest bool

	kindColor *RGBA
	kindBg    *RGBA
	hidden    bool
}

func initPopupmenuNew() *PopupMenu {
	layout := widgets.NewQGridLayout2()
	layout.SetSpacing(0)
	layout.SetContentsMargins(0, editor.iconSize/5, 0, 0)

	scrollCol := widgets.NewQWidget(nil, 0)
	scrollCol.SetContentsMargins(0, 0, 0, 0)
	scrollCol.SetFixedWidth(5)
	scrollBar := widgets.NewQWidget(scrollCol, 0)
	scrollBar.SetFixedWidth(5)

	mainLayout := widgets.NewQHBoxLayout()
	mainLayout.AddLayout(layout, 0)
	mainLayout.AddWidget(scrollCol, 0, 0)
	mainLayout.SetContentsMargins(0, 0, 0, 0)
	mainLayout.SetSpacing(0)

	widget := widgets.NewQWidget(nil, 0)
	widget.SetLayout(mainLayout)
	widget.SetContentsMargins(1, 1, 1, 1)

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

		kindLabel := widgets.NewQLabel(nil, 0)
		kindLabel.SetContentsMargins(margin, margin, margin, margin)
		kindLabel.SetObjectName("kindlabelpopup")

		detailWidget := widgets.NewQWidget(nil, 0)
		detailLayout := widgets.NewQHBoxLayout()
		detailLayout.SetContentsMargins(margin, 0, margin, 0)

		detailWidget.SetLayout(detailLayout)
		detailWidget.SetObjectName("detailpopup")

		detailLabel := widgets.NewQLabel(nil, 0)
        detailLayout.AddWidget(detailLabel, 0, 0)

		layout.AddWidget2(kindWidget, i, 0, 0)
		layout.AddWidget2(menu, i, 1, 0)
		layout.AddWidget2(kindLabel, i, 2, 0)
		layout.AddWidget2(detailWidget, i, 3, 0)

		popupItem := &PopupItem{
			kindLabel:    kindLabel,
			kindWidget:   kindWidget,
			kindIcon:     kindIcon,
			menuLabel:    menu,
			detailWidget: detailWidget,
			detailLabel:  detailLabel,
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

	popup.widget.SetGraphicsEffect(util.DropShadow(-2, 6, 40, 200))

	return popup
}

func (p *PopupMenu) updateFont(font *Font) {
	for i := 0; i < p.total; i++ {
		popupItem := p.items[i]
		popupItem.kindLabel.SetFont(font.fontNew)
		popupItem.menuLabel.SetFont(font.fontNew)
		popupItem.detailLabel.SetFont(font.fontNew)
	}
}

func (p *PopupMenu) setColor() {
	fg := editor.colors.widgetFg.String()
	inactiveFg := editor.colors.inactiveFg.String()
	bg := editor.colors.widgetBg
	transparent := transparent()
	p.scrollBar.SetStyleSheet(fmt.Sprintf("background-color: %s;", inactiveFg))
	p.widget.SetStyleSheet(fmt.Sprintf("* {background-color: rgba(%d, %d, %d, %f); color: %s;} #kindlabelpopup { color: %s; }", bg.R, bg.G, bg.B, transparent, fg, inactiveFg))
}

func (p *PopupMenu) setPumblend(arg interface{}) {
	var pumblend int
	var err error
	switch val := arg.(type) {
	case string:
		pumblend, err = strconv.Atoi(val)
		if err != nil {
			return
		}
	case int32: // can't combine these in a type switch without compile error
		pumblend = int(val)
	case int64:
		pumblend = int(val)
	default:
		return
	}
	alpha := float64(100-pumblend) / float64(100)
	if alpha < 0 {
		alpha = 0
	}

	fg := editor.colors.widgetFg.String()
	inactiveFg := editor.colors.inactiveFg.String()
	bg := editor.colors.widgetBg
	p.scrollBar.SetStyleSheet(fmt.Sprintf("background-color: %s;", inactiveFg))
	p.widget.SetStyleSheet(fmt.Sprintf("* {background-color: rgba(%d, %d, %d, %f); color: %s;} #kindlabelpopup { color: %s; }", bg.R, bg.G, bg.B, alpha, fg, inactiveFg))
}

func (p *PopupMenu) showItems(args []interface{}) {
	arg := args[0].([]interface{})
	items := arg[0].([]interface{})
	selected := util.ReflectToInt(arg[1])
	row := util.ReflectToInt(arg[2])
	col := util.ReflectToInt(arg[3])
	gridid := util.ReflectToInt(arg[4])

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

	popupWidth := editor.iconSize + popupItems[0].menuLabel.Width()

	x := int(float64(col) * p.ws.font.truewidth)
	y := row*p.ws.font.lineHeight + p.ws.font.lineHeight
	if p.ws.drawTabline {
		y += p.ws.tabline.widget.Height()
	}
  
	if x+popupWidth >= p.ws.screen.widget.Width() {
		x = p.ws.screen.widget.Width() - popupWidth - 5
	}
	win := p.ws.screen.windows[gridid]
	if win != nil {
		x += int(float64(win.pos[0]) * p.ws.font.truewidth)
		y += win.pos[1] * p.ws.font.lineHeight
	}

	p.widget.Move2(x, y)
	p.hide()
	p.show()
}

func (p *PopupMenu) show() {
	p.widget.Raise()
	p.widget.Show()
}

func (p *PopupMenu) hide() {
	p.widget.Hide()
}

func (p *PopupMenu) selectItem(args []interface{}) {
	selected := util.ReflectToInt(args[0].([]interface{})[0])
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
			p.kindWidget.SetStyleSheet(fmt.Sprintf("background-color: %s;", editor.colors.selectedBg.StringTransparent()))
			p.menuLabel.SetStyleSheet(fmt.Sprintf("background-color: %s;", editor.colors.selectedBg.StringTransparent()))
			p.detailLabel.SetStyleSheet(fmt.Sprintf("background-color: %s;", editor.colors.selectedBg.StringTransparent()))
			p.kindLabel.SetStyleSheet(fmt.Sprintf("background-color: %s;", editor.colors.selectedBg.StringTransparent()))

			p.detailLabel.Show()
		} else {
			p.kindWidget.SetStyleSheet("background-color: rgba(0, 0, 0, 0);")
			p.menuLabel.SetStyleSheet("background-color: rgba(0, 0, 0, 0);")
			p.detailLabel.SetStyleSheet("background-color: rgba(0, 0, 0, 0);")
			p.kindLabel.SetStyleSheet("background-color: rgba(0, 0, 0, 0);")

			p.detailLabel.Hide()
		}
	}
	if p.menuTextRequest != p.menuText {
		p.menuText = p.menuTextRequest
		p.menuLabel.SetText(p.menuText)

		// Use first line of `p.detailTextRequest` as `kindLabel`'s text.
		p.kindLabel.SetText(strings.Split(p.detailTextRequest, "\n")[0])

		p.detailText = p.detailTextRequest
		p.detailLabel.SetText(p.detailText)
	}
	p.menuLabel.AdjustSize()
	p.detailLabel.AdjustSize()
	p.menuLabel.AdjustSize()
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
	case "const", "module", "keyword", "package":
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
}

func (p *PopupItem) hide() {
	if p.hidden {
		return
	}
	p.hidden = true
	p.kindLabel.Hide()
	p.kindIcon.Hide()
	p.menuLabel.Hide()
	p.detailLabel.Hide()
}

func (p *PopupItem) show() {
	if !p.hidden {
		return
	}
	p.hidden = false
	p.kindLabel.Show()
	p.kindIcon.Show()
	p.menuLabel.Show()
}

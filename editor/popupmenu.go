package editor

import (
	"errors"
	"fmt"
	"math"
	"sort"
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
	itemLayout      *widgets.QGridLayout
	items           []*PopupItem
	rawItems        []interface{}
	total           int
	showTotal       int
	selected        int
	top             int
	completeMode    string
	scrollBar       *widgets.QWidget
	scrollBarPos    int
	scrollBarHeight int
	scrollCol       *widgets.QWidget
	detailLabel     *widgets.QLabel
	hideItemIdx     [2]bool
}

// PopupItem is
type PopupItem struct {
	p *PopupMenu

	wordLabel   *widgets.QLabel
	word        string
	wordRequest string

	kindwidget *widgets.QWidget
	kindIcon   *svg.QSvgWidget

	menuLabel   *widgets.QLabel
	menu        string
	menuRequest string

	infoLabel   *widgets.QLabel
	info        string
	infoRequest string

	selected        bool
	selectedRequest bool

	hidden bool

	detailText string
}

func initPopupmenuNew() *PopupMenu {
	layout := widgets.NewQHBoxLayout()
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetSpacing(10)

	widget := widgets.NewQWidget(nil, 0)
	widget.SetLayout(layout)
	widget.SetContentsMargins(editor.iconSize/5, 0, 0, editor.iconSize/5)
	widget.SetMaximumSize2(editor.width, editor.height)

	margin := editor.config.Editor.Linespace/2 + 2

	itemLayout := widgets.NewQGridLayout2()
	itemLayout.SetSpacing(0)
	itemLayout.SetContentsMargins(0, editor.iconSize/5, 0, editor.iconSize/5)

	scrollCol := widgets.NewQWidget(nil, 0)
	scrollCol.SetContentsMargins(0, 0, 0, 0)
	scrollCol.SetFixedWidth(5)
	scrollBar := widgets.NewQWidget(scrollCol, 0)
	scrollBar.SetFixedWidth(5)

	detailLabel := widgets.NewQLabel(widget, 0)
	detailLabel.SetContentsMargins(margin, margin, margin, margin)
	detailLabel.SetObjectName("detailText")
	detailLabel.SetWordWrap(true)
	detailLabel.SetAlignment(core.Qt__AlignTop)

	layout.AddLayout(itemLayout, 0)
	layout.AddWidget(detailLabel, 0, core.Qt__AlignTop|core.Qt__AlignLeft)
	layout.AddWidget(scrollCol, 0, 0)

	total := editor.config.Popupmenu.Total

	popup := &PopupMenu{
		widget:      widget,
		itemLayout:  itemLayout,
		detailLabel: detailLabel,
		total:       total,
		scrollBar:   scrollBar,
		scrollCol:   scrollCol,
	}

	var popupItems []*PopupItem
	for i := 0; i < total; i++ {
		iconwidget := widgets.NewQWidget(nil, 0)
		iconlayout := widgets.NewQHBoxLayout()
		iconlayout.SetContentsMargins(0, 0, 0, 0)
		iconwidget.SetLayout(iconlayout)
		kindIcon := svg.NewQSvgWidget(nil)
		kindIcon.SetFixedSize2(editor.iconSize, editor.iconSize)
		iconlayout.AddWidget(kindIcon, 0, 0)
		iconwidget.SetContentsMargins(margin, margin, margin, margin)

		word := widgets.NewQLabel(widget, 0)
		word.SetContentsMargins(margin, margin, margin*2, margin)
		word.SetObjectName("wordlabel")

		menu := widgets.NewQLabel(widget, 0)
		menu.SetContentsMargins(margin, margin, margin, margin)

		info := widgets.NewQLabel(widget, 0)
		info.SetContentsMargins(margin, margin, margin, margin)

		itemLayout.AddWidget2(iconwidget, i, 0, 0)
		itemLayout.AddWidget2(word, i, 1, 0)
		itemLayout.AddWidget2(menu, i, 2, core.Qt__AlignLeft)
		itemLayout.AddWidget2(info, i, 3, core.Qt__AlignLeft)

		popupItem := &PopupItem{
			p:          popup,
			kindIcon:   kindIcon,
			kindwidget: iconwidget,
			wordLabel:  word,
			menuLabel:  menu,
			infoLabel:  info,
		}
		popupItems = append(popupItems, popupItem)
	}
	popup.items = popupItems

	popup.widget.SetGraphicsEffect(util.DropShadow(-2, 6, 40, 200))

	return popup
}

func (p *PopupMenu) updateFont(font *Font) {
	p.detailLabel.SetFont(font.fontNew)
	for i := 0; i < p.total; i++ {
		popupItem := p.items[i]
		popupItem.wordLabel.SetFont(font.fontNew)
		popupItem.menuLabel.SetFont(font.fontNew)
		popupItem.infoLabel.SetFont(font.fontNew)
		popupItem.kindIcon.SetFixedSize2(font.lineHeight, font.lineHeight)
		popupItem.kindwidget.SetFixedWidth(font.lineHeight + editor.config.Editor.Linespace*2)
	}
}

func (p *PopupMenu) setColor() {
	fg := editor.colors.widgetFg.String()
	detail := warpColor(editor.colors.widgetFg, 10).String()
	inactiveFg := editor.colors.inactiveFg.String()
	bg := editor.colors.widgetBg
	transparent := transparent()
	p.scrollBar.SetStyleSheet(fmt.Sprintf("background-color: %s;", inactiveFg))
	p.widget.SetStyleSheet(
		fmt.Sprintf(`
			* { background-color: rgba(%d, %d, %d, %f); color: %s; } 
			.QLabel { background-color: rgba(0, 0, 0, 0.0); color: %s; }
			#wordlabel { color: %s; }
			`,
			bg.R, bg.G, bg.B, transparent, fg, inactiveFg, fg,
		),
	)
	p.detailLabel.SetStyleSheet(
		fmt.Sprintf(
			"* { background-color: rgba(0, 0, 0, 0.0); color: %s; }",
			detail,
		),
	)
}

func (p *PopupMenu) setPumblend(pumblend int) {
	alpha := float64(100-pumblend) / float64(100)
	if alpha < 0 {
		alpha = 0
	}

	fg := editor.colors.widgetFg.String()
	detail := warpColor(editor.colors.widgetFg, 10).String()
	inactiveFg := editor.colors.inactiveFg.String()
	bg := editor.colors.widgetBg
	p.scrollBar.SetStyleSheet(fmt.Sprintf("background-color: %s;", inactiveFg))
	p.widget.SetStyleSheet(
		fmt.Sprintf(`
			* { background-color: rgba(%d, %d, %d, %f); color: %s; } 
			.QLabel { background-color: rgba(0, 0, 0, 0.0); color: %s; }
			#wordlabel { color: %s; }
			`,
			bg.R, bg.G, bg.B, alpha, fg, inactiveFg, fg,
		),
	)
	p.detailLabel.SetStyleSheet(
		fmt.Sprintf(
			"* { background-color: rgba(0, 0, 0, 0.0); color: %s; }",
			detail,
		),
	)
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

	x, y, lineHeight, isCursorBelowTheCenter := p.ws.getPointInWidget(col, row, gridid)

	// Detect vim complete mode
	completeMode, err := detectVimCompleteMode()
	if err == nil {
		p.completeMode = completeMode
	}

	p.detailLabel.SetText("")

	popupItems := p.items
	itemHeight := (lineHeight) + (editor.config.Editor.Linespace + 4)

	// Calc the maximum completion items
	//   where,
	//     `row` is the anchor position, where the first character of the completed word will be
	//     `p.ws.screen.height` is the entire screen height
	heightLeft := 0
	if isCursorBelowTheCenter {
		heightLeft = row * lineHeight
	} else {
		heightLeft = p.ws.screen.height - (row+1)*lineHeight
	}
	total := heightLeft / itemHeight
	if total < p.total {
		p.showTotal = total
	} else {
		p.showTotal = p.total
	}

	startNum := 0
	if selected >= len(items)-p.showTotal && len(items) > p.total {
		startNum = len(items) - p.showTotal
	}

	itemNum := 0
	maxItemLen := 0
	for i := startNum; i < p.total+startNum; i++ {
		j := i - startNum
		popupItem := popupItems[j]
		if i >= len(items) || j >= total {
			popupItem.hide()
			continue
		}

		item := items[i].([]interface{})
		itemLen := p.detectItemLen(item)
		if itemLen > maxItemLen {
			maxItemLen = itemLen
		}

		popupItem.setItem(item, selected == i)
		popupItem.hide()
		popupItem.show()
		itemNum++
	}

	switch maxItemLen {
	case 3:
		p.hideItemIdx = [2]bool{false, true}
	case 4:
		p.hideItemIdx = [2]bool{false, false}
	default:
		p.hideItemIdx = [2]bool{true, true}
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

	p.hide()
	p.show()
	p.setWidgetWidth()
	p.moveWidget(x, y, lineHeight, isCursorBelowTheCenter, itemNum)
}

func (p *PopupMenu) detectItemLen(item []interface{}) int {
	itemlen := 0
	for _, i := range item {
		if i.(string) == "" {
			continue
		}
		itemlen++
	}
	return itemlen
}

func (p *PopupMenu) show() {
	p.widget.Raise()
	p.widget.Show()
}

func (p *PopupMenu) setWidgetWidth() {
	maxWordLabelLen := 0
	isMenuHidden := p.hideItemIdx[0]
	isInfoHidden := p.hideItemIdx[1]
	for _, item := range p.items {
		if item.hidden {
			continue
		}
		if isMenuHidden {
			item.menuLabel.Hide()
		} else {
			item.menuLabel.Show()
		}
		if isInfoHidden {
			item.infoLabel.Hide()
		} else {
			item.infoLabel.Show()
		}

		wordLabelLen := int(math.Ceil(p.ws.font.fontMetrics.HorizontalAdvance(item.wordLabel.Text(), -1)))
		if wordLabelLen > maxWordLabelLen {
			maxWordLabelLen = wordLabelLen
		}
	}
	if isMenuHidden && isInfoHidden {
		p.detailLabel.Hide()
	} else {
		p.detailLabel.Show()
	}

	menuWidth := 0
	if !isMenuHidden {
		menuWidth = editor.config.Popupmenu.MenuWidth
	}

	infoWidth := 0
	if !isInfoHidden {
		infoWidth = editor.config.Popupmenu.InfoWidth
	}

	detailWidth := 0
	if editor.config.Popupmenu.ShowDetail && !isMenuHidden && !isInfoHidden {
		detailWidth = editor.config.Popupmenu.DetailWidth
	}

	margin := editor.config.Editor.Linespace/2 + 2

	p.widget.SetFixedWidth(
		editor.iconSize*2 + maxWordLabelLen + menuWidth + infoWidth + detailWidth + 5 + margin*4 + editor.iconSize/5*4,
	)
}

func (p *PopupMenu) moveWidget(x int, y int, lineHeight int, isCursorBelowTheCenter bool, itemNum int) {
	popupWidth := p.widget.Width()

	// x, y, lineHeight, isCursorBelowTheCenter := p.ws.getPointInWidget(col, row, gridid)
	y += lineHeight

	if x+popupWidth >= p.ws.screen.widget.Width() {
		x = p.ws.screen.widget.Width() - popupWidth - 5
	}

	popupHeight := itemNum*(lineHeight+editor.config.Editor.Linespace+2) + 2 + editor.iconSize*2/5
	p.widget.SetFixedHeight(popupHeight)

	if isCursorBelowTheCenter {
		y = y - (lineHeight + popupHeight)
		p.widget.SetGraphicsEffect(util.DropShadow(-2, -6, 40, 200))
	}

	p.widget.Move2(x, y)
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

	isMenuHidden := p.hideItemIdx[0]
	isInfoHidden := p.hideItemIdx[1]
	for i := 0; i < p.showTotal; i++ {
		popupItem := p.items[i]
		isSelected := selected == i+p.top

		popupItem.setSelected(isSelected)
		if isSelected {
			if editor.config.Popupmenu.ShowDetail {
				if !(isMenuHidden && isInfoHidden) {
					popupItem.p.detailLabel.SetText(popupItem.detailText)
					popupItem.p.detailLabel.Show()
				}
			} else {
				popupItem.p.detailLabel.Hide()
			}
		}
	}
}

func (p *PopupMenu) scroll(n int) {
	p.top += n
	items := p.rawItems
	popupItems := p.items
	maxItemLen := 0
	for i := 0; i < p.showTotal; i++ {
		popupItem := popupItems[i]
		item := items[i+p.top].([]interface{})
		itemLen := p.detectItemLen(item)
		if itemLen > maxItemLen {
			maxItemLen = itemLen
		}
		popupItem.setItem(item, false)
	}

	switch maxItemLen {
	case 3:
		p.hideItemIdx = [2]bool{false, true}
	case 4:
		p.hideItemIdx = [2]bool{false, false}
	default:
		p.hideItemIdx = [2]bool{true, true}
	}

	p.scrollBarPos = int((float64(p.top) / float64(len(items))) * float64(p.widget.Height()))
	p.scrollBar.Move2(0, p.scrollBarPos)
	p.hide()
	p.show()
	p.setWidgetWidth()
}

func (p *PopupItem) updateContent() {
	if p.selected != p.selectedRequest {
		p.selected = p.selectedRequest
		if p.selected {
			p.kindwidget.SetStyleSheet(fmt.Sprintf("background-color: %s;", editor.colors.selectedBg.StringTransparent()))
			p.wordLabel.SetStyleSheet(fmt.Sprintf("background-color: %s;", editor.colors.selectedBg.StringTransparent()))
			p.menuLabel.SetStyleSheet(fmt.Sprintf("background-color: %s;", editor.colors.selectedBg.StringTransparent()))
			p.infoLabel.SetStyleSheet(fmt.Sprintf("background-color: %s;", editor.colors.selectedBg.StringTransparent()))
		} else {
			p.kindwidget.SetStyleSheet("background-color: rgba(0, 0, 0, 0);")
			p.wordLabel.SetStyleSheet("background-color: rgba(0, 0, 0, 0);")
			p.menuLabel.SetStyleSheet("background-color: rgba(0, 0, 0, 0);")
			p.infoLabel.SetStyleSheet("background-color: rgba(0, 0, 0, 0);")
		}
	}
	if p.wordRequest != p.word {
		p.word = p.wordRequest
		p.menu = p.menuRequest
		p.info = p.infoRequest
		p.wordLabel.SetText(p.word)

		menuLines := strings.Split(p.menuRequest, "\n")
		infoLines := strings.Split(p.infoRequest, "\n")
		p.menuLabel.SetText(menuLines[0])
		p.infoLabel.SetText(infoLines[0])

		menuLabelTextLen := math.Ceil(p.p.ws.font.fontMetrics.HorizontalAdvance(menuLines[0], -1))

		if len(menuLines) > 1 || len(infoLines) > 1 || menuLabelTextLen > float64(editor.config.Popupmenu.MenuWidth) {
			p.detailText = p.menuRequest + "\n" + p.infoRequest
		} else {
			p.detailText = ""
		}
	}
	p.wordLabel.AdjustSize()
	// p.menuLabel.AdjustSize()
	// p.infoLabel.AdjustSize()
	p.menuLabel.SetFixedWidth(editor.config.Popupmenu.MenuWidth)
	p.infoLabel.SetFixedWidth(editor.config.Popupmenu.InfoWidth)
}

func (p *PopupItem) setSelected(selected bool) {
	p.selectedRequest = selected
	p.updateContent()
}

func (p *PopupItem) setItem(item []interface{}, selected bool) {
	word := item[0].(string)
	kind := item[1].(string)
	menu := item[2].(string)
	info := item[3].(string)

	p.wordRequest = word
	p.menuRequest = menu
	p.infoRequest = info
	p.setKind(kind, selected)
	p.setSelected(selected)
}

func detectVimCompleteMode() (string, error) {
	ws := editor.workspaces[editor.active]

	var enableCompleteMode int
	err := ws.nvim.Eval("exists('*complete_info')", &enableCompleteMode)
	if err != nil {
		return "vim_unknown", errors.New("complete_info does not exists")
	}

	if enableCompleteMode == 1 {
		var info map[string]interface{}
		ws.nvim.Eval("complete_info()", &info)
		kind, ok := info["mode"]
		if !ok {
			return "", nil
		}
		return kind.(string), nil
	}

	return "", errors.New("does not exits complete_mode()")

}

func (p *PopupItem) setKind(kind string, selected bool) {
	var colorOfFunc, colorOfStatement, colorOfType, colorOfKeyword *RGBA

	hiAttrDef := editor.workspaces[editor.active].screen.hlAttrDef
	var keys []int
	for k := range hiAttrDef {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, k := range keys {
		hi := hiAttrDef[k]
		switch hi.hlName {
		case "Function":
			colorOfFunc = hi.fg()
		case "Statement":
			colorOfStatement = hi.fg()
		case "Type":
			colorOfType = hi.fg()
		case "String":
			colorOfKeyword = hi.fg()
		default:
			continue
		}
	}

	formattedKind := normalizeKind(kind)

	icon := ""
	switch formattedKind {
	case "text":
		icon = editor.getSvg("lsp_"+formattedKind, colorOfKeyword)
	case "method", "function", "constructor":
		icon = editor.getSvg("lsp_function", colorOfFunc)
	case "field", "variable", "property":
		icon = editor.getSvg("lsp_variable", colorOfFunc)
	case "class":
		icon = editor.getSvg("lsp_"+formattedKind, colorOfFunc)
	case "interface":
		icon = editor.getSvg("lsp_"+formattedKind, colorOfFunc)
	case "module":
		icon = editor.getSvg("lsp_"+formattedKind, colorOfKeyword)
	case "unit":
		icon = editor.getSvg("lsp_"+formattedKind, colorOfStatement)
	case "value":
		icon = editor.getSvg("lsp_"+formattedKind, colorOfKeyword)
	case "enum":
		icon = editor.getSvg("lsp_"+formattedKind, colorOfType)
	case "keyword":
		icon = editor.getSvg("lsp_"+formattedKind, colorOfKeyword)
	case "snippet":
		icon = editor.getSvg("lsp_"+formattedKind, colorOfKeyword)
	case "color":
		icon = editor.getSvg("lsp_"+formattedKind, colorOfKeyword)
	case "file":
		icon = editor.getSvg("lsp_"+formattedKind, colorOfType)
	case "reference":
		icon = editor.getSvg("lsp_"+formattedKind, colorOfType)
	case "folder":
		icon = editor.getSvg("lsp_"+formattedKind, colorOfType)
	case "enummember":
		icon = editor.getSvg("lsp_"+formattedKind, colorOfKeyword)
	case "constant":
		icon = editor.getSvg("lsp_"+formattedKind, colorOfKeyword)
	case "struct":
		icon = editor.getSvg("lsp_"+formattedKind, colorOfStatement)
	case "event":
		icon = editor.getSvg("lsp_"+formattedKind, colorOfStatement)
	case "operato":
		icon = editor.getSvg("lsp_"+formattedKind, colorOfStatement)
	case "typeparameter":
		icon = editor.getSvg("lsp_"+formattedKind, colorOfType)
	default:
		iconColor := warpColor(editor.colors.fg, -45)
		switch p.p.completeMode {
		case "keyword",
			"whole_line",
			"files",
			"tags",
			"path_defines",
			"path_patterns",
			"dictionary",
			"thesaurus",
			"cmdline",
			"function",
			"omni",
			"spell",
			"eval":
			icon = editor.getSvg("vim_"+p.p.completeMode, iconColor)
			p.kindIcon.Load2(core.NewQByteArray2(icon, len(icon)))
		default:
			icon = editor.getSvg("vim_unknown", iconColor)
			p.kindIcon.Load2(core.NewQByteArray2(icon, len(icon)))
		}
	}
	p.kindIcon.Load2(core.NewQByteArray2(icon, len(icon)))

}

func normalizeKind(kind string) string {
	// Completion kinds is
	//   Text
	//   Method
	//   Function
	//   Constructor
	//   Field
	//   Variable
	//   Class
	//   Interface
	//   Module
	//   Property
	//   Unit
	//   Value
	//   Enum
	//   Keyword
	//   Snippet
	//   Color
	//   File
	//   Reference
	//   Folder
	//   EnumMember
	//   Constant
	//   Struct
	//   Event
	//   Operator
	//   TypeParameter
	if len(kind) == 1 {
		switch kind {
		case "v":
			return "variable" // equate with "text", "value", "color", "constant"
		case "f":
			return "function" // equate with "method", "constructor"
		case "m":
			return "field" // equate with "property", "enumMember"
		case "C":
			return "class"
		case "I":
			return "interface"
		case "M":
			return "module"
		case "U":
			return "unit"
		case "E":
			return "enum" // equate with "event"??
		case "k":
			return "keyword"
		case "S":
			return "snippet" // equate with "struct"??
		case "F":
			return "file" // equate with "folder"
		case "r":
			return "reference"
		case "O":
			return "operator"
		case "T":
			return "typeparameter"
		}
	}

	lowerKindText := strings.ToLower(kind)
	switch lowerKindText {
	case "func", "instance":
		return "function"
	case "var", "statement", "param", "const":
		return "variable"
	}

	return lowerKindText
}

func (p *PopupItem) hide() {
	if p.hidden {
		return
	}
	p.hidden = true
	p.kindwidget.Hide()
	p.wordLabel.Hide()
	p.menuLabel.Hide()
	p.infoLabel.Hide()
}

func (p *PopupItem) show() {
	if !p.hidden {
		return
	}
	p.hidden = false
	p.kindwidget.Show()
	p.wordLabel.Show()
	p.menuLabel.Show()
	p.infoLabel.Show()
}

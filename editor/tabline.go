package editor

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/akiyosi/goneovim/util"
	"github.com/akiyosi/qt/core"
	"github.com/akiyosi/qt/gui"
	"github.com/akiyosi/qt/svg"
	"github.com/akiyosi/qt/widgets"
	shortpath "github.com/akiyosi/short_path"
	"github.com/neovim/go-client/nvim"
)

// Tabline of the editor
type Tabline struct {
	ws     *Workspace
	widget *widgets.QWidget
	layout *widgets.QLayout
	// font            *gui.QFont
	color           *RGBA
	currentFileText string
	Tabs            []*Tab
	showtabline     int
	marginBottom    int
	height          int
	CurrentID       int
	marginTop       int
}

const (
	TABLINEMARGIN = 5
)

// Tab in the tabline
type Tab struct {
	t         *Tabline
	widget    *widgets.QWidget
	layout    *widgets.QHBoxLayout
	file      *widgets.QLabel
	fileIcon  *svg.QSvgWidget
	closeIcon *svg.QSvgWidget
	fileText  string
	Name      string
	fileType  string
	ID        int
	active    bool
	hidden    bool
}

func (t *Tabline) connectUI() {
	if t == nil {
		return
	}

	if !t.ws.isDrawTabline {
		t.widget.Hide()
		t.height = 0
		t.marginTop = 0
		t.marginBottom = 0
		return
	}
}

func (t *Tabline) setColor() {
	if t == nil {
		return
	}

	if t.color.equals(editor.colors.inactiveFg) {
		return
	}

	t.color = editor.colors.inactiveFg
	colorStr := t.color.String()
	t.widget.SetStyleSheet(fmt.Sprintf(`
	.QWidget { 
		border-bottom: 0px solid;
		border-right: 0px solid;
		background-color: rgba(0, 0, 0, 0); } QWidget { color: %s; } `, colorStr))
	t.updateTabs()
}

func initTabline() *Tabline {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(
		TABLINEMARGIN,
		TABLINEMARGIN,
		TABLINEMARGIN,
		TABLINEMARGIN,
	)

	layout := util.NewVFlowLayout(16, 5, 1, 0, 0)
	// layout := widgets.NewQLayout2()
	// layout.SetSpacing(0)
	// layout.SetContentsMargins(0, 0, 0, 0)

	// // width := 210
	// items := []*widgets.QLayoutItem{}
	// layout.ConnectSizeHint(func() *core.QSize {
	// 	size := core.NewQSize()
	// 	for _, item := range items {
	// 		size = size.ExpandedTo(item.MinimumSize())
	// 	}
	// 	return size
	// })
	// layout.ConnectAddItem(func(item *widgets.QLayoutItem) {
	// 	fmt.Println("connect add widget")
	// 	items = append(items, item)
	// })
	// layout.ConnectSetGeometry(func(r *core.QRect) {
	// 	fmt.Println("connect set geometry")
	// 	for i := 0; i < len(items); i++ {
	// 		items[i].SetGeometry(core.NewQRect4(width*i, 0, width, r.Height()))
	// 	}
	// })
	// layout.ConnectItemAt(func(index int) *widgets.QLayoutItem {
	// 	fmt.Println("connect Item at")
	// 	if index < len(items) {
	// 		return items[index]
	// 	}
	// 	return nil
	// })
	// layout.ConnectTakeAt(func(index int) *widgets.QLayoutItem {
	// 	fmt.Println("connect Take at")
	// 	if index < len(items) {
	// 		return items[index]
	// 	}
	// 	return nil
	// })

	widget.SetLayout(layout)

	space := editor.config.Editor.Linespace
	tabline := &Tabline{
		widget:       widget,
		layout:       layout,
		marginTop:    int(math.Ceil(float64(space) / 3.0)),
		marginBottom: int(math.Ceil(float64(space) * 2.0 / 3.0)),
		showtabline:  -1, // The initial value is -1, and the correct value is retrieved by event notification from nvim.
	}

	tabs := []*Tab{}
	for i := 0; i < 24; i++ {
		tab := newTab()
		tab.t = tabline
		tabs = append(tabs, tab)
	}
	tabline.Tabs = tabs

	return tabline
}

func (t *Tabline) showEvent(event *gui.QShowEvent) {
	t.ws.updateSize()
}

func (t *Tabline) initTab() {
	for i, tab := range t.Tabs {
		t.layout.AddWidget(tab.widget)
		if i > 0 {
			tab.hide()
		}
	}
}

func newTab() *Tab {
	w := widgets.NewQWidget(nil, 0)
	// space := int(float64(editor.config.Editor.Linespace) * 0.3)
	w.SetContentsMargins(5, 0, 5, 0)
	l := widgets.NewQHBoxLayout()
	l.SetContentsMargins(0, 0, 0, 0) // tab margins
	l.SetSpacing(0)
	var fileIcon *svg.QSvgWidget
	if editor.config.Tabline.ShowIcon {
		fileIcon = svg.NewQSvgWidget(nil)
		fileIcon.SetFixedWidth(editor.iconSize * 5 / 6)
		fileIcon.SetFixedHeight(editor.iconSize * 5 / 6)
	}
	file := widgets.NewQLabel(nil, 0)
	file.SetContentsMargins(0, 0, editor.iconSize/4, 0)
	file.SetStyleSheet(" .QLabel { padding: 1px; background-color: rgba(0, 0, 0, 0); }")
	l.SetAlignment(file, core.Qt__AlignTop)
	closeIcon := svg.NewQSvgWidget(nil)
	closeIcon.SetFixedWidth(editor.iconSize)
	closeIcon.SetFixedHeight(editor.iconSize)
	if editor.config.Tabline.ShowIcon {
		l.AddWidget(fileIcon, 0, 0)
	}
	l.AddWidget(file, 1, 0)
	l.AddWidget(closeIcon, 0, 0)
	w.SetLayout(l)
	tab := &Tab{
		widget:    w,
		layout:    l,
		file:      file,
		closeIcon: closeIcon,
	}
	if editor.config.Tabline.ShowIcon {
		tab.fileIcon = fileIcon
	}
	tab.closeIcon.Hide()

	tab.widget.ConnectEnterEvent(tab.enterEvent)
	tab.widget.ConnectLeaveEvent(tab.leaveEvent)
	tab.widget.ConnectMousePressEvent(tab.pressEvent)

	closeIcon.ConnectMousePressEvent(tab.closeIconPressEvent)
	closeIcon.ConnectMouseReleaseEvent(tab.closeIconReleaseEvent)
	closeIcon.ConnectEnterEvent(tab.closeIconEnterEvent)
	closeIcon.ConnectLeaveEvent(tab.closeIconLeaveEvent)

	return tab
}

func (t *Tab) updateStyle() {
	if t == nil {
		return
	}

	if editor.colors.fg == nil || editor.colors.bg == nil {
		return
	}
	if t.t.ws.screen.hlAttrDef == nil {
		return
	}
	fg := editor.colors.fg
	inactiveFg := editor.colors.inactiveFg
	accent := t.t.ws.screen.hlAttrDef[t.t.ws.screen.highlightGroup["TabLineFill"]].foreground.Hex()

	if t.active {
		activeStyle := fmt.Sprintf(`
		.QWidget { 
			border-bottom: 1.0px solid %s; 
			background-color: rgba(0, 0, 0, 0); 
		} QWidget{ color: %s; } `, accent, warpColor(fg, -30))
		t.widget.SetStyleSheet(activeStyle)
	} else {
		inActiveStyle := fmt.Sprintf(`
		.QWidget { 
			border: 0px solid %s; 
			background-color: rgba(0, 0, 0, 0); 
		} QWidget{ color: %s; } `, accent, warpColor(inactiveFg, -30))
		t.widget.SetStyleSheet(inActiveStyle)
	}
	svgContent := editor.getSvg("cross", fg)
	t.closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
}

// func (t *Tabline) updateFont() {
// 	if t == nil {
// 		return
// 	}
//
// 	t.font = t.ws.font.qfont
//
// 	// t.widget.SetFont(t.font)
// 	for _, tab := range t.Tabs {
// 		tab.file.SetFont(t.font)
// 	}
//
// 	t.updateSize()
// }

func (t *Tab) show() {
	if t == nil {
		return
	}
	if !t.hidden {
		return
	}

	t.hidden = false
	t.widget.Show()
	t.updateStyle()
}

func (t *Tab) hide() {
	if t == nil {
		return
	}
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
	t.updateStyle()
}

func (t *Tab) updateFileText() {
	path, _ := shortpath.PrettyMinimum(t.fileText)
	t.file.SetText(path)
	t.updateSize()
}

func (t *Tab) updateSize() {
	if t == nil {
		return
	}

	// if t.t.font == nil {
	// 	return
	// }

	// fontmetrics := gui.NewQFontMetricsF(t.t.font)
	// width := int(fontmetrics.HorizontalAdvance(
	// 	t.file.Text(),
	// 	-1,
	// ))
	width := t.t.ws.font.horizontalAdvance(t.file.Text())

	if editor.config.Tabline.ShowIcon {
		width += float64(editor.iconSize)
	}
	height := int(float64(t.t.ws.font.height)*1.3) + t.t.marginTop + t.t.marginBottom + 1
	t.widget.SetFixedSize2(int(width)+editor.iconSize+5+10+5, height)
}

func (t *Tab) updateFileIcon() {
	if t == nil {
		return
	}

	iconColor := editor.colors.inactiveFg
	if t.active {
		iconColor = nil
	}
	svgContent := editor.getSvg(t.fileType, iconColor)
	t.fileIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
}

func (t *Tabline) updateMargin() {
	if t == nil {
		return
	}

	for _, tab := range t.Tabs {
		tab.file.SetContentsMargins(0, t.marginTop, 0, t.marginBottom)
		if !tab.hidden {
			tab.hide()
			tab.show()
		}
	}
}

func (t *Tabline) updateSize() {
	if t == nil {
		return
	}

	for _, tab := range t.Tabs {
		tab.updateSize()
	}
}

func (t *Tabline) updateTabs() {
	if t == nil {
		return
	}

	for _, tab := range t.Tabs {
		svgContent := editor.getSvg(tab.fileType, nil)
		tab.fileIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		if !tab.hidden {
			tab.updateStyle()
		}
	}
}

func (t *Tabline) handle(args []interface{}) {
	if t == nil {
		return
	}

	arg := args[0].([]interface{})
	t.CurrentID = int(arg[0].(nvim.Tabpage))
	tabs := arg[1].([]interface{})
	if len(tabs) == 1 {
		t.Tabs[0].setActive(false)
		t.Tabs[0].updateStyle()
	}
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
		tab.setActive(tab.ID == t.CurrentID)

		fileType := getFileType(text)
		tab.fileType = fileType
		if editor.config.Tabline.ShowIcon {
			tab.updateFileIcon()
		}

		if text != tab.fileText {
			tab.fileText = text
			tab.updateFileText()
		}

		if tab.ID == t.CurrentID {
			t.currentFileText = text
		}
		tab.show()
	}

	// Set color
	t.setColor()
	t.update(tabs)
}

func (t *Tabline) update(tabs []interface{}) {
	if t == nil {
		return
	}

	lenhiddentabs := 0
	for i := len(tabs); i < len(t.Tabs); i++ {
		tab := t.Tabs[i]
		tab.setActive(false)
		tab.hide()
		lenhiddentabs++
	}

	lenshowntabs := 24 - lenhiddentabs
	// Support showtabline behavior in external tabline
	height := t.Tabs[0].widget.Height() + (TABLINEMARGIN * 2)
	isChangeHeight := t.height != height

	doUpdate := t.showtabline != t.ws.showtabline || t.ws.showtabline == 1 || isChangeHeight
	if doUpdate {
		t.ws.cursor.update()
		t.showtabline = t.ws.showtabline
	}

	if t.ws.showtabline == 1 {
		if lenshowntabs > 1 {
			t.widget.Show()
			t.ws.isDrawTabline = true
		} else {
			t.widget.Hide()
			t.ws.isDrawTabline = false
			t.height = 0
		}
	} else if t.ws.showtabline == 2 {
		t.widget.Show()
		t.height = height
	} else {
		t.widget.Hide()
		t.ws.isDrawTabline = false
		t.height = 0
	}

	if doUpdate {
		t.ws.updateSize()
	}
}

func getFileType(text string) string {
	if strings.HasPrefix(text, "term://") {
		return "terminal"
	}
	base := filepath.Base(text)
	if strings.Contains(base, ".") {
		parts := strings.Split(base, ".")
		filetype := parts[len(parts)-1]
		if filetype == "md" {
			filetype = "markdown"
		}
		if filetype == "rs" {
			filetype = "rust"
		}
		if filetype == "yml" {
			filetype = "yaml"
		}
		if filetype == "rb" {
			filetype = "ruby"
		}
		if filetype == "hs" {
			filetype = "haskell"
		}
		if filetype == "pl" {
			filetype = "perl"
		}
		if filetype == "jl" {
			filetype = "julia"
		}
		return filetype
	}
	return "default"
}

func (t *Tab) enterEvent(event *core.QEvent) {
	t.closeIcon.Show()
}

func (t *Tab) leaveEvent(event *core.QEvent) {
	t.closeIcon.Hide()
}

func (t *Tab) pressEvent(event *gui.QMouseEvent) {
	targetTab := nvim.Tabpage(t.ID)
	go t.t.ws.nvim.SetCurrentTabpage(targetTab)
}

func (t *Tab) closeIconPressEvent(event *gui.QMouseEvent) {
	t.closeIcon.SetFixedWidth(editor.iconSize)
	t.closeIcon.SetFixedHeight(editor.iconSize)
	svgContent := editor.getSvg("hoverclose", nil)
	t.closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
}

func (t *Tab) closeIconReleaseEvent(event *gui.QMouseEvent) {
	if t.ID == 1 {
		go t.t.ws.nvim.Command(fmt.Sprintf("q"))
	} else {
		targetTab := nvim.Tabpage(t.ID)
		go func() {
			t.t.ws.nvim.SetCurrentTabpage(targetTab)
			t.t.ws.nvim.Command("tabclose!")
		}()
	}
}

func (t *Tab) closeIconEnterEvent(event *core.QEvent) {
	t.closeIcon.SetFixedWidth(editor.iconSize)
	t.closeIcon.SetFixedHeight(editor.iconSize)
	svgContent := editor.getSvg("hoverclose", editor.colors.fg)
	t.closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))

	cursor := gui.NewQCursor()
	cursor.SetShape(core.Qt__PointingHandCursor)
	t.widget.SetCursor(cursor)
}

func (t *Tab) closeIconLeaveEvent(event *core.QEvent) {
	t.closeIcon.SetFixedWidth(editor.iconSize)
	t.closeIcon.SetFixedHeight(editor.iconSize)
	svgContent := editor.getSvg("cross", editor.colors.fg)
	t.closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))

	cursor := gui.NewQCursor()
	cursor.SetShape(core.Qt__ArrowCursor)
	t.widget.SetCursor(cursor)
}

package editor

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/akiyosi/goneovim/util"
	shortpath "github.com/akiyosi/short_path"
	"github.com/neovim/go-client/nvim"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

// Tabline of the editor
type Tabline struct {
	ws              *Workspace
	widget          *widgets.QWidget
	layout          *widgets.QLayout
	CurrentID       int
	currentFileText string
	Tabs            []*Tab
	showtabline     int
	marginDefault   int
	marginTop       int
	marginBottom    int
	height          int

	font       *gui.QFont
	fontfamily string
	fontsize   int
}

// Tab in the tabline
type Tab struct {
	t         *Tabline
	widget    *widgets.QWidget
	layout    *widgets.QHBoxLayout
	ID        int
	active    bool
	Name      string
	fileIcon  *svg.QSvgWidget
	fileType  string
	closeIcon *svg.QSvgWidget
	file      *widgets.QLabel
	fileText  string
	hidden    bool
}

func (t *Tabline) subscribe() {
	if !t.ws.drawTabline {
		t.widget.Hide()
		t.height = 0
		t.marginDefault = 0
		t.marginTop = 0
		t.marginBottom = 0
		return
	}
}

func (t *Tabline) setColor() {
	inactiveFg := editor.colors.inactiveFg.String()
	t.widget.SetStyleSheet(fmt.Sprintf(`
	.QWidget { 
		border-bottom: 0px solid;
		border-right: 0px solid;
		background-color: rgba(0, 0, 0, 0); } QWidget { color: %s; } `, inactiveFg))
	t.updateTabsIcon()
}

func initTabline() *Tabline {
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(5, 5, 5, 5)

	layout := util.NewVFlowLayout(16, 10, 1, 0, 0)
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

	marginDefault := 10
	marginTop := int(float64(editor.extFontSize) / 2.2) // No effect now
	marginBot := int(float64(editor.extFontSize) / 1.8) // No effect now
	tabline := &Tabline{
		widget:        widget,
		layout:        layout,
		marginDefault: marginDefault,
		marginTop:     marginTop,
		marginBottom:  marginBot,
		showtabline:   2,
	}

	tabs := []*Tab{}
	for i := 0; i < 24; i++ {
		tab := newTab()
		tab.t = tabline
		tabs = append(tabs, tab)
	}
	go func() {
		for i, tab := range tabs {
			layout.AddWidget(tab.widget)
			if i > 0 {
				tab.hide()
			}
		}
	}()
	tabline.Tabs = tabs

	return tabline
}

func newTab() *Tab {
	w := widgets.NewQWidget(nil, 0)
	w.SetContentsMargins(5, 0, 5, 0)
	l := widgets.NewQHBoxLayout()
	l.SetContentsMargins(0, 0, 0, 0) // tab margins
	l.SetSpacing(0)
	var fileIcon *svg.QSvgWidget
	if editor.config.Tabline.ShowIcon {
		fileIcon = svg.NewQSvgWidget(nil)
		fileIcon.SetFixedWidth(editor.iconSize)
		fileIcon.SetFixedHeight(editor.iconSize)
	}
	file := widgets.NewQLabel(nil, 0)
	file.SetContentsMargins(0, 0, editor.iconSize/4, 0)
	file.SetStyleSheet(" .QLabel { padding: 2px; background-color: rgba(0, 0, 0, 0); }")
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
			border-bottom: 2.0px solid %s; 
			background-color: rgba(0, 0, 0, 0); 
		} QWidget{ color: %s; } `, accent, warpColor(fg, -30))
		t.widget.SetStyleSheet(activeStyle)
		svgContent := editor.getSvg("cross", nil)
		t.closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	} else {
		inActiveStyle := fmt.Sprintf(`
		.QWidget { 
			border: 0px solid %s; 
			background-color: rgba(0, 0, 0, 0); 
		} QWidget{ color: %s; } `, accent, warpColor(inactiveFg, -30))
		t.widget.SetStyleSheet(inActiveStyle)
		svgContent := editor.getSvg("cross", inactiveFg)
		t.closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	}
}

func (t *Tabline) updateFont() {
	size := editor.extFontSize - 1
	if size <= 0 {
		size = editor.extFontSize
	}
	if t.fontfamily == editor.extFontFamily && t.fontsize == size {
		return
	}
	t.font = gui.NewQFont2(editor.extFontFamily, size, 1, false)

	// t.widget.SetFont(t.font)
	for _, tab := range t.Tabs {
		tab.file.SetFont(t.font)
	}
}

func (t *Tab) show() {
	if !t.hidden {
		return
	}
	t.hidden = false
	t.widget.Show()
	t.updateStyle()
}

func (t *Tab) hide() {
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
	if t.t.font == nil {
		return
	}

	fontmetrics := gui.NewQFontMetricsF(t.t.font)
	width := int(fontmetrics.HorizontalAdvance(
		t.file.Text(),
		-1,
	))
	if editor.config.Tabline.ShowIcon {
		width += +editor.iconSize
	}
	height := int(fontmetrics.Height()) + t.t.marginTop + t.t.marginBottom
	t.widget.SetFixedSize2(width+editor.iconSize+5+10+5, height)
}

func (t *Tab) updateFileIcon() {
	svgContent := editor.getSvg(t.fileType, nil)
	t.fileIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
}

func (t *Tabline) updateMargin() {
	for _, tab := range t.Tabs {
		tab.file.SetContentsMargins(0, t.marginTop, 0, t.marginBottom)
		if !tab.hidden {
			tab.hide()
			tab.show()
		}
	}
}

func (t *Tabline) updateSize() {
	for _, tab := range t.Tabs {
		tab.updateSize()
	}
}

func (t *Tabline) updateTabsIcon() {
	for _, tab := range t.Tabs {
		svgContent := editor.getSvg(tab.fileType, nil)
		tab.fileIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	}
}

func (t *Tabline) update(args []interface{}) {
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

		fileType := getFileType(text)
		if fileType != tab.fileType {
			tab.fileType = fileType
			if editor.config.Tabline.ShowIcon {
				tab.updateFileIcon()
			}
		}

		if text != tab.fileText {
			tab.fileText = text
			tab.updateFileText()
		}

		tab.setActive(tab.ID == t.CurrentID)
		if tab.ID == t.CurrentID {
			t.currentFileText = text
		}
		tab.show()
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
	if t.ws.showtabline == 1 {
		if lenshowntabs > 1 {
			t.widget.Show()
			t.ws.drawTabline = true
		} else {
			t.widget.Hide()
			t.ws.drawTabline = false
			t.height = 0
		}
	} else if t.ws.showtabline == 2 {
		t.widget.Show()
		t.height = t.widget.Height()
	} else {
		t.widget.Hide()
		t.ws.drawTabline = false
		t.height = 0
	}
	if t.showtabline != t.ws.showtabline || t.ws.showtabline == 1 {
		t.ws.updateSize()
		t.showtabline = t.ws.showtabline
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
	svgContent := editor.getSvg("hoverclose", nil)
	t.closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))

	cursor := gui.NewQCursor()
	cursor.SetShape(core.Qt__PointingHandCursor)
	t.widget.SetCursor(cursor)
}

func (t *Tab) closeIconLeaveEvent(event *core.QEvent) {
	t.closeIcon.SetFixedWidth(editor.iconSize)
	t.closeIcon.SetFixedHeight(editor.iconSize)
	svgContent := editor.getSvg("cross", nil)
	t.closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))

	cursor := gui.NewQCursor()
	cursor.SetShape(core.Qt__ArrowCursor)
	t.widget.SetCursor(cursor)
}

package editor

import (
	"fmt"
	"path/filepath"
	"strings"

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
	marginDefault   int
	marginTop       int
	marginBottom    int
	height          int
}

// Tab in the tabline
type Tab struct {
	t         *Tabline
	widget    *widgets.QWidget
	layout    *widgets.QHBoxLayout
	ID        int
	active    bool
	Name      string
	width     int
	chars     int
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
		t.ws.updateSize()
		return
	}
}

func (t *Tabline) setColor() {
	inactiveFg := editor.colors.inactiveFg.String()
	// bg := editor.colors.bg.StringTransparent()
	t.widget.SetStyleSheet(fmt.Sprintf(`
	.QWidget { 
		border-bottom: 0px solid;
		border-right: 0px solid;
		background-color: rgba(0, 0, 0, 0); } QWidget { color: %s; } `, inactiveFg))
}

func newTabline() *Tabline {
	width := 210
	widget := widgets.NewQWidget(nil, 0)
	layout := widgets.NewQLayout2()
	layout.SetSpacing(0)
	layout.SetContentsMargins(0, 0, 0, 0)
	items := []*widgets.QLayoutItem{}
	layout.ConnectSizeHint(func() *core.QSize {
		size := core.NewQSize()
		for _, item := range items {
			size = size.ExpandedTo(item.MinimumSize())
		}
		return size
	})
	layout.ConnectAddItem(func(item *widgets.QLayoutItem) {
		items = append(items, item)
	})
	layout.ConnectSetGeometry(func(r *core.QRect) {
		for i := 0; i < len(items); i++ {
			items[i].SetGeometry(core.NewQRect4(width*i, 0, width, r.Height()))
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
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetLayout(layout)

	marginDefault := 10
	marginTop := editor.config.Editor.FontSize / 3    // No effect now
	marginBottom := editor.config.Editor.FontSize / 3 // No effect now
	tabline := &Tabline{
		widget:        widget,
		layout:        layout,
		marginDefault: marginDefault,
		marginTop:     marginTop,
		marginBottom:  marginBottom,
	}

	tabs := []*Tab{}
	for i := 0; i < 10; i++ {
		w := widgets.NewQWidget(nil, 0)
		w.SetContentsMargins(10, 0, 10, 0)
		l := widgets.NewQHBoxLayout()
		l.SetContentsMargins(8, 2, 0, 2) // tab margins
		l.SetSpacing(10)
		// fileIcon := svg.NewQSvgWidget(nil)
		// fileIcon.SetFixedWidth(12)
		// fileIcon.SetFixedHeight(12)
		file := widgets.NewQLabel(nil, 0)
		file.SetContentsMargins(0, marginTop, 0, marginBottom)
		closeIcon := svg.NewQSvgWidget(nil)
		closeIcon.SetFixedWidth(editor.iconSize)
		closeIcon.SetFixedHeight(editor.iconSize)
		// l.AddWidget(fileIcon, 0, 0)
		l.AddWidget(file, 1, 0)
		l.AddWidget(closeIcon, 0, 0)
		w.SetLayout(l)
		tab := &Tab{
			t:      tabline,
			widget: w,
			layout: l,
			file:   file,
			// fileIcon:  fileIcon,
			closeIcon: closeIcon,
		}
		tab.closeIcon.Hide()

		tab.widget.ConnectEnterEvent(tab.enterEvent)
		tab.widget.ConnectLeaveEvent(tab.leaveEvent)
		tab.widget.ConnectMousePressEvent(tab.pressEvent)

		closeIcon.ConnectMousePressEvent(tab.closeIconPressEvent)
		closeIcon.ConnectMouseReleaseEvent(tab.closeIconReleaseEvent)
		closeIcon.ConnectEnterEvent(tab.closeIconEnterEvent)
		closeIcon.ConnectLeaveEvent(tab.closeIconLeaveEvent)

		tabs = append(tabs, tab)
		layout.AddWidget(w)
		if i > 0 {
			tab.hide()
		}
	}
	tabline.Tabs = tabs

	return tabline
}

func (t *Tab) updateActive() {
	if editor.colors.fg == nil || editor.colors.bg == nil {
		return
	}
	bg := editor.colors.bg
	fg := editor.colors.fg.String()
	inactiveFg := editor.colors.inactiveFg
	if t.active {
		activeStyle := fmt.Sprintf(`
		.QWidget { 
			border-left: 2px solid %s; 
			background-color: rgba(%d, %d, %d, %d); 
		} QWidget{ color: %s; } `, editor.config.SideBar.AccentColor, bg.R, bg.G, bg.B, 0, fg)
		t.widget.SetStyleSheet(activeStyle)
		svgContent := editor.getSvg("cross", nil)
		t.closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	} else {
		inActiveStyle := fmt.Sprintf(`
		.QWidget { 
			border-left: 0px solid %s; 
			background-color: rgba(%d, %d, %d, %d); 
		} QWidget{ color: %s; } `, editor.config.SideBar.AccentColor, bg.R, bg.G, bg.B, 0, inactiveFg)
		t.widget.SetStyleSheet(inActiveStyle)
		svgContent := editor.getSvg("cross", inactiveFg)
		t.closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	}
}

func (t *Tab) show() {
	if !t.hidden {
		return
	}
	t.hidden = false
	t.widget.Show()
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
	t.updateActive()
}

func (t *Tab) updateFileText() {
	text := t.t.ws.font.defaultFontMetrics.ElidedText(t.fileText, core.Qt__ElideLeft, float64(t.file.Width()), 0)
	t.file.SetText(text)
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

func (t *Tabline) update(args []interface{}) {
	arg := args[0].([]interface{})
	t.CurrentID = int(arg[0].(nvim.Tabpage))
	tabs := arg[1].([]interface{})
	if len(tabs) == 1 {
		t.Tabs[0].setActive(false)
		t.Tabs[0].updateActive()
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
			// tab.updateFileIcon()
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
	for i := len(tabs); i < len(t.Tabs); i++ {
		tab := t.Tabs[i]
		tab.setActive(false)
		tab.hide()
	}
}

func getFileType(text string) string {
	if strings.HasPrefix(text, "term://") {
		return "terminal"
	}
	base := filepath.Base(text)
	if strings.Index(base, ".") >= 0 {
		parts := strings.Split(base, ".")
		filetype := parts[len(parts)-1]
		if filetype == "md" {
			filetype = "markdown"
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
	t.t.ws.nvim.SetCurrentTabpage(targetTab)
}

func (t *Tab) closeIconPressEvent(event *gui.QMouseEvent) {
	t.closeIcon.SetFixedWidth(editor.iconSize)
	t.closeIcon.SetFixedHeight(editor.iconSize)
	svgContent := editor.getSvg("hoverclose", nil)
	t.closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
}

func (t *Tab) closeIconReleaseEvent(event *gui.QMouseEvent) {
	if t.ID == 1 {
		t.t.ws.nvim.Command(fmt.Sprintf("q"))
	} else {
		targetTab := nvim.Tabpage(t.ID)
		t.t.ws.nvim.SetCurrentTabpage(targetTab)
		t.t.ws.nvim.Command("tabclose!")
	}
}

func (t *Tab) closeIconEnterEvent(event *core.QEvent) {
	t.closeIcon.SetFixedWidth(editor.iconSize)
	t.closeIcon.SetFixedHeight(editor.iconSize)
	svgContent := editor.getSvg("hoverclose", nil)
	t.closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	cursor := gui.NewQCursor()
	cursor.SetShape(core.Qt__PointingHandCursor)
	gui.QGuiApplication_SetOverrideCursor(cursor)
}

func (t *Tab) closeIconLeaveEvent(event *core.QEvent) {
	t.closeIcon.SetFixedWidth(editor.iconSize)
	t.closeIcon.SetFixedHeight(editor.iconSize)
	svgContent := editor.getSvg("cross", nil)
	t.closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	gui.QGuiApplication_RestoreOverrideCursor()
}

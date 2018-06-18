package editor

import (
	"fmt"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

type Navigation struct {
	widget   *widgets.QWidget
	layout   *widgets.QVBoxLayout
	editItem *NavigationItem
	deinItem *NavigationItem
}

type NavigationItem struct {
	widget *widgets.QWidget
	text   string
	icon   *svg.QSvgWidget
	active bool
}

func newNavigation() *Navigation {
	naviLayout := widgets.NewQVBoxLayout()
	naviLayout.SetContentsMargins(1, 10, 0, 0)
	naviLayout.SetSpacing(1)

	naviSubLayout := widgets.NewQVBoxLayout()
	naviSubLayout.SetSpacing(1)
	naviSubLayout.SetContentsMargins(0, 0, 0, 0)
	naviSubLayout.SetSpacing(15)

	fg := editor.fgcolor

	editLayout := widgets.NewQVBoxLayout()
	editLayout.SetContentsMargins(12, 5, 12, 5)
	editLayout.SetSpacing(1)
	editIcon := svg.NewQSvgWidget(nil)
	editIcon.SetFixedWidth(22)
	editIcon.SetFixedHeight(22)
	svgContent := editor.workspaces[editor.active].getSvg("naviedit", newRGBA(warpColor(fg, 5).R, warpColor(fg, 5).G, warpColor(fg, 5).B, 1))
	//svgContent := editor.workspaces[editor.active].getSvg("naviedit", nil)
	editIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	editLayout.AddWidget(editIcon, 0, 0)
	editWidget := widgets.NewQWidget(nil, 0)
	editWidget.SetLayout(editLayout)
	editItem := &NavigationItem{
		widget: editWidget,
		text:   "naviedit",
		icon:   editIcon,
		active: true,
	}

	deinLayout := widgets.NewQVBoxLayout()
	deinLayout.SetContentsMargins(12, 5, 12, 5)
	deinLayout.SetSpacing(1)
	deinIcon := svg.NewQSvgWidget(nil)
	deinIcon.SetFixedWidth(22)
	deinIcon.SetFixedHeight(22)
	svgDeinContent := editor.workspaces[editor.active].getSvg("navidein", newRGBA(gradColor(fg).R, gradColor(fg).G, gradColor(fg).B, 1))
	//svgDeinContent := editor.workspaces[editor.active].getSvg("navidein", nil)
	deinIcon.Load2(core.NewQByteArray2(svgDeinContent, len(svgDeinContent)))
	deinLayout.AddWidget(deinIcon, 0, 0)
	deinWidget := widgets.NewQWidget(nil, 0)
	deinWidget.SetLayout(deinLayout)
	deinItem := &NavigationItem{
		widget: deinWidget,
		text:   "navidein",
		icon:   deinIcon,
	}

	naviSubLayout.AddWidget(editWidget, 0, 0)
	naviSubLayout.AddWidget(deinWidget, 0, 0)
	naviSubWidget := widgets.NewQWidget(nil, 0)
	naviSubWidget.SetLayout(naviSubLayout)

	naviLayout.AddWidget(naviSubWidget, 0, 0)
	naviLayout.SetAlignment(naviSubWidget, core.Qt__AlignTop)

	navigation := &Navigation{
		layout:   naviLayout,
		editItem: editItem,
		deinItem: deinItem,
	}

	navigation.editItem.widget.ConnectEnterEvent(editItem.enterEvent)
	navigation.editItem.widget.ConnectLeaveEvent(editItem.leaveEvent)
	navigation.editItem.widget.ConnectMousePressEvent(editItem.mouseEvent)
	navigation.deinItem.widget.ConnectEnterEvent(deinItem.enterEvent)
	navigation.deinItem.widget.ConnectLeaveEvent(deinItem.leaveEvent)
	navigation.deinItem.widget.ConnectMousePressEvent(deinItem.mouseEvent)

	return navigation
}

func (n *NavigationItem) enterEvent(event *core.QEvent) {
	fg := editor.fgcolor
	n.widget.SetStyleSheet(fmt.Sprintf(" * { color: rgba(%d, %d, %d, 1); } ", warpColor(fg, 10).R, warpColor(fg, 10).G, warpColor(fg, 10).B))
	svgContent := editor.workspaces[editor.active].getSvg(n.text, newRGBA(warpColor(fg, 10).R, warpColor(fg, 10).G, warpColor(fg, 10).B, 1))
	n.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
}

func (n *NavigationItem) leaveEvent(event *core.QEvent) {
	fg := editor.fgcolor
	var svgContent string
	if n.active == true {
		n.widget.SetStyleSheet(fmt.Sprintf(" * { color: rgba(%d, %d, %d, 1); } ", warpColor(fg, 10).R, warpColor(fg, 10).G, warpColor(fg, 10).B))
		svgContent = editor.workspaces[editor.active].getSvg(n.text, newRGBA(warpColor(fg, 10).R, warpColor(fg, 10).G, warpColor(fg, 10).B, 1))
	} else {
		n.widget.SetStyleSheet(fmt.Sprintf(" * { color: rgba(%d, %d, %d, 1); } ", gradColor(fg).R, gradColor(fg).G, gradColor(fg).B))
		svgContent = editor.workspaces[editor.active].getSvg(n.text, newRGBA(gradColor(fg).R, gradColor(fg).G, gradColor(fg).B, 1))
	}
	n.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
}

func (n *NavigationItem) mouseEvent(event *gui.QMouseEvent) {
	items := []*NavigationItem{editor.navigation.editItem, editor.navigation.deinItem}
	for _, item := range items {
		item.active = false
	}
	n.active = true
	setNavigationItemColor()
}

func setNavigationItemColor() {
	fg := editor.fgcolor
	var svgContent string
	items := []*NavigationItem{editor.navigation.editItem, editor.navigation.deinItem}
	for _, item := range items {
		if item.active == true {
			item.widget.SetStyleSheet(fmt.Sprintf(" * { color: rgba(%d, %d, %d, 1); } ", warpColor(fg, 10).R, warpColor(fg, 10).G, warpColor(fg, 10).B))
			svgContent = editor.workspaces[editor.active].getSvg(item.text, newRGBA(warpColor(fg, 10).R, warpColor(fg, 10).G, warpColor(fg, 10).B, 1))
		} else {
			item.widget.SetStyleSheet(fmt.Sprintf(" * { color: rgba(%d, %d, %d, 1); } ", gradColor(fg).R, gradColor(fg).G, gradColor(fg).B))
			svgContent = editor.workspaces[editor.active].getSvg(item.text, newRGBA(gradColor(fg).R, gradColor(fg).G, gradColor(fg).B, 1))
		}
		item.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	}
}

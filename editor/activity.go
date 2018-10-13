package editor

import (
	"fmt"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

// Activity is the Activity bar
type Activity struct {
	widget   *widgets.QWidget
	layout   *widgets.QVBoxLayout
	editItem *ActivityItem
	deinItem *ActivityItem
	//sideArea *widgets.QScrollArea
	sideArea *widgets.QStackedWidget
}

// ActivityItem is the item in Activity bar
type ActivityItem struct {
	widget *widgets.QWidget
	text   string
	icon   *svg.QSvgWidget
	active bool
	id     int
}

func newActivity() *Activity {
	activityLayout := widgets.NewQVBoxLayout()
	activityLayout.SetContentsMargins(1, 10, 0, 0)
	activityLayout.SetSpacing(1)

	activitySubLayout := widgets.NewQVBoxLayout()
	activitySubLayout.SetSpacing(1)
	activitySubLayout.SetContentsMargins(0, 0, 0, 0)
	activitySubLayout.SetSpacing(15)

	// fg := editor.fgcolor

	editLayout := widgets.NewQVBoxLayout()
	editLayout.SetContentsMargins(12, 5, 12, 5)
	editLayout.SetSpacing(1)
	editIcon := svg.NewQSvgWidget(nil)
	editIcon.SetFixedWidth(22)
	editIcon.SetFixedHeight(22)
	// svgContent := editor.workspaces[editor.active].getSvg("activityedit", newRGBA(warpColor(fg, 5).R, warpColor(fg, 5).G, warpColor(fg, 5).B, 1))
	// editIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	editLayout.AddWidget(editIcon, 0, 0)
	editWidget := widgets.NewQWidget(nil, 0)
	editWidget.SetLayout(editLayout)
	editItem := &ActivityItem{
		widget: editWidget,
		text:   "activityedit",
		icon:   editIcon,
		active: editor.config.SideBar.Visible,
		id:     1,
	}

	// bg := editor.bgcolor

	deinLayout := widgets.NewQVBoxLayout()
	deinLayout.SetContentsMargins(12, 5, 12, 5)
	deinLayout.SetSpacing(1)
	deinIcon := svg.NewQSvgWidget(nil)
	deinIcon.SetFixedWidth(22)
	deinIcon.SetFixedHeight(22)
	// svgDeinContent := editor.workspaces[editor.active].getSvg("activitydein", newRGBA(gradColor(bg).R, gradColor(bg).G, gradColor(bg).B, 1))
	// deinIcon.Load2(core.NewQByteArray2(svgDeinContent, len(svgDeinContent)))
	deinLayout.AddWidget(deinIcon, 0, 0)
	deinWidget := widgets.NewQWidget(nil, 0)
	deinWidget.SetLayout(deinLayout)
	deinItem := &ActivityItem{
		widget: deinWidget,
		text:   "activitydein",
		icon:   deinIcon,
		id:     2,
	}

	activitySubLayout.AddWidget(editWidget, 0, 0)
	activitySubLayout.AddWidget(deinWidget, 0, 0)
	activitySubWidget := widgets.NewQWidget(nil, 0)
	activitySubWidget.SetLayout(activitySubLayout)

	activityLayout.AddWidget(activitySubWidget, 0, 0)
	activityLayout.SetAlignment(activitySubWidget, core.Qt__AlignTop)

	stackedWidget := widgets.NewQStackedWidget(nil)

	activity := &Activity{
		layout:   activityLayout,
		editItem: editItem,
		deinItem: deinItem,
		sideArea: stackedWidget,
	}

	activity.editItem.widget.ConnectEnterEvent(editItem.enterEvent)
	activity.editItem.widget.ConnectLeaveEvent(editItem.leaveEvent)
	activity.editItem.widget.ConnectMousePressEvent(editItem.mouseEvent)
	activity.deinItem.widget.ConnectEnterEvent(deinItem.enterEvent)
	activity.deinItem.widget.ConnectLeaveEvent(deinItem.leaveEvent)
	activity.deinItem.widget.ConnectMousePressEvent(deinItem.mouseEvent)

	return activity
}

func (n *ActivityItem) enterEvent(event *core.QEvent) {
	cursor := gui.NewQCursor()
	cursor.SetShape(core.Qt__PointingHandCursor)
	gui.QGuiApplication_SetOverrideCursor(cursor)
	fg := editor.fgcolor
	n.widget.SetStyleSheet(fmt.Sprintf(" * { color: rgba(%d, %d, %d, 1); } ", warpColor(fg, 15).R, warpColor(fg, 15).G, warpColor(fg, 15).B))
	svgContent := editor.workspaces[editor.active].getSvg(n.text, newRGBA(warpColor(fg, 15).R, warpColor(fg, 15).G, warpColor(fg, 15).B, 1))
	n.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
}

func (n *ActivityItem) leaveEvent(event *core.QEvent) {
	fg := editor.fgcolor
	bg := editor.bgcolor
	var svgContent string
	if n.active == true {
		n.widget.SetStyleSheet(fmt.Sprintf(" * { color: rgba(%d, %d, %d, 1); } ", warpColor(fg, 15).R, warpColor(fg, 15).G, warpColor(fg, 15).B))
		svgContent = editor.workspaces[editor.active].getSvg(n.text, newRGBA(warpColor(fg, 15).R, warpColor(fg, 15).G, warpColor(fg, 15).B, 1))
	} else {
		n.widget.SetStyleSheet(fmt.Sprintf(" * { color: rgba(%d, %d, %d, 1); } ", gradColor(bg).R, gradColor(bg).G, gradColor(bg).B))
		svgContent = editor.workspaces[editor.active].getSvg(n.text, newRGBA(gradColor(bg).R, gradColor(bg).G, gradColor(bg).B, 1))
	}
	n.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	gui.QGuiApplication_RestoreOverrideCursor()
}

func (n *ActivityItem) mouseEvent(event *gui.QMouseEvent) {
	if n.active == true {
		editor.activity.sideArea.Hide()
		n.active = false
		return
	}
	editor.activity.sideArea.Show()

	items := []*ActivityItem{editor.activity.editItem, editor.activity.deinItem}
	for _, item := range items {
		item.active = false
	}
	n.active = true

	setActivityItemColor()

	switch n.text {
	case "activitydein":
		if editor.deinSide == nil {

			editor.deinSide = newDeinSide()

			sideArea := widgets.NewQScrollArea(nil)
			sideArea.SetWidgetResizable(true)
			sideArea.SetVerticalScrollBarPolicy(core.Qt__ScrollBarAsNeeded)
			sideArea.SetFocusPolicy(core.Qt__ClickFocus)
			sideArea.SetWidget(editor.deinSide.widget)
			sideArea.SetFrameShape(widgets.QFrame__NoFrame)
			editor.deinSide.scrollarea = sideArea
			editor.deinSide.scrollarea.ConnectResizeEvent(deinSideResize)

			bg := editor.bgcolor
			editor.deinSide.scrollarea.SetStyleSheet(fmt.Sprintf(".QScrollBar { border-width: 0px; background-color: rgb(%d, %d, %d); width: 5px; margin: 0 0 0 0; } .QScrollBar::handle:vertical {background-color: rgb(%d, %d, %d); min-height: 25px;} .QScrollBar::add-line:vertical, .QScrollBar::sub-line:vertical { border: none; background: none; } .QScrollBar::add-page:vertical, QScrollBar::sub-page:vertical { background: none; }", shiftColor(bg, -5).R, shiftColor(bg, -5).G, shiftColor(bg, -5).B, gradColor(bg).R, gradColor(bg).G, gradColor(bg).B))

			editor.activity.sideArea.AddWidget(editor.deinSide.scrollarea)
		}
		editor.activity.sideArea.SetCurrentWidget(editor.deinSide.scrollarea)

	case "activityedit":
		editor.activity.sideArea.SetCurrentWidget(editor.wsSide.scrollarea)
		editor.workspaces[editor.active].setCwd()
		for n, item := range editor.wsSide.items {
			item.setSideItemLabel(n)
		}
	}

}

func deinSideResize(event *gui.QResizeEvent) {
	width := editor.splitter.Widget(editor.splitter.IndexOf(editor.activity.sideArea)).Width()
	if width <= 0 {
		return
	}

	editor.deinSide.header.SetMinimumWidth(width)
	editor.deinSide.header.SetMaximumWidth(width)
	editor.deinSide.searchresult.widget.SetMinimumWidth(width)
	editor.deinSide.searchresult.widget.SetMaximumWidth(width)

	editor.deinSide.combobox.widget.SetMinimumWidth(width)
	editor.deinSide.combobox.widget.SetMaximumWidth(width)

	editor.deinSide.searchbox.widget.SetMinimumWidth(width)
	editor.deinSide.searchbox.widget.SetMaximumWidth(width)
	editor.deinSide.searchbox.editBox.SetFixedWidth(width - (20 + 20))

	editor.deinSide.installedplugins.widget.SetMinimumWidth(width)
	editor.deinSide.installedplugins.widget.SetMaximumWidth(width)

	for _, item := range editor.deinSide.installedplugins.items {
		item.nameLabel.SetFixedWidth(width)
		item.widget.SetMaximumWidth(width)
		item.widget.SetMinimumWidth(width)
	}

	for _, item := range editor.deinSide.searchresult.plugins {
		item.widget.SetMaximumWidth(width)
		item.widget.SetMinimumWidth(width)
		item.nameLabel.SetFixedWidth(width)
		item.head.SetMaximumWidth(width)
		item.head.SetMaximumWidth(width)
		item.desc.SetMinimumWidth(width - 20)
		item.desc.SetMinimumWidth(width - 20)
	}
}

func setActivityItemColor() {
	fg := editor.fgcolor
	bg := editor.bgcolor
	var svgContent string
	items := []*ActivityItem{editor.activity.editItem, editor.activity.deinItem}
	for _, item := range items {
		if item.active == true {
			item.widget.SetStyleSheet(fmt.Sprintf(" * { color: rgba(%d, %d, %d, 1); } ", warpColor(fg, 15).R, warpColor(fg, 15).G, warpColor(fg, 15).B))
			svgContent = editor.workspaces[editor.active].getSvg(item.text, newRGBA(warpColor(fg, 15).R, warpColor(fg, 15).G, warpColor(fg, 15).B, 1))
		} else {
			item.widget.SetStyleSheet(fmt.Sprintf(" * { color: rgba(%d, %d, %d, 1); } ", gradColor(bg).R, gradColor(bg).G, gradColor(bg).B))
			svgContent = editor.workspaces[editor.active].getSvg(item.text, newRGBA(gradColor(bg).R, gradColor(bg).G, gradColor(bg).B, 1))
		}
		item.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	}
}

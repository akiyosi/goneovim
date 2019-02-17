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

	// fg := editor.colors.fg

	editLayout := widgets.NewQVBoxLayout()
	editLayout.SetContentsMargins(12, 5, 12, 5)
	editLayout.SetSpacing(1)
	editIcon := svg.NewQSvgWidget(nil)
	editIcon.SetFixedWidth((editor.iconSize - 2) * 2)
	editIcon.SetFixedHeight((editor.iconSize - 2) * 2)
	svgContent := editor.getSvg("activityedit", nil)
	editIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
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

	// bg := editor.colors.bg

	deinLayout := widgets.NewQVBoxLayout()
	deinLayout.SetContentsMargins(12, 5, 12, 5)
	deinLayout.SetSpacing(1)
	deinIcon := svg.NewQSvgWidget(nil)
	deinIcon.SetFixedWidth((editor.iconSize - 2) * 2)
	deinIcon.SetFixedHeight((editor.iconSize - 2) * 2)
	svgDeinContent := editor.getSvg("activitydein", newRGBA(255, 255, 255, 1))
	deinIcon.Load2(core.NewQByteArray2(svgDeinContent, len(svgDeinContent)))
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
	activitySubWidget.SetStyleSheet(" * { background-color: rgba(0, 0, 0, 0); } ")

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
	fg := editor.colors.fg
	n.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: rgba(0, 0, 0, 0); color: %s; } ", fg.String()))
	svgContent := editor.getSvg(n.text, nil)
	n.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
}

func (n *ActivityItem) leaveEvent(event *core.QEvent) {
	fg := editor.colors.fg
	inactiveFg := editor.colors.inactiveFg
	var svgContent string
	if n.active == true {
		n.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: rgba(0, 0, 0, 0); color: %s; } ", fg.String()))
		svgContent = editor.getSvg(n.text, nil)
	} else {
		n.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: rgba(0, 0, 0, 0); color: %s; } ", inactiveFg))
		svgContent = editor.getSvg(n.text, inactiveFg)
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
			sideArea.ConnectEnterEvent(func(event *core.QEvent) {
				sideArea.SetVerticalScrollBarPolicy(core.Qt__ScrollBarAsNeeded)
			})
			sideArea.ConnectLeaveEvent(func(event *core.QEvent) {
				sideArea.SetVerticalScrollBarPolicy(core.Qt__ScrollBarAlwaysOff)
			})
			editor.deinSide.scrollarea = sideArea
			editor.deinSide.scrollarea.ConnectResizeEvent(deinSideResize)

			bg := editor.colors.sideBarBg.StringTransparent()
			sbg := editor.colors.sideBarSelectedItemBg.StringTransparent()
			editor.deinSide.scrollarea.SetStyleSheet(fmt.Sprintf(`
			.QScrollBar {
				border-width: 0px;
				background-color: %s;
				width: 5px;
				margin: 0 0 0 0; 
			} 
			.QScrollBar::handle:vertical {
				background-color: %s;
				min-height: 25px;
			} 
			.QScrollBar::handle:vertical:hover {
				background-color: %s; 
				min-height: 25px;
			} 
			.QScrollBar::add-line:vertical, .QScrollBar::sub-line:vertical {
				border: none; 
				background: none; 
			} 
			.QScrollBar::add-page:vertical, QScrollBar::sub-page:vertical {
				background: none; 
			}`, bg, sbg, editor.config.SideBar.AccentColor))

			editor.activity.sideArea.AddWidget(editor.deinSide.scrollarea)
		}
		editor.activity.sideArea.SetCurrentWidget(editor.deinSide.scrollarea)

	case "activityedit":
		editor.activity.sideArea.SetCurrentWidget(editor.wsSide.scrollarea)
		editor.workspaces[editor.active].nvim.Command(`call rpcnotify(0, "Gui", "gonvim_workspace_cwd", getcwd())`)
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
	fg := editor.colors.fg
	inactiveFg := editor.colors.inactiveFg
	var svgContent string
	items := []*ActivityItem{editor.activity.editItem, editor.activity.deinItem}
	for _, item := range items {
		if item.active == true {
			item.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: rgba(0, 0, 0, 0); color: %s; } ", fg.String()))
			svgContent = editor.getSvg(item.text, fg)
		} else {
			item.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: rgba(0, 0, 0, 0); color: %s; } ", inactiveFg.String()))
			svgContent = editor.getSvg(item.text, inactiveFg)
		}
		item.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	}
}

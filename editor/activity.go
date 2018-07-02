package editor

import (
	"fmt"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

type Activity struct {
	widget   *widgets.QWidget
	layout   *widgets.QVBoxLayout
	editItem *ActivityItem
	deinItem *ActivityItem
	//sideArea *widgets.QScrollArea
	sideArea *widgets.QStackedWidget
}

type ActivityItem struct {
	widget *widgets.QWidget
	text   string
	icon   *svg.QSvgWidget
	active bool
	id     int
}

func newActivity() *Activity {
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
	editItem := &ActivityItem{
		widget: editWidget,
		text:   "naviedit",
		icon:   editIcon,
		active: true,
		id:     1,
	}

	bg := editor.bgcolor

	deinLayout := widgets.NewQVBoxLayout()
	deinLayout.SetContentsMargins(12, 5, 12, 5)
	deinLayout.SetSpacing(1)
	deinIcon := svg.NewQSvgWidget(nil)
	deinIcon.SetFixedWidth(22)
	deinIcon.SetFixedHeight(22)
	svgDeinContent := editor.workspaces[editor.active].getSvg("navidein", newRGBA(gradColor(bg).R, gradColor(bg).G, gradColor(bg).B, 1))
	//svgDeinContent := editor.workspaces[editor.active].getSvg("navidein", nil)
	deinIcon.Load2(core.NewQByteArray2(svgDeinContent, len(svgDeinContent)))
	deinLayout.AddWidget(deinIcon, 0, 0)
	deinWidget := widgets.NewQWidget(nil, 0)
	deinWidget.SetLayout(deinLayout)
	deinItem := &ActivityItem{
		widget: deinWidget,
		text:   "navidein",
		icon:   deinIcon,
		id:     2,
	}

	naviSubLayout.AddWidget(editWidget, 0, 0)
	naviSubLayout.AddWidget(deinWidget, 0, 0)
	naviSubWidget := widgets.NewQWidget(nil, 0)
	naviSubWidget.SetLayout(naviSubLayout)

	naviLayout.AddWidget(naviSubWidget, 0, 0)
	naviLayout.SetAlignment(naviSubWidget, core.Qt__AlignTop)

	stackedWidget := widgets.NewQStackedWidget(nil)

	activity := &Activity{
		layout:   naviLayout,
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
}

func (n *ActivityItem) mouseEvent(event *gui.QMouseEvent) {
	if n.active == true {
		editor.activity.sideArea.Hide()
		n.active = false
		return
	} else {
		editor.activity.sideArea.Show()
	}

	items := []*ActivityItem{editor.activity.editItem, editor.activity.deinItem}
	for _, item := range items {
		item.active = false
	}
	n.active = true

	setActivityItemColor()

	switch n.text {
	case "navidein":
		if editor.deinSide == nil {

			editor.deinSide = newDeinSide()

			sideArea := widgets.NewQScrollArea(nil)
			sideArea.SetWidgetResizable(true)
			sideArea.SetVerticalScrollBarPolicy(core.Qt__ScrollBarAsNeeded)
			sideArea.SetFocusPolicy(core.Qt__ClickFocus)
			sideArea.SetWidget(editor.deinSide.widget)
			sideArea.SetFrameShape(widgets.QFrame__NoFrame)
			sideArea.SetMaximumWidth(editor.config.sideWidth)
			sideArea.SetMinimumWidth(editor.config.sideWidth)
			editor.deinSide.scrollarea = sideArea

			bg := editor.bgcolor
			editor.deinSide.scrollarea.SetStyleSheet(fmt.Sprintf(".QScrollBar { border-width: 0px; background-color: rgb(%d, %d, %d); width: 5px; margin: 0 0 0 0; } .QScrollBar::handle:vertical {background-color: rgb(%d, %d, %d); min-height: 25px;} .QScrollBar::add-line:vertical, .QScrollBar::sub-line:vertical { border: none; background: none; } .QScrollBar::add-page:vertical, QScrollBar::sub-page:vertical { background: none; }", shiftColor(bg, -5).R, shiftColor(bg, -5).G, shiftColor(bg, -5).B, gradColor(bg).R, gradColor(bg).G, gradColor(bg).B))

			editor.activity.sideArea.AddWidget(editor.deinSide.scrollarea)
		}
		editor.activity.sideArea.SetCurrentWidget(editor.deinSide.scrollarea)

	case "naviedit":
		editor.activity.sideArea.SetCurrentWidget(editor.wsSide.scrollarea)
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

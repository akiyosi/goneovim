package editor

import (
	"fmt"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

type Notification struct {
	widget    *widgets.QWidget
	closeIcon *svg.QSvgWidget
	pos       *core.QPoint
	isDrag    bool
}

func newNotification(message string) *Notification {
	ws := editor.workspaces[editor.active]
	widget := widgets.NewQWidget(nil, 0)
	layout := widgets.NewQHBoxLayout()
	layout.SetContentsMargins(10, 10, 10, 15)
	widget.SetLayout(layout)
	widget.SetFixedWidth(400)

	infoIcon := svg.NewQSvgWidget(nil)
	infoIcon.SetFixedWidth(14)
	infoIcon.SetFixedHeight(14)
	infoIcon.SetContentsMargins(0, 0, 0, 0)
	info := ws.getSvg("info", newRGBA(27, 161, 226, 1))
	infoIcon.Load2(core.NewQByteArray2(info, len(info)))

	label := widgets.NewQLabel(nil, 0)
	label.SetWordWrap(true)
	label.SetText(message)

	closeIcon := svg.NewQSvgWidget(nil)
	closeIcon.SetFixedWidth(14)
	closeIcon.SetFixedHeight(14)
	closeIcon.Hide()

	closeIcon.ConnectMousePressEvent(func(event *gui.QMouseEvent) {
		fg := editor.fgcolor
		svgContent := ws.getSvg("hoverclose", newRGBA(gradColor(fg).R, gradColor(fg).G, gradColor(fg).B, 1))
		closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	})
	closeIcon.ConnectEnterEvent(func(event *core.QEvent) {
		svgContent := ws.getSvg("hoverclose", nil)
		closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		cursor := gui.NewQCursor()
		cursor.SetShape(core.Qt__PointingHandCursor)
		gui.QGuiApplication_SetOverrideCursor(cursor)
	})
	closeIcon.ConnectLeaveEvent(func(event *core.QEvent) {
		svgContent := ws.getSvg("cross", nil)
		closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		gui.QGuiApplication_RestoreOverrideCursor()
	})

	layout.AddWidget(infoIcon, 0, 0)
	layout.AddWidget(label, 0, 0)
	layout.AddWidget(closeIcon, 0, 0)

	layout.SetAlignment(infoIcon, core.Qt__AlignTop)
	layout.SetAlignment(closeIcon, core.Qt__AlignTop)

	isDrag := false
	startPos := editor.notifyStartPos

	notification := &Notification{
		widget:    widget,
		closeIcon: closeIcon,
		pos:       startPos,
		isDrag:    isDrag,
	}
	notification.widget.Hide()

	notification.closeIcon.ConnectMouseReleaseEvent(func(event *gui.QMouseEvent) {
		notification.widget.Hide()
		editor.notifyStartPos = core.NewQPoint2(editor.width-400-10, editor.height-30)
	})
	notification.widget.ConnectEnterEvent(func(event *core.QEvent) {
		fg := editor.fgcolor
		svgContent := ws.getSvg("cross", newRGBA(fg.R, fg.G, fg.B, 1))
		notification.closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		notification.closeIcon.Show()
	})
	notification.widget.ConnectLeaveEvent(func(event *core.QEvent) {
		notification.closeIcon.Hide()
	})
	notification.widget.ConnectMousePressEvent(func(event *gui.QMouseEvent) {
		notification.isDrag = true
		notification.pos = event.Pos()
	})
	notification.widget.ConnectMouseReleaseEvent(func(*gui.QMouseEvent) {
		notification.isDrag = false
	})
	notification.widget.ConnectMouseMoveEvent(func(event *gui.QMouseEvent) {
		if notification.isDrag == true {
			x := event.Pos().X() - notification.pos.X()
			y := event.Pos().Y() - notification.pos.Y()
			newPos := core.NewQPoint2(x, y)
			trans := notification.widget.MapToParent(newPos)
			notification.widget.Move(trans)
		}
	})

	return notification
}

func (n *Notification) show() {
	fg := editor.fgcolor
	bg := editor.bgcolor
	n.widget.SetStyleSheet(fmt.Sprintf(" * {color: rgb(%d, %d, %d); background: rgb(%d, %d, %d);}", fg.R, fg.G, fg.B, shiftColor(bg, -8).R, shiftColor(bg, -8).G, shiftColor(bg, -8).B))
	n.widget.Show()
}

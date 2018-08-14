package editor

import (
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/widgets"
)

type Notification struct {
	widget *widgets.QWidget
	pos    *core.QPoint
	isDrag bool
}

func newNotification() *Notification {

	widget := widgets.NewQWidget(nil, 0)
	layout := widgets.NewQHBoxLayout()
	widget.SetLayout(layout)
	widget.SetFixedWidth(200)
	widget.SetFixedHeight(100)
	label := widgets.NewQLabel(nil, 0)
	label.SetText("It is Alerm!")
	layout.AddWidget(label, 0, 0)
	label.SetStyleSheet(" * { background: #111; }")

	isDrag := false
	startPos := core.NewQPoint2(0, 0)

	notification := &Notification{
		widget: widget,
		pos:    startPos,
		isDrag: isDrag,
	}

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

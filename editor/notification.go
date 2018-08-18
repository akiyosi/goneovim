package editor

import (
	"fmt"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

type NotifyLevel int

const (
	NotifyInfo NotifyLevel = 0
	NotifyWarn NotifyLevel = 1
)

type Notification struct {
	widget    *widgets.QWidget
	closeIcon *svg.QSvgWidget
	pos       *core.QPoint
	isDrag    bool
	isDragged bool
	hide      bool
}

type NotifyOptions struct {
	buttons []*NotifyButton
}

type NotifyOptionArg func(*NotifyOptions)

func notifyOptionArg(b []*NotifyButton) NotifyOptionArg {
	return func(option *NotifyOptions) {
		option.buttons = b
	}
}

func newNotification(l NotifyLevel, message string, options ...NotifyOptionArg) *Notification {
	ws := editor.workspaces[editor.active]

	widget := widgets.NewQWidget(nil, 0)
	layout := widgets.NewQVBoxLayout()
	layout.SetContentsMargins(10, 10, 10, 15)
	layout.SetSpacing(8)
	widget.SetLayout(layout)
	widget.SetFixedWidth(400)

	messageWidget := widgets.NewQWidget(nil, 0)
	messageLayout := widgets.NewQHBoxLayout()
	messageLayout.SetContentsMargins(0, 0, 0, 0)
	messageWidget.SetLayout(messageLayout)
	// messageWidget.SetFixedWidth(400)

	levelIcon := svg.NewQSvgWidget(nil)
	levelIcon.SetFixedWidth(14)
	levelIcon.SetFixedHeight(14)
	levelIcon.SetContentsMargins(0, 0, 0, 0)
	var level string
	switch l {
	case NotifyInfo:
		level = ws.getSvg("info", newRGBA(27, 161, 226, 1))
	case NotifyWarn:
		level = ws.getSvg("warn", newRGBA(255, 205, 0, 1))
	default:
		level = ws.getSvg("info", newRGBA(27, 161, 226, 1))
	}
	levelIcon.Load2(core.NewQByteArray2(level, len(level)))

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

	messageLayout.AddWidget(levelIcon, 0, 0)
	messageLayout.AddWidget(label, 0, 0)
	messageLayout.AddWidget(closeIcon, 0, 0)
	messageLayout.SetAlignment(levelIcon, core.Qt__AlignTop)
	messageLayout.SetAlignment(closeIcon, core.Qt__AlignTop)

	layout.AddWidget(messageWidget, 0, 0)
	layout.SetAlignment(messageWidget, core.Qt__AlignTop)

	bottomwidget := widgets.NewQWidget(nil, 0)
	bottomlayout := widgets.NewQHBoxLayout()
	bottomlayout.SetContentsMargins(0, 0, 0, 0)
	bottomlayout.SetSpacing(10)
	bottomwidget.SetLayout(bottomlayout)

	opts := NotifyOptions{}
	for _, o := range options {
		o(&opts)
	}
	for _, opt := range opts.buttons {
		if opt.text != "" {
			// * plugin install button
			buttonLabel := widgets.NewQLabel(nil, 0)
			buttonLabel.SetFixedHeight(28)
			buttonLabel.SetContentsMargins(10, 5, 10, 5)
			buttonLabel.SetAlignment(core.Qt__AlignCenter)
			button := widgets.NewQWidget(nil, 0)
			buttonLayout := widgets.NewQHBoxLayout()
			buttonLayout.SetContentsMargins(0, 0, 0, 0)
			buttonLayout.AddWidget(buttonLabel, 0, 0)
			button.SetLayout(buttonLayout)
			button.SetObjectName("button")
			buttonLabel.SetText(opt.text)
			color := "#0e639c"
			button.SetStyleSheet(fmt.Sprintf(" #button QLabel { color: #ffffff; background: %s;} ", color))
			fn := opt.action
			button.ConnectMousePressEvent(func(*gui.QMouseEvent) {
				go fn()
			})
			button.ConnectEnterEvent(func(event *core.QEvent) {
				hoverColor := "#1177bb"
				button.SetStyleSheet(fmt.Sprintf(" #button QLabel { color: #ffffff; background: %s;} ", hoverColor))
			})
			button.ConnectLeaveEvent(func(event *core.QEvent) {
				button.SetStyleSheet(fmt.Sprintf(" #button QLabel { color: #ffffff; background: %s;} ", color))
			})
			bottomlayout.AddWidget(button, 0, 0)
			bottomlayout.SetAlignment(button, core.Qt__AlignRight)
		}
	}
	bottomwidget.AdjustSize()
	layout.AddWidget(bottomwidget, 0, 0)
	layout.SetAlignment(bottomwidget, core.Qt__AlignRight)

	isDrag := false
	isDragged := false
	startPos := editor.notifyStartPos

	notification := &Notification{
		widget:    widget,
		closeIcon: closeIcon,
		pos:       startPos,
		isDrag:    isDrag,
		isDragged: isDragged,
	}
	notification.widget.Hide()

	notification.closeIcon.ConnectMouseReleaseEvent(notification.closeNotification)
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
		notification.widget.Raise()
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
		notification.isDragged = true
	})

	// Drop shadow to widget
	go func() {
		shadow := widgets.NewQGraphicsDropShadowEffect(nil)
		shadow.SetBlurRadius(40)
		shadow.SetColor(gui.NewQColor3(0, 0, 0, 35))
		shadow.SetOffset3(2, 2)
		widget.SetGraphicsEffect(shadow)
	}()

	timer := core.NewQTimer(nil)
	timer.SetSingleShot(true)
	timer.ConnectTimeout(notification.hideNotification)
	timer.Start(6000)

	return notification
}

func (n *Notification) closeNotification(event *gui.QMouseEvent) {
	var newNotifications []*Notification
	var del int
	dropHeight := 0
	for i, item := range editor.notifications {
		if n == item {
			del = i
			dropHeight = item.widget.Height() + 4
			item.widget.DestroyQWidget()
			continue
		}
		if i > del && !item.isDragged {
			x := item.widget.Pos().X()
			y := item.widget.Pos().Y() + dropHeight
			item.widget.Move2(x, y)
			item.widget.Hide()
			if !item.hide {
				item.widget.Show()
			}
		}
		newNotifications = append(newNotifications, item)
	}
	editor.notifications = newNotifications
	editor.notifyStartPos = core.NewQPoint2(editor.notifyStartPos.X(), editor.notifyStartPos.Y()+dropHeight)
	editor.pushNotification(NotifyInfo, "") // dummy push
}

func (n *Notification) hideNotification() {
	var newNotifications []*Notification
	var hide int
	dropHeight := 0
	for i, item := range editor.notifications {
		newNotifications = append(newNotifications, item)
		if n == item {
			hide = i
			dropHeight = item.widget.Height() + 4
			item.widget.Hide()
			item.hide = true
			continue
		}
		if i > hide && !item.isDragged {
			x := item.widget.Pos().X()
			y := item.widget.Pos().Y() + dropHeight
			item.widget.Move2(x, y)
			item.widget.Hide()
			if !item.hide {
				item.widget.Show()
			}
		}
	}
	editor.notifications = newNotifications
	editor.notifyStartPos = core.NewQPoint2(editor.notifyStartPos.X(), editor.notifyStartPos.Y()+dropHeight)

	editor.displayNotifications = false
	for _, item := range editor.notifications {
		if item.hide == false {
			editor.displayNotifications = true
		}
	}
}

func (e *Editor) showNotifications() {
	e.notifyStartPos = core.NewQPoint2(e.width-400-10, e.height-30)
	x := e.notifyStartPos.X()
	y := e.notifyStartPos.Y()
	var newNotifications []*Notification
	for _, item := range e.notifications {
		x = e.notifyStartPos.X()
		y = e.notifyStartPos.Y() - item.widget.Height() - 4
		item.widget.Move2(x, y)
		item.hide = false
		item.widget.Show()
		e.notifyStartPos = core.NewQPoint2(x, y)
		newNotifications = append(newNotifications, item)
	}
	e.notifications = newNotifications
	e.displayNotifications = true
}

func (e *Editor) hideNotifications() {
	var newNotifications []*Notification
	for _, item := range e.notifications {
		item.hide = true
		item.widget.Hide()
		newNotifications = append(newNotifications, item)
	}
	e.notifications = newNotifications
	e.notifyStartPos = core.NewQPoint2(e.width-400-10, e.height-30)
	e.displayNotifications = false
}

func (n *Notification) show() {
	fg := editor.fgcolor
	bg := editor.bgcolor
	n.widget.SetStyleSheet(fmt.Sprintf(" * {color: rgb(%d, %d, %d); background: rgb(%d, %d, %d);}", fg.R, fg.G, fg.B, shiftColor(bg, -8).R, shiftColor(bg, -8).G, shiftColor(bg, -8).B))
	n.widget.Show()
}

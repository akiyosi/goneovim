package editor

import (
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/akiyosi/goneovim/util"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

// NotifyLevel is notification type like "warn", "error"
type NotifyLevel int

const (
	// NotifyInfo is a type of "information"
	NotifyInfo NotifyLevel = 0
	// NotifyWarn is a type of "warning"
	NotifyWarn NotifyLevel = 1
)

// Notification is
type Notification struct {
	widget    *widgets.QWidget
	closeIcon *svg.QSvgWidget
	pos       *core.QPoint
	isDrag    bool
	isMoved   bool
	isHide    bool
}

// NotifyOptions is
type NotifyOptions struct {
	buttons []*NotifyButton
}

// NotifyOptionArg is
type NotifyOptionArg func(*NotifyOptions)

func notifyOptionArg(b []*NotifyButton) NotifyOptionArg {
	return func(option *NotifyOptions) {
		option.buttons = b
	}
}

func newNotification(l NotifyLevel, p int, message string, options ...NotifyOptionArg) *Notification {
	e := editor

	widget := widgets.NewQWidget(nil, 0)
	layout := widgets.NewQVBoxLayout()
	widget.SetLayout(layout)
	widget.SetFixedWidth(e.notificationWidth)

	messageWidget := widgets.NewQWidget(nil, 0)
	messageLayout := widgets.NewQHBoxLayout()
	messageLayout.SetContentsMargins(0, 0, 0, 0)
	messageWidget.SetLayout(messageLayout)
	messageWidget.SetStyleSheet(" * {background-color: rgba(0, 0, 0, 0)}")

	levelIcon := svg.NewQSvgWidget(nil)
	levelIcon.SetFixedWidth(editor.iconSize)
	levelIcon.SetFixedHeight(editor.iconSize)
	levelIcon.SetContentsMargins(0, 0, 0, 0)
	levelIcon.SetStyleSheet(" * {background-color: rgba(0, 0, 0, 0)}")
	var level string
	switch l {
	case NotifyInfo:
		level = e.getSvg("info", newRGBA(27, 161, 226, 1))
	case NotifyWarn:
		level = e.getSvg("warn", newRGBA(255, 205, 0, 1))
	default:
		level = e.getSvg("info", newRGBA(27, 161, 226, 1))
	}
	levelIcon.Load2(core.NewQByteArray2(level, len(level)))

	label := widgets.NewQLabel(nil, 0)
	label.SetStyleSheet(" * {background-color: rgba(0, 0, 0, 0)}")
	size := int(float64(editor.workspaces[editor.active].font.width) * 1.33)
	label.SetFont(gui.NewQFont2(editor.extFontFamily, size, 1, false))
	if utf8.RuneCountInString(message) > 50 {
		label.SetWordWrap(true)
	}
	label.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Expanding)
	label.SetText(message)

	closeIcon := svg.NewQSvgWidget(nil)
	svgContent := e.getSvg("cross", editor.colors.widgetBg)
	closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	closeIcon.SetFixedWidth(editor.iconSize - 1)
	closeIcon.SetFixedHeight(editor.iconSize - 1)

	closeIcon.ConnectMousePressEvent(func(event *gui.QMouseEvent) {
		inactiveFg := editor.colors.inactiveFg
		svgContent := e.getSvg("hoverclose", inactiveFg)
		closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	})
	closeIcon.ConnectEnterEvent(func(event *core.QEvent) {
		svgContent := e.getSvg("hoverclose", nil)
		closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		cursor := gui.NewQCursor()
		cursor.SetShape(core.Qt__PointingHandCursor)
		widget.SetCursor(cursor)
	})
	closeIcon.ConnectLeaveEvent(func(event *core.QEvent) {
		svgContent := e.getSvg("cross", nil)
		closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		cursor := gui.NewQCursor()
		cursor.SetShape(core.Qt__ArrowCursor)
		widget.SetCursor(cursor)
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

	notification := &Notification{}
	opts := NotifyOptions{}
	for _, o := range options {
		o(&opts)
	}
	for _, opt := range opts.buttons {
		if opt.text != "" {
			// * plugin install button
			buttonLabel := widgets.NewQLabel(nil, 0)
			buttonLabel.SetFont(gui.NewQFont2(editor.extFontFamily, editor.extFontSize-1, 1, false))
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
				notification.closeNotification()
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
	if len(opts.buttons) > 0 {
		layout.SetSpacing(8)
		bottomlayout.SetContentsMargins(0, 0, 0, 0)
		bottomlayout.SetSpacing(10)
		bottomwidget.AdjustSize()
		layout.AddWidget(bottomwidget, 0, 0)
		layout.SetAlignment(bottomwidget, core.Qt__AlignRight)
	}
	layout.SetContentsMargins(10, 10, 10, 10)

	isDrag := false
	isMoved := false
	startPos := editor.notifyStartPos

	notification.widget = widget
	notification.closeIcon = closeIcon
	notification.pos = startPos
	notification.isDrag = isDrag
	notification.isMoved = isMoved
	notification.widget.Hide()

	notification.closeIcon.ConnectMouseReleaseEvent(func(event *gui.QMouseEvent) {
		notification.closeNotification()
	})
	notification.widget.ConnectEnterEvent(func(event *core.QEvent) {
		svgContent := e.getSvg("cross", nil)
		notification.closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	})
	notification.widget.ConnectLeaveEvent(func(event *core.QEvent) {
		svgContent := e.getSvg("cross", editor.colors.widgetBg)
		notification.closeIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
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
		if !notification.isMoved {
			notification.dropNotifications()
		}
		if notification.isDrag {
			x := event.Pos().X() - notification.pos.X()
			y := event.Pos().Y() - notification.pos.Y()
			newPos := core.NewQPoint2(x, y)
			trans := notification.widget.MapToParent(newPos)
			notification.widget.Move(trans)
		}
		notification.isMoved = true
	})

	// Drop shadow to widget
	go func() {
		widget.SetGraphicsEffect(util.DropShadow(-2, -1, 40, 200))
	}()

	// Notification hiding
	var displayPeriod int
	if p < 0 { // default display period is 6 seconds
		displayPeriod = 6
	} else if p == 0 {
		displayPeriod = 0
	} else {
		displayPeriod = p
	}
	if displayPeriod > 0 {
		timer := core.NewQTimer(nil)
		timer.SetSingleShot(true)
		timer.ConnectTimeout(notification.hideNotification)
		timer.Start(displayPeriod * 1000)
	}

	return notification
}

func (n *Notification) dropNotifications(fn ...func(*Notification)) {
	e := editor
	var newNotifications []*Notification
	var x, y, self int
	var isClose bool
	var dropOK bool
	dropHeight := 0
	for i, item := range e.notifications {
		if n == item {
			self = i
			dropHeight = item.widget.Height() + 4
			if len(fn) > 0 {
				for _, f := range fn {
					f(item)
				}
			}
		}

		isClose = fmt.Sprintf("%v", item.widget) == "&{{<nil>} {<nil>}}"
		// Skip if widget is broken
		if !isClose {
			newNotifications = append(newNotifications, item)
		}

		// set drop flag when n == item
		if n == item {
			dropOK = (n.isDrag && !n.isHide && !isClose) || (!n.isMoved && n.isHide && !isClose) || (!n.isMoved && !n.isHide && isClose)
		}
		if n != item && i > self && !item.isMoved && dropOK {
			x = item.widget.Pos().X()
			y = item.widget.Pos().Y() + dropHeight
			item.widget.Move2(x, y)
			item.widget.Hide()
			if !item.isHide {
				item.widget.Show()
			}
		}
	}
	if dropOK {
		e.notifyStartPos = core.NewQPoint2(editor.notifyStartPos.X(), editor.notifyStartPos.Y()+dropHeight)
	}
	e.notifications = newNotifications
}

func (n *Notification) closeNotification() {
	n.dropNotifications(func(item *Notification) {
		item.widget.DestroyQWidget()
	})
	editor.pushNotification(NotifyInfo, -1, "") // dummy push
}

func (n *Notification) hideNotification() {
	n.dropNotifications(func(item *Notification) {
		item.widget.Hide()
		item.isHide = true
	})
	editor.isDisplayNotifications = false
	for _, item := range editor.notifications {
		if !item.isHide {
			editor.isDisplayNotifications = true
		}
	}
}

func (e *Editor) showNotifications() {
	e.notifyStartPos = core.NewQPoint2(e.width-e.notificationWidth-10, e.height-30)
	var x, y int
	var newNotifications []*Notification
	for _, item := range e.notifications {
		x = e.notifyStartPos.X()
		y = e.notifyStartPos.Y() - item.widget.Height() - 4
		item.widget.Move2(x, y)
		item.statusReset()
		e.notifyStartPos = core.NewQPoint2(x, y)
		newNotifications = append(newNotifications, item)
	}
	e.notifications = newNotifications
	e.isDisplayNotifications = true
}

func (e *Editor) hideNotifications() {
	var newNotifications []*Notification
	for _, item := range e.notifications {
		item.isHide = true
		item.widget.Hide()
		newNotifications = append(newNotifications, item)
	}
	e.notifications = newNotifications
	e.notifyStartPos = core.NewQPoint2(e.width-e.notificationWidth-10, e.height-30)
	e.isDisplayNotifications = false
}

func (n *Notification) statusReset() {
	n.isHide = false
	n.isMoved = false
	n.widget.Show()
}

func (n *Notification) show() {
	for {
		if editor.isSetGuiColor {
			break
		} else {
			time.Sleep(100 * time.Millisecond)
		}
	}
	fg := editor.colors.widgetFg.String()
	bg := editor.colors.widgetBg
	// transparent := editor.config.Editor.Transparent / 2.0
	transparent := transparent()
	n.widget.SetStyleSheet(fmt.Sprintf(" * {color: %s; background: rgba(%d, %d, %d, %f);}", fg, bg.R, bg.G, bg.B, transparent))
	n.widget.Show()
}

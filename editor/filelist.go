package editor

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

type Filelist struct {
	WSitem    *WorkspaceSideItem
	widget    *widgets.QWidget
	Fileitems []*Fileitem
	isload    bool
	active    int
}

type Fileitem struct {
	fl             *Filelist
	widget         *widgets.QWidget
	fileIcon       *svg.QSvgWidget
	fileType       string
	file           *widgets.QLabel
	fileText       string
	fileName       string
	filenameWidget *widgets.QWidget
	path           string
	fileModified   *svg.QSvgWidget
	isOpened       bool
	isModified     string
}

func newFilelistwidget(path string) *Filelist {
	fileitems := []*Fileitem{}
	lsfiles, _ := ioutil.ReadDir(path)

	filelist := &Filelist{}
	filelist.active = -1

	filelistwidget := widgets.NewQWidget(nil, 0)
	// filelistwidget.SetSizePolicy2(widgets.QSizePolicy__Maximum, widgets.QSizePolicy__Maximum)
	filelistwidget.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Expanding)
	filelistlayout := widgets.NewQBoxLayout(widgets.QBoxLayout__TopToBottom, filelistwidget)
	filelistlayout.SetContentsMargins(0, 0, 0, 0)
	filelistlayout.SetSpacing(1)
	bg := editor.bgcolor

	filewidgetLeftMargin := 35

	// var filewidgetMarginBuf int
	// if runtime.GOOS == "windows" || runtime.GOOS == "linux" {
	// 	filewidgetMarginBuf = 55
	// } else {
	// 	filewidgetMarginBuf = 70
	// }
	width := editor.splitter.Widget(editor.splitter.IndexOf(editor.activity.sideArea)).Width()

	for _, f := range lsfiles {

		filewidget := widgets.NewQWidget(nil, 0)
		filewidget.SetMaximumWidth(width)
		filewidget.SetMinimumWidth(width)
		filewidget.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Expanding)
		// filewidget.SetSizePolicy2(widgets.QSizePolicy__Maximum, widgets.QSizePolicy__Maximum)

		filelayout := widgets.NewQHBoxLayout()
		filelayout.SetContentsMargins(filewidgetLeftMargin, 0, 10, 0)

		fileIcon := svg.NewQSvgWidget(nil)
		fileIcon.SetFixedWidth(11)
		fileIcon.SetFixedHeight(11)

		filenameWidget := widgets.NewQWidget(nil, 0)
		filenameWidget.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Expanding)
		// filenameWidget.SetSizePolicy2(widgets.QSizePolicy__Maximum, widgets.QSizePolicy__Maximum)
		filenameLayout := widgets.NewQHBoxLayout()
		filenameLayout.SetContentsMargins(0, 5, 5, 5)
		filenameLayout.SetSpacing(0)
		file := widgets.NewQLabel(nil, 0)
		file.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Expanding)
		//file.SetContentsMargins(0, 5, 10, 5)
		file.SetContentsMargins(0, 0, 0, 0)
		// file.SetFont(editor.workspaces[editor.active].font.fontNew)

		fileModified := svg.NewQSvgWidget(nil)
		fileModified.SetFixedWidth(11)
		fileModified.SetFixedHeight(11)
		fileModified.SetContentsMargins(0, 0, 0, 0)
		// Hide with the same color as the background
		svgModified := editor.workspaces[editor.active].getSvg("circle", newRGBA(shiftColor(bg, -5).R, shiftColor(bg, -5).G, shiftColor(bg, -5).B, 1))
		fileModified.Load2(core.NewQByteArray2(svgModified, len(svgModified)))

		filename := f.Name()

		filenameLayout.AddWidget(file, 0, 0)

		filenameWidget.SetLayout(filenameLayout)

		filepath := filepath.Join(path, f.Name())
		finfo, err := os.Stat(filepath)
		if err != nil {
			continue
		}
		var filetype string

		if finfo.IsDir() {
			filetype = "/"
			svgContent := editor.workspaces[editor.active].getSvg("directory", nil)
			fileIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		} else {
			filetype = getFileType(filename)
			svgContent := editor.workspaces[editor.active].getSvg(filetype, nil)
			fileIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		}

		filelayout.AddWidget(fileIcon, 0, 0)
		filelayout.AddWidget(filenameWidget, 0, 0)
		filelayout.AddWidget(fileModified, 0, 0)
		filewidget.SetLayout(filelayout)
		filewidget.SetAttribute(core.Qt__WA_Hover, true)

		fileitem := &Fileitem{
			fl:             filelist,
			widget:         filewidget,
			fileText:       filename,
			fileName:       f.Name(),
			filenameWidget: filenameWidget,
			file:           file,
			fileIcon:       fileIcon,
			fileType:       filetype,
			path:           filepath,
			fileModified:   fileModified,
		}
		// charWidth := int(editor.workspaces[editor.active].font.defaultFontMetrics.Width("W"))
		charWidth := int(editor.workspaces[editor.active].font.defaultFontMetrics.HorizontalAdvance("W", -1))
		maxfilenameLength := float64(width - fileIcon.Width() - fileModified.Width() - filewidgetLeftMargin - charWidth - 35)
		fileitem.setFilename(maxfilenameLength)

		fileitem.widget.ConnectEnterEvent(fileitem.enterEvent)
		fileitem.widget.ConnectLeaveEvent(fileitem.leaveEvent)
		fileitem.widget.ConnectMousePressEvent(fileitem.mouseEvent)

		fileitems = append(fileitems, fileitem)
		filelistlayout.AddWidget(filewidget, 0, 0)
	}
	filelistwidget.SetLayout(filelistlayout)

	filelist.widget = filelistwidget
	filelist.Fileitems = fileitems
	filelist.isload = true

	editor.wsSide.scrollarea.ConnectResizeEvent(func(*gui.QResizeEvent) {
		if editor.activity.editItem.active == false {
			return
		}

		ws := editor.wsSide.items[editor.active]
		if len(ws.Filelist.Fileitems) == 0 {
			return
		}
		filewidgetLeftMargin := 35
		currFileItem := ws.Filelist.Fileitems[0]
		width := editor.splitter.Widget(editor.splitter.IndexOf(editor.activity.sideArea)).Width()
		// charWidth := int(editor.workspaces[editor.active].font.defaultFontMetrics.Width("W"))
		charWidth := int(editor.workspaces[editor.active].font.defaultFontMetrics.HorizontalAdvance("W", -1))
		length := float64(width - currFileItem.fileIcon.Width() - currFileItem.fileModified.Width() - filewidgetLeftMargin - charWidth - 35)

		for _, item := range editor.wsSide.items {
			item.label.SetMaximumWidth(editor.activity.sideArea.Width())
			item.label.SetMinimumWidth(editor.activity.sideArea.Width())
			for _, fileitem := range item.Filelist.Fileitems {
				fileitem.widget.SetMaximumWidth(width)
				fileitem.widget.SetMinimumWidth(width)
				fileitem.setFilename(length)
			}
		}
	})

	return filelist
}

func (f *Fileitem) setFilename(length float64) {
	metrics := gui.NewQFontMetricsF(gui.NewQFont())
	elidedfilename := metrics.ElidedText(f.fileText, core.Qt__ElideRight, length, 0)
	f.file.Clear()
	f.file.SetText(elidedfilename)
}

// func (f *Fileitem) resizeEvent(event *gui.QResizeEvent) {
// width := editor.config.sideWidth
// maxfilenameLength := float64(width-(filewidgetLeftMargin+filewidgetMarginBuf))
// metrics := gui.NewQFontMetricsF(gui.NewQFont())
// elidedfilename := metrics.ElidedText(filename, core.Qt__ElideRight, maxfilenameLength, 0)
// file.SetText(elidedfilename)
// filenameLayout.AddWidget(file, 0, 0)
// }

func (f *Fileitem) enterEvent(event *core.QEvent) {
	fg := editor.fgcolor
	bg := editor.bgcolor
	var svgModified string
	cfn := ""
	cfnITF, err := editor.workspaces[editor.active].nvimEval("expand('%:t')")
	if err != nil {
		cfn = ""
	} else {
		cfn = cfnITF.(string)
	}

	if cfn == f.fileName {
		f.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: rgba(%d, %d, %d); }", shiftColor(bg, -15).R, shiftColor(bg, -15).G, shiftColor(bg, -15).B))
		if f.isModified == "1" {
			svgModified = editor.workspaces[editor.active].getSvg("circle", newRGBA(gradColor(fg).R, gradColor(fg).G, gradColor(fg).B, 1))
		} else {
			svgModified = editor.workspaces[editor.active].getSvg("circle", newRGBA(shiftColor(bg, -15).R, shiftColor(bg, -15).G, shiftColor(bg, -15).B, 1))
		}
		f.fileModified.Load2(core.NewQByteArray2(svgModified, len(svgModified)))
	} else {
		f.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: rgba(%d, %d, %d); text-decoration: underline; } ", shiftColor(bg, -9).R, shiftColor(bg, -9).G, shiftColor(bg, -9).B))
		if f.isModified == "1" {
			svgModified = editor.workspaces[editor.active].getSvg("circle", newRGBA(gradColor(fg).R, gradColor(fg).G, gradColor(fg).B, 1))
		} else {
			svgModified = editor.workspaces[editor.active].getSvg("circle", newRGBA(shiftColor(bg, -9).R, shiftColor(bg, -9).G, shiftColor(bg, -9).B, 1))
		}
		f.fileModified.Load2(core.NewQByteArray2(svgModified, len(svgModified)))
	}
	cursor := gui.NewQCursor()
	cursor.SetShape(core.Qt__PointingHandCursor)
	gui.QGuiApplication_SetOverrideCursor(cursor)
}

func (f *Fileitem) leaveEvent(event *core.QEvent) {
	bg := editor.bgcolor
	var svgModified string

	cfn := ""
	cfnITF, err := editor.workspaces[editor.active].nvimEval("expand('%:t')")
	if err != nil {
		cfn = ""
	}
	cfn = cfnITF.(string)

	if cfn != f.fileName {
		f.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: rgba(%d, %d, %d); text-decoration: none; } ", shiftColor(bg, -5).R, shiftColor(bg, -5).G, shiftColor(bg, -5).B))
		svgModified = editor.workspaces[editor.active].getSvg("circle", newRGBA(shiftColor(bg, -5).R, shiftColor(bg, -5).G, shiftColor(bg, -5).B, 1))
		f.fileModified.Load2(core.NewQByteArray2(svgModified, len(svgModified)))
	}
	gui.QGuiApplication_RestoreOverrideCursor()
}

func (f *Fileitem) mouseEvent(event *gui.QMouseEvent) {
	editor.workspaces[editor.active].nvim.Command(":e " + f.path)
	f.fl.WSitem.setCurrentFileLabel()
}

func (i *WorkspaceSideItem) setCurrentFileLabel() {
	bg := editor.bgcolor
	var svgModified string

	cfn := ""
	cfnITF, err := editor.workspaces[editor.active].nvimEval("expand('%:t')")
	if err != nil {
		cfn = ""
	} else {
		cfn = cfnITF.(string)
	}

	for j, fileitem := range i.Filelist.Fileitems {
		if fileitem.fileName != cfn {
			fileitem.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: rgba(%d, %d, %d); }", shiftColor(bg, -5).R, shiftColor(bg, -5).G, shiftColor(bg, -5).B))
			svgModified = editor.workspaces[editor.active].getSvg("circle", newRGBA(shiftColor(bg, -5).R, shiftColor(bg, -5).G, shiftColor(bg, -5).B, 1))
			fileitem.fileModified.Load2(core.NewQByteArray2(svgModified, len(svgModified)))
			fileitem.isOpened = false
		} else {
			fileitem.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: rgba(%d, %d, %d); }", shiftColor(bg, -15).R, shiftColor(bg, -15).G, shiftColor(bg, -15).B))
			svgModified = editor.workspaces[editor.active].getSvg("circle", newRGBA(shiftColor(bg, -15).R, shiftColor(bg, -15).G, shiftColor(bg, -15).B, 1))
			fileitem.fileModified.Load2(core.NewQByteArray2(svgModified, len(svgModified)))
			fileitem.isOpened = true
			i.Filelist.active = j
		}
	}
}

func (f *Fileitem) updateModifiedbadge() {
	var isModified string
	isModified, err := editor.workspaces[editor.active].nvimCommandOutput("echo &modified")
	if err != nil {
		isModified = ""
	}
	if isModified == "" {
		return
	}

	fg := editor.fgcolor
	bg := editor.bgcolor
	var svgModified string
	if isModified == "1" {
		svgModified = editor.workspaces[editor.active].getSvg("circle", newRGBA(gradColor(fg).R, gradColor(fg).G, gradColor(fg).B, 1))
	} else {
		if f.isOpened {
			svgModified = editor.workspaces[editor.active].getSvg("circle", newRGBA(shiftColor(bg, -15).R, shiftColor(bg, -15).G, shiftColor(bg, -15).B, 1))
		} else {
			svgModified = editor.workspaces[editor.active].getSvg("circle", newRGBA(shiftColor(bg, -5).R, shiftColor(bg, -5).G, shiftColor(bg, -5).B, 1))
		}
	}
	f.fileModified.Load2(core.NewQByteArray2(svgModified, len(svgModified)))
	f.isModified = isModified
}

func unicodeCount(str string) int {
	count := 0
	for _, r := range str {
		if utf8.RuneLen(r) >= 2 {
			count++
		}
	}
	return count
}

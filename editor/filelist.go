package editor

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

type Filelist struct {
	WSitem    *WorkspaceSideItem
	widget    *widgets.QWidget
	Fileitems []*Fileitem
	active    int
	cwdpath   string
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
	isModified     bool
}

func newFilelistwidget(path string) *Filelist {
	fileitems := []*Fileitem{}
	lsfiles, _ := ioutil.ReadDir(path)

	filelist := &Filelist{}
	filelist.active = -1
	filelist.cwdpath = path

	filelistwidget := widgets.NewQWidget(nil, 0)
	// filelistwidget.SetSizePolicy2(widgets.QSizePolicy__Maximum, widgets.QSizePolicy__Maximum)
	filelistwidget.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Expanding)
	filelistlayout := widgets.NewQBoxLayout(widgets.QBoxLayout__TopToBottom, filelistwidget)
	filelistlayout.SetContentsMargins(0, 0, 0, 0)
	filelistlayout.SetSpacing(1)
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
		filelayout.SetContentsMargins(filewidgetLeftMargin, 0, 0, 0)

		fileIcon := svg.NewQSvgWidget(nil)
		fileIcon.SetFixedWidth(editor.iconSize)
		fileIcon.SetFixedHeight(editor.iconSize)

		filenameWidget := widgets.NewQWidget(nil, 0)
		filenameWidget.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Expanding)
		// filenameWidget.SetSizePolicy2(widgets.QSizePolicy__Maximum, widgets.QSizePolicy__Maximum)
		filenameLayout := widgets.NewQHBoxLayout()
		filenameLayout.SetContentsMargins(0, editor.config.Editor.Linespace/2, 0, editor.config.Editor.Linespace/2)
		filenameLayout.SetSpacing(0)
		file := widgets.NewQLabel(nil, 0)
		file.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Expanding)
		//file.SetContentsMargins(0, 5, 10, 5)
		file.SetContentsMargins(0, 0, 0, 0)
		file.SetFont(gui.NewQFont2(editor.config.Editor.FontFamily, editor.config.Editor.FontSize, 1, false))

		fileModified := svg.NewQSvgWidget(nil)
		fileModified.SetFixedWidth(editor.iconSize)
		fileModified.SetFixedHeight(editor.iconSize)
		fileModified.SetContentsMargins(0, 0, 0, 0)
		// Hide with the same color as the background
		svgModified := editor.getSvg("circle", editor.colors.sideBarBg)
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
			svgContent := editor.getSvg("directory", nil)
			fileIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		} else {
			filetype = getFileType(filename)
			svgContent := editor.getSvg(filetype, nil)
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

func (f *Fileitem) enterEvent(event *core.QEvent) {
	c := editor.colors.selectedBg.String()
	currFilepath := editor.workspaces[editor.active].filepath
	cfn := filepath.Base(currFilepath)
	if cfn == f.fileName {
		f.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: %s; }", c))
	} else {
		f.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: %s; text-decoration: underline; } ", c))
	}
	f.loadModifiedBadge()
	cursor := gui.NewQCursor()
	cursor.SetShape(core.Qt__PointingHandCursor)
	gui.QGuiApplication_SetOverrideCursor(cursor)
}

func (f *Fileitem) leaveEvent(event *core.QEvent) {
	c := editor.colors.sideBarBg
	svgModified := ""
	currFilepath := editor.workspaces[editor.active].filepath
	cfn := filepath.Base(currFilepath)

	if cfn != f.fileName {
		f.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: %s; text-decoration: none; } ", c.String()))
		svgModified = editor.getSvg("circle", c)
		f.fileModified.Load2(core.NewQByteArray2(svgModified, len(svgModified)))
	}
	gui.QGuiApplication_RestoreOverrideCursor()
}

func (f *Fileitem) mouseEvent(event *gui.QMouseEvent) {
	editor.workspaces[editor.active].nvim.Command(":e " + f.path)
	f.fl.WSitem.setCurrentFileLabel()
}

func (i *WorkspaceSideItem) setCurrentFileLabel() {
	if !editor.activity.editItem.active {
		return
	}

	bg := editor.colors.sideBarBg.String()
	sbg := editor.colors.selectedBg.String()
	currFilepath := editor.workspaces[editor.active].filepath
	isMatchPath := filepath.Dir(currFilepath) == editor.workspaces[editor.active].cwd
	for j, fileitem := range i.Filelist.Fileitems {
		if !isMatchPath || filepath.Base(currFilepath) != fileitem.fileName {
			if !fileitem.isOpened {
				continue
			}
			fileitem.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: %s; }", bg))
			fileitem.isOpened = false
		} else {
			fileitem.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: %s; }", sbg))
			fileitem.isOpened = true
			i.Filelist.active = j
		}
		fileitem.loadModifiedBadge()
	}
}


func (f *Fileitem) setColor() {
	fg := editor.colors.fg
	fmt.Println("fg: ", fg.String())
	switch f.fileType {
	case "/":
		svgContent := editor.getSvg("directory", fg)
		f.fileIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))

	default:
		svgContent := editor.getSvg(f.fileType, fg)
		f.fileIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	}

}

func (f *Fileitem) updateModifiedbadge() {
	if !editor.activity.editItem.active {
		return
	}

	// err := editor.workspaces[editor.active].nvim.Call("modified", &f.isModified)
	// if err != nil {
	// 	return
	// }

	var isModified string
	isModified, err := editor.workspaces[editor.active].nvimCommandOutput("echo &modified")
	if err != nil {
		isModified = ""
	}
	if isModified == "" {
		return
	}
	if isModified == "1" {
		f.isModified = true
	} else {
		f.isModified = false
	}

	f.loadModifiedBadge()
}

func (f *Fileitem) loadModifiedBadge() {
	fg := editor.colors.fg
	bg := editor.colors.sideBarBg
	selectedBg := editor.colors.selectedBg
	var svgModified string
	if f.isModified {
		svgModified = editor.getSvg("circle", fg)
	} else {
		if f.isOpened {
			svgModified = editor.getSvg("circle", selectedBg)
		} else {
			svgModified = editor.getSvg("circle", bg)
		}
	}
	f.fileModified.Load2(core.NewQByteArray2(svgModified, len(svgModified)))
}


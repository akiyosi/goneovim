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

const (
	// FilewidgetLeftMargin is left margin in filelist
	FilewidgetLeftMargin = 35
)

// Filelist is
type Filelist struct {
	WSitem    *WorkspaceSideItem
	widget    *widgets.QWidget
	Fileitems []*Fileitem
	active    int
	cwdpath   string
}

// Fileitem is
type Fileitem struct {
	fl             *Filelist
	widget         *widgets.QWidget
	fileIcon       *svg.QSvgWidget
	fileType       string
	fileLabel      *widgets.QLabel
	fileText       string
	fileName       string
	filenameWidget *widgets.QWidget
	path           string
	fileModified   *svg.QSvgWidget
	isOpened       bool
	isModified     bool
}

func newFilelist(path string) (*Filelist, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	filelist := &Filelist{
		Fileitems: []*Fileitem{},
	}

	filelist.active = -1
	filelist.cwdpath = path

	filelistwidget := widgets.NewQWidget(nil, 0)
	filelistwidget.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Expanding)
	filelistlayout := widgets.NewQBoxLayout(widgets.QBoxLayout__TopToBottom, filelistwidget)
	filelistlayout.SetContentsMargins(0, 0, 0, 0)
	filelistlayout.SetSpacing(1)
	filelistwidget.SetLayout(filelistlayout)
	filelist.widget = filelistwidget

	width := editor.splitter.Widget(editor.splitter.IndexOf(editor.activity.sideArea)).Width()

	// go func() {
	fl := editor.wsSide.items[editor.active].Filelist
	parentDirItem := &Fileitem{
		fl:       fl,
		fileText: "",
		fileName: "",
		fileType: "..",
		path:     "",
	}
	parentDirItem.makeWidget(width)
	filelist.Fileitems = append(filelist.Fileitems, parentDirItem)
	filelistlayout.AddWidget(parentDirItem.widget, 0, 0)
	for i, f := range files {
		if i >= editor.config.FileExplorer.MaxItems {
			break
		}
		fileitem, err := fl.newFileitem(f, path)
		if err != nil {
			continue
		}

		fileitem.makeWidget(width)
		// fileitem.widget.MoveToThread(editor.app.Thread())  // Can't work...

		// // Use signal
		// // Too Slow...
		// fl.WSitem.filelistUpdate <- fileitem
		// fl.WSitem.signal.FilelistUpdateSignal()

		filelist.Fileitems = append(filelist.Fileitems, fileitem)
		filelistlayout.AddWidget(fileitem.widget, 0, 0)

	}

	return filelist, nil
}

func (f *Fileitem) makeWidget(width int) {
	filewidget := widgets.NewQWidget(nil, 0)
	filelayout := widgets.NewQHBoxLayout()
	fileIcon := svg.NewQSvgWidget(nil)
	fileLabel := widgets.NewQLabel(nil, 0)
	fileModified := svg.NewQSvgWidget(nil)

	f.fileLabel = fileLabel
	f.fileIcon = fileIcon
	f.fileModified = fileModified

	f.widget = filewidget

	go func() {
		filewidget.SetMaximumWidth(width)
		filewidget.SetMinimumWidth(width)
		filewidget.SetStyleSheet(" * {background-color: rgba(0, 0, 0, 0);}")

		f.widget.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Expanding)

		f.fileIcon.SetFixedSize2(editor.iconSize, editor.iconSize)

		f.fileLabel.SetSizePolicy2(widgets.QSizePolicy__Expanding, widgets.QSizePolicy__Expanding)
		f.fileLabel.SetContentsMargins(0, 0, 0, 0)
		f.fileLabel.SetContentsMargins(0, editor.config.Editor.Linespace/2, 0, editor.config.Editor.Linespace/2)

		f.fileModified.SetFixedSize2(editor.iconSize, editor.iconSize)
		f.fileModified.SetContentsMargins(0, 0, 0, 0)

		filelayout.SetContentsMargins(FilewidgetLeftMargin, 0, 0, 0)
		filelayout.AddWidget(f.fileIcon, 0, 0)
		filelayout.AddWidget(f.fileLabel, 0, 0)
		filelayout.AddWidget(f.fileModified, 0, 0)

		f.widget.SetAttribute(core.Qt__WA_Hover, true)
		f.widget.SetLayout(filelayout)

		f.fileLabel.SetText(f.fileText)

		// Hide with the same color as the background
		svgModified := editor.getSvg("circle", editor.colors.sideBarBg)
		f.fileModified.Load2(core.NewQByteArray2(svgModified, len(svgModified)))
		f.fileModified.Hide()

		switch f.fileType {
		case "..":
			svgContent := editor.getSvg("backParentDir", nil)
			f.fileIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		case "/":
			svgContent := editor.getSvg("directory", nil)
			f.fileIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		default:
			svgContent := editor.getSvg(f.fileType, nil)
			f.fileIcon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
		}

		f.widget.ConnectEnterEvent(f.enterEvent)
		f.widget.ConnectLeaveEvent(f.leaveEvent)
		f.widget.ConnectMousePressEvent(f.mouseEvent)
	}()
}

func (fl *Filelist) newFileitem(f os.FileInfo, path string) (*Fileitem, error) {

	filename := f.Name()
	filepath := filepath.Join(path, f.Name())

	finfo, err := os.Stat(filepath)
	if err != nil {
		return nil, err
	}
	filetype := ""
	if finfo.IsDir() {
		filetype = "/"
	} else {
		filetype = getFileType(filename)
	}

	fileitem := &Fileitem{
		fl:       fl,
		fileText: filename,
		fileName: f.Name(),
		fileType: filetype,
		path:     filepath,
	}

	return fileitem, nil
}

func (f *Fileitem) setFilename(length float64) {
	metrics := gui.NewQFontMetricsF(gui.NewQFont())
	elidedfilename := metrics.ElidedText(f.fileText, core.Qt__ElideRight, length, 0)
	f.fileLabel.Clear()
	go f.fileLabel.SetText(elidedfilename)
}

func (f *Fileitem) enterEvent(event *core.QEvent) {
	fg := editor.colors.fg
	selectedBg := editor.colors.selectedBg

	currFilepath := editor.workspaces[editor.active].filepath
	cfn := filepath.Base(currFilepath)
	if cfn == f.fileName {
		f.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: %s; }", selectedBg.StringTransparent()))
	} else {
		f.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: %s; text-decoration: underline; } ", selectedBg.StringTransparent()))
	}

	var svgModified string
	if f.isModified {
		svgModified = editor.getSvg("circle", fg)
	} else {
		svgModified = editor.getSvg("circle", selectedBg)
	}
	f.fileModified.Load2(core.NewQByteArray2(svgModified, len(svgModified)))

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
		f.widget.SetStyleSheet(" * { background-color: rgba(0, 0, 0, 0); text-decoration: none; } ")
		svgModified = editor.getSvg("circle", c)
		f.fileModified.Load2(core.NewQByteArray2(svgModified, len(svgModified)))
	}
	gui.QGuiApplication_RestoreOverrideCursor()
}

func (f *Fileitem) mouseEvent(event *gui.QMouseEvent) {
	openCommand := ""
	switch f.fileType {
	case "/":
		openCommand = ":cd " + f.path
	case "..":
		openCommand = ":cd .."
	default:
		if editor.config.FileExplorer.OpenCmd == "" {
			openCommand = ":e " + f.path
		} else {
			openCommand = editor.config.FileExplorer.OpenCmd + " " + f.path
		}

	}
	go editor.workspaces[editor.active].nvim.Command(openCommand)
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
	isShow := false
	var svgModified string
	if f.isModified {
		svgModified = editor.getSvg("circle", fg)
		isShow = true
	} else {
		if f.isOpened {
			svgModified = editor.getSvg("circle", selectedBg)
		} else {
			svgModified = editor.getSvg("circle", bg)
		}
	}
	f.fileModified.Load2(core.NewQByteArray2(svgModified, len(svgModified)))
	if isShow {
		f.fileModified.Show()
	} else {
		f.fileModified.Hide()
	}
}

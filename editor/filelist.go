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
	fl           *Filelist
	widget       *widgets.QWidget
	fileIcon     *svg.QSvgWidget
	fileType     string
	file         *widgets.QLabel
	fileText     string
	fileName     string
	path         string
	fileModified *svg.QSvgWidget
	isOpened     bool
}

func newFilelistwidget(path string) *Filelist {
	fileitems := []*Fileitem{}
	lsfiles, _ := ioutil.ReadDir(path)

	filelist := &Filelist{}
	filelist.active = -1

	filelistwidget := widgets.NewQWidget(nil, 0)
	filelistlayout := widgets.NewQBoxLayout(widgets.QBoxLayout__TopToBottom, filelistwidget)
	filelistlayout.SetContentsMargins(0, 0, 0, 0)
	filelistlayout.SetSpacing(1)
	bg := editor.bgcolor
	width := editor.config.sideWidth
	filewidgetLeftMargin := 35
	filewidgetMarginBuf := 75
	maxfilenameLength := int(float64(width-(filewidgetLeftMargin+filewidgetMarginBuf)) / float64(editor.workspaces[editor.active].font.truewidth))

	for _, f := range lsfiles {

		filewidget := widgets.NewQWidget(nil, 0)

		filelayout := widgets.NewQHBoxLayout()
		filelayout.SetContentsMargins(filewidgetLeftMargin, 0, 10, 0)

		fileIcon := svg.NewQSvgWidget(nil)
		fileIcon.SetFixedWidth(11)
		fileIcon.SetFixedHeight(11)

		filenameWidget := widgets.NewQWidget(nil, 0)
		filenameLayout := widgets.NewQHBoxLayout()
		filenameLayout.SetContentsMargins(0, 5, 10, 5)
		filenameLayout.SetSpacing(0)
		file := widgets.NewQLabel(nil, 0)
		//file.SetContentsMargins(0, 5, 10, 5)
		file.SetContentsMargins(0, 0, 0, 0)
		file.SetFont(editor.workspaces[editor.active].font.fontNew)

		fileModified := svg.NewQSvgWidget(nil)
		fileModified.SetFixedWidth(11)
		fileModified.SetFixedHeight(11)
		fileModified.SetContentsMargins(0, 0, 0, 0)

		filename := f.Name()
		multibyteCharNum := unicodeCount(filename)
		filenamerune := []rune(filename)
		filenameDisplayLength := len(filenamerune) + multibyteCharNum

		if filenameDisplayLength > maxfilenameLength {
			if multibyteCharNum > 0 {
				for filenameDisplayLength > maxfilenameLength {
					filename = string(filenamerune[:(len(filenamerune) - 1)])
					filenamerune = []rune(filename)
					multibyteCharNum = unicodeCount(filename)
					filenameDisplayLength = len(filenamerune) + multibyteCharNum
				}
			} else {
				filename = filename[:maxfilenameLength]
			}
			moreIcon := svg.NewQSvgWidget(nil)
			moreIcon.SetFixedWidth(11)
			moreIcon.SetFixedHeight(11)
			svgMoreDotsContent := editor.workspaces[editor.active].getSvg("moredots", nil)
			moreIcon.Load2(core.NewQByteArray2(svgMoreDotsContent, len(svgMoreDotsContent)))
			file.SetText(filename)
			filenameLayout.AddWidget(file, 0, 0)
			filenameLayout.AddWidget(moreIcon, 0, 0)
		} else {
			file.SetText(filename)
			filenameLayout.AddWidget(file, 0, 0)
		}
		filenameWidget.SetLayout(filenameLayout)

		filepath := filepath.Join(path, f.Name())
		finfo, _ := os.Stat(filepath)
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

		svgModified := editor.workspaces[editor.active].getSvg("circle", newRGBA(shiftColor(bg, -5).R, shiftColor(bg, -5).G, shiftColor(bg, -5).B, 1))
		fileModified.Load2(core.NewQByteArray2(svgModified, len(svgModified)))

		filelayout.AddWidget(fileIcon, 0, 0)
		filelayout.AddWidget(filenameWidget, 0, 0)
		filelayout.AddWidget(fileModified, 0, 0)
		filewidget.SetLayout(filelayout)
		filewidget.SetAttribute(core.Qt__WA_Hover, true)

		fileitem := &Fileitem{
			fl:           filelist,
			widget:       filewidget,
			fileText:     filename,
			fileName:     f.Name(),
			file:         file,
			fileIcon:     fileIcon,
			fileType:     filetype,
			path:         filepath,
			fileModified: fileModified,
		}

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

	return filelist
}

func (f *Fileitem) enterEvent(event *core.QEvent) {
	bg := editor.bgcolor
	var svgModified string
	cfn := ""
	editor.workspaces[editor.active].nvim.Eval("expand('%:t')", &cfn)
	if cfn == f.fileName {
		f.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: rgba(%d, %d, %d); }", shiftColor(bg, -15).R, shiftColor(bg, -15).G, shiftColor(bg, -15).B))
		svgModified = editor.workspaces[editor.active].getSvg("circle", newRGBA(shiftColor(bg, -15).R, shiftColor(bg, -15).G, shiftColor(bg, -15).B, 1))
		f.fileModified.Load2(core.NewQByteArray2(svgModified, len(svgModified)))
	} else {
		f.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: rgba(%d, %d, %d); text-decoration: underline; } ", shiftColor(bg, -9).R, shiftColor(bg, -9).G, shiftColor(bg, -9).B))
		svgModified = editor.workspaces[editor.active].getSvg("circle", newRGBA(shiftColor(bg, -9).R, shiftColor(bg, -9).G, shiftColor(bg, -9).B, 1))
		f.fileModified.Load2(core.NewQByteArray2(svgModified, len(svgModified)))
	}
}

func (f *Fileitem) leaveEvent(event *core.QEvent) {
	bg := editor.bgcolor
	var svgModified string
	cfn := ""
	editor.workspaces[editor.active].nvim.Eval("expand('%:t')", &cfn)
	if cfn != f.fileName {
		f.widget.SetStyleSheet(fmt.Sprintf(" * { background-color: rgba(%d, %d, %d); text-decoration: none; } ", shiftColor(bg, -5).R, shiftColor(bg, -5).G, shiftColor(bg, -5).B))
		svgModified = editor.workspaces[editor.active].getSvg("circle", newRGBA(shiftColor(bg, -5).R, shiftColor(bg, -5).G, shiftColor(bg, -5).B, 1))
		f.fileModified.Load2(core.NewQByteArray2(svgModified, len(svgModified)))
	}
}

func (f *Fileitem) mouseEvent(event *gui.QMouseEvent) {
	editor.workspaces[editor.active].nvim.Command(":e " + f.path)
	f.fl.WSitem.setCurrentFileLabel()
}

func (i *WorkspaceSideItem) setCurrentFileLabel() {
	bg := editor.bgcolor
	var svgModified string
	cfn := ""
	editor.workspaces[editor.active].nvim.Eval("expand('%:t')", &cfn)

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
	isModified, _ = editor.workspaces[editor.active].nvim.CommandOutput("echo &modified")

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

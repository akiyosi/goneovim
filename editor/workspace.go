package editor

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/akiyosi/gonvim/fuzzy"
	shortpath "github.com/akiyosi/short_path"
	"github.com/neovim/go-client/nvim"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

type workspaceSignal struct {
	core.QObject
	_ func() `signal:"markdownSignal"`
	_ func() `signal:"stopSignal"`
	_ func() `signal:"redrawSignal"`
	_ func() `signal:"guiSignal"`
	_ func() `signal:"statuslineSignal"`
	_ func() `signal:"locpopupSignal"`
	_ func() `signal:"lintSignal"`
	_ func() `signal:"gitSignal"`
	_ func() `signal:"messageSignal"`
}

// Workspace is an editor workspace
type Workspace struct {
	widget     *widgets.QWidget
	font       *Font
	cursor     *Cursor
	tabline    *Tabline
	statusline *Statusline
	screen     *Screen
	markdown   *Markdown
	finder     *Finder
	palette    *Palette
	popup      *PopupMenu
	loc        *Locpopup
	cmdline    *Cmdline
	signature  *Signature
	message    *Message
	svgs       map[string]*SvgXML
	svgsOnce   sync.Once
	width      int
	height     int
	hidden     bool

	nvim       *nvim.Nvim
	rows       int
	cols       int
	uiAttached bool
	foreground *RGBA
	background *RGBA
	special    *RGBA
	mode       string
	cwd        string
	cwdBase    string
	cwdlabel   string

	signal        *workspaceSignal
	redrawUpdates chan [][]interface{}
	guiUpdates    chan []interface{}
	stopOnce      sync.Once
	stop          chan struct{}

	drawStatusline bool
	drawTabline    bool
	drawLint       bool

	setGuiFgColor bool
	setGuiBgColor bool
}

func newWorkspace(path string) (*Workspace, error) {
	w := &Workspace{
		stop:          make(chan struct{}),
		signal:        NewWorkspaceSignal(nil),
		redrawUpdates: make(chan [][]interface{}, 1000),
		guiUpdates:    make(chan []interface{}, 1000),
		setGuiFgColor: false,
		setGuiBgColor: false,
	}
	w.signal.ConnectRedrawSignal(func() {
		updates := <-w.redrawUpdates
		w.handleRedraw(updates)
	})
	w.signal.ConnectGuiSignal(func() {
		updates := <-w.guiUpdates
		w.handleRPCGui(updates)
	})
	w.signal.ConnectStopSignal(func() {
		workspaces := []*Workspace{}
		index := 0
		for i, ws := range editor.workspaces {
			if ws != w {
				workspaces = append(workspaces, ws)
			} else {
				index = i
			}
		}
		if len(workspaces) == 0 {
			editor.close()
			return
		}
		editor.workspaces = workspaces
		w.hide()
		if editor.active == index {
			if index > 0 {
				editor.active--
			}
			editor.workspaceUpdate()
		}
	})
	fontFamily := ""
	switch runtime.GOOS {
	case "windows":
		fontFamily = "Consolas"
	case "darwin":
		fontFamily = "Courier New"
	default:
		fontFamily = "Monospace"
	}
	w.font = initFontNew(fontFamily, 14, 6)
	w.tabline = newTabline()
	w.tabline.ws = w
	w.statusline = initStatuslineNew()
	w.statusline.ws = w
	w.screen = newScreen()
	w.screen.toolTipFont(w.font)
	w.screen.ws = w
	w.markdown = newMarkdown(w)
	w.markdown.webview.SetParent(w.screen.widget)
	w.cursor = initCursorNew()
	w.cursor.widget.SetParent(w.screen.widget)
	w.cursor.ws = w
	w.popup = initPopupmenuNew(w.font)
	w.popup.widget.SetParent(w.screen.widget)
	w.popup.ws = w
	w.finder = initFinder()
	w.finder.ws = w
	w.palette = initPalette()
	w.palette.widget.SetParent(w.screen.widget)
	w.palette.ws = w
	w.loc = initLocpopup()
	w.loc.widget.SetParent(w.screen.widget)
	w.loc.ws = w
	w.signature = initSignature()
	w.signature.widget.SetParent(w.screen.widget)
	w.signature.ws = w
	w.message = initMessage()
	w.message.widget.SetParent(w.screen.widget)
	w.message.ws = w
	w.cmdline = initCmdline()
	w.cmdline.ws = w

	// screenLayout := widgets.NewQHBoxLayout()
	// screenLayout.SetContentsMargins(0, 0, 0, 0)
	// screenLayout.SetSpacing(0)
	// screenWidget := widgets.NewQWidget(nil, 0)
	// screenWidget.SetContentsMargins(0, 0, 0, 0)
	// screenWidget.SetLayout(screenLayout)
	// screenLayout.AddWidget(w.screen.widget, 1, 0)
	// screenLayout.AddWidget(w.markdown.webview, 0, 0)

	layout := widgets.NewQVBoxLayout()
	w.widget = widgets.NewQWidget(nil, 0)
	w.widget.SetContentsMargins(0, 0, 0, 0)
	w.widget.SetLayout(layout)
	w.widget.SetFocusPolicy(core.Qt__WheelFocus)
	w.widget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
	w.widget.ConnectInputMethodEvent(w.InputMethodEvent)
	w.widget.ConnectInputMethodQuery(w.InputMethodQuery)
	editor.wsWidget.SetAttribute(core.Qt__WA_InputMethodEnabled, true)
	editor.wsWidget.ConnectInputMethodEvent(w.InputMethodEvent)
	editor.wsWidget.ConnectInputMethodQuery(w.InputMethodQuery)
	layout.AddWidget(w.tabline.widget, 0, 0)
	layout.AddWidget(w.screen.widget, 1, 0)
	layout.AddWidget(w.statusline.widget, 0, 0)

	//// Drop shadow to statusline
	//go func() {
	// shadow := widgets.NewQGraphicsDropShadowEffect(nil)
	// shadow.SetBlurRadius(120)
	// shadow.SetColor(gui.NewQColor3(0, 0, 0, 50))
	// shadow.SetOffset3(0, -8)
	// w.statusline.widget.SetGraphicsEffect(shadow)
	//}()

	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetSpacing(0)

	w.popup.widget.Hide()
	w.palette.hide()
	w.loc.widget.Hide()
	w.signature.widget.Hide()

	w.widget.SetParent(editor.wsWidget)
	w.widget.Move2(0, 0)
	w.updateSize()

	// err := w.startNvim()
	// if err != nil {
	// 	return nil, err
	// }

	//go w.startNvim(path)
	w.startNvim(path) // fix: akiyosi/gonvim/issues/1

	return w, nil
}

func (w *Workspace) hide() {
	if w.hidden {
		return
	}
	w.hidden = true
	w.widget.Hide()
}

func (w *Workspace) show() {
	if !w.hidden {
		return
	}
	w.hidden = false
	w.widget.Show()
	w.widget.SetFocus2Default()
}

func (w *Workspace) startNvim(path string) error {
	neovim, err := nvim.NewEmbedded(&nvim.EmbedOptions{
		Args: os.Args[1:],
	})
	if err != nil {
		return err
	}
	w.nvim = neovim
	w.nvim.RegisterHandler("Gui", func(updates ...interface{}) {
		w.guiUpdates <- updates
		w.signal.GuiSignal()
	})
	w.nvim.RegisterHandler("redraw", func(updates ...[]interface{}) {
		w.redrawUpdates <- updates
		w.signal.RedrawSignal()
	})
	go func() {
		err := w.nvim.Serve()
		if err != nil {
			fmt.Println(err)
		}
		w.stopOnce.Do(func() {
			close(w.stop)
		})
		w.signal.StopSignal()
	}()

	w.configure()
	w.attachUI(path)
	w.initCwd()

	return nil
}

func (w *Workspace) configure() {
	var drawSplit interface{}
	w.nvim.Var("gonvim_draw_split", &drawSplit)
	if isZero(drawSplit) {
		w.screen.drawSplit = false
	} else {
		w.screen.drawSplit = true
	}

	var drawStatusline interface{}
	w.nvim.Var("gonvim_draw_statusline", &drawStatusline)
	if isZero(drawStatusline) {
		w.drawStatusline = false
	} else {
		w.drawStatusline = true
	}

	var drawTabline interface{}
	w.nvim.Var("gonvim_draw_tabline", &drawTabline)
	if isZero(drawTabline) {
		w.drawTabline = false
	} else {
		w.drawTabline = true
	}

	var drawLint interface{}
	w.nvim.Var("gonvim_draw_lint", &drawLint)
	if isZero(drawLint) {
		w.drawLint = false
	} else {
		w.drawLint = true
	}

	var startFullscreen interface{}
	w.nvim.Var("gonvim_start_fullscreen", &startFullscreen)
	if isTrue(startFullscreen) {
		editor.window.ShowFullScreen()
	}
}

func (w *Workspace) attachUI(path string) error {
	w.nvim.Subscribe("Gui")
	w.nvim.Command("runtime plugin/nvim_gui_shim.vim")
	w.nvim.Command("runtime! ginit.vim")
	w.nvim.Command("let g:gonvim_running=1")
	w.nvim.Command(fmt.Sprintf("command! GonvimVersion echo \"%s\"", editor.version))
	w.workspaceCommands(path)
	w.markdown.commands()
	fuzzy.RegisterPlugin(w.nvim)
	w.tabline.subscribe()
	w.statusline.subscribe()
	w.loc.subscribe()
	w.message.subscribe()
	w.uiAttached = true
	err := w.nvim.AttachUI(w.cols, w.rows, w.attachUIOption())
	if err != nil {
		return err
	}
	return nil
}

func (w *Workspace) workspaceCommands(path string) {
	w.nvim.Command(`autocmd DirChanged * call rpcnotify(0, "Gui", "gonvim_workspace_cwd")`)
	w.nvim.Command(`autocmd BufEnter * call rpcnotify(0, "Gui", "gonvim_workspace_redrawSideItems")`)
	w.nvim.Command(`autocmd TextChanged,TextChangedI,BufEnter,TabEnter * call rpcnotify(0, "Gui", "gonvim_workspace_redrawSideItem")`)
	w.nvim.Command(`command! GonvimWorkspaceNew call rpcnotify(0, 'Gui', 'gonvim_workspace_new')`)
	w.nvim.Command(`command! GonvimWorkspaceNext call rpcnotify(0, 'Gui', 'gonvim_workspace_next')`)
	w.nvim.Command(`command! GonvimWorkspacePrevious call rpcnotify(0, 'Gui', 'gonvim_workspace_previous')`)
	w.nvim.Command(`command! -nargs=1 GonvimWorkspaceSwitch call rpcnotify(0, 'Gui', 'gonvim_workspace_switch', <args>)`)
	if path != "" {
		w.nvim.Command("so " + path)
	}
}

func (w *Workspace) initCwd() {
	cwd := ""
	w.nvim.Eval("getcwd()", &cwd)
	w.nvim.Command("cd " + cwd)
}

func (w *Workspace) setCwd() {
	cwd := ""
	w.nvim.Eval("getcwd()", &cwd)
	if cwd == w.cwd {
		return
	}

	w.cwd = cwd

	var labelpath string
	switch editor.workspacepath {
	case "name":
		labelpath = filepath.Base(cwd)
	case "minimum":
		labelpath, _ = shortpath.Minimum(cwd)
	case "full":
		labelpath, _ = filepath.Abs(cwd)
	default:
		labelpath, _ = filepath.Abs(cwd)
	}
	w.cwdlabel = labelpath
	w.cwdBase = filepath.Base(cwd)
	for i, ws := range editor.workspaces {
		if i >= len(editor.wsSide.items) {
			return
		}
		if ws == w {
			path, _ := filepath.Abs(cwd)
			editor.wsSide.items[i].label.SetText(w.cwdlabel)
			editor.wsSide.items[i].cwdpath = path

			if (len(editor.workspaces) == 1) && (editor.showWorkspaceside == false) {
				return
			}

			filelist := newFilelistwidget(path)
			editor.wsSide.items[i].setFilelistwidget(filelist)

			return
		}
	}
}

func (i *WorkspaceSideItem) setFilelistwidget(f *Filelist) {
	i.layout.RemoveWidget(i.Filelistwidget)
	i.layout.AddWidget(f.widget, 0, 0)
	i.Filelistwidget = f.widget
	i.Filelist = f
	i.Filelist.WSitem = i
	i.active = true
}

func newFilelistwidget(path string) *Filelist {
	fileitems := []*Fileitem{}
	lsfiles, _ := ioutil.ReadDir(path)

	filelist := &Filelist{}
	filelist.active = -1

	filelistwidget := widgets.NewQWidget(nil, 0)
	filelistlayout := widgets.NewQBoxLayout(widgets.QBoxLayout__TopToBottom, filelistwidget)
	filelistlayout.SetContentsMargins(0, 0, 0, 100)
	filelistlayout.SetSpacing(1)
	bg := editor.bgcolor
	width := editor.workspacewidth
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

func (w *Workspace) attachUIOption() map[string]interface{} {
	o := make(map[string]interface{})
	o["rgb"] = true

	apiInfo, err := w.nvim.APIInfo()
	if err == nil {
		for _, item := range apiInfo {
			i, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			for k, v := range i {
				if k != "ui_events" {
					continue
				}
				events, ok := v.([]interface{})
				if !ok {
					continue
				}
				for _, event := range events {
					function, ok := event.(map[string]interface{})
					if !ok {
						continue
					}
					name, ok := function["name"]
					if !ok {
						continue
					}
					if name == "wildmenu_show" {
						o["ext_wildmenu"] = true
					} else if name == "cmdline_show" {
						o["ext_cmdline"] = true
					} else if name == "msg_chunk" {
						o["ext_messages"] = true
					} else if name == "popupmenu_show" {
						o["ext_popupmenu"] = true
					} else if name == "tabline_update" {
						o["ext_tabline"] = w.drawTabline
					}
				}
			}
		}
	}
	return o
}

func (w *Workspace) updateSize() {
	e := editor
	width := e.wsWidget.Width()
	height := e.wsWidget.Height()
	if width != w.width || height != w.height {
		w.width = width
		w.height = height
		w.widget.Resize2(width, height)
		if !w.hidden {
			w.hide()
			w.show()
		} else {
			w.show()
			w.hide()
		}
	}

	if w.drawTabline {
		w.tabline.height = w.tabline.widget.Height()
	}
	if w.drawStatusline {
		w.statusline.height = w.statusline.widget.Height()
	}

	height = w.height - w.tabline.height - w.statusline.height

	rows := height / w.font.lineHeight
	remainingHeight := height - rows*w.font.lineHeight
	remainingHeightBottom := remainingHeight / 2
	remainingHeightTop := remainingHeight - remainingHeightBottom
	w.tabline.marginTop = w.tabline.marginDefault + remainingHeightTop
	w.tabline.marginBottom = w.tabline.marginDefault + remainingHeightBottom
	w.tabline.updateMargin()
	w.screen.height = height - remainingHeight

	w.screen.updateSize()
	w.palette.resize()
	w.message.resize()
}

func (w *Workspace) handleRedraw(updates [][]interface{}) {
	s := w.screen
	for _, update := range updates {
		event := update[0].(string)
		args := update[1:]
		switch event {
		case "update_fg":
			args := update[1].([]interface{})
			color := reflectToInt(args[0])
			if color == -1 {
				w.foreground = newRGBA(255, 255, 255, 1)
			} else {
				w.foreground = calcColor(reflectToInt(args[0]))
			}
			if w.setGuiFgColor == false {
				if editor.wsSide.fgcolor == nil {
					editor.wsSide.fgcolor = editor.workspaces[0].foreground
				}
				if editor.fgcolor == nil {
					editor.fgcolor = editor.workspaces[0].foreground
				}
				w.setGuiColor()
				w.setGuiFgColor = true
			}
		case "update_bg":
			args := update[1].([]interface{})
			s.updateBg(args)
			if w.setGuiBgColor == false {
				go w.nvim.Command(`call rpcnotify(0, "statusline", "bufenter", expand("%:p"), &filetype, &fileencoding)`)
				go w.nvim.Command(`call rpcnotify(0, "statusline", "cursormoved", getpos("."))`)

				if editor.wsSide.bgcolor == nil {
					editor.wsSide.bgcolor = editor.workspaces[0].background
				}
				if editor.bgcolor == nil {
					editor.bgcolor = editor.workspaces[0].background
				}
				w.setGuiColor()
				w.setGuiBgColor = true
			}
		case "update_sp":
			args := update[1].([]interface{})
			color := reflectToInt(args[0])
			if color == -1 {
				w.special = newRGBA(255, 255, 255, 1)
			} else {
				w.special = calcColor(reflectToInt(args[0]))
			}
		case "cursor_goto":
			s.cursorGoto(args)
		case "put":
			s.put(args)
		case "eol_clear":
			s.eolClear(args)
		case "clear":
			s.clear(args)
		case "resize":
			s.resize(args)
		case "highlight_set":
			s.highlightSet(args)
		case "set_scroll_region":
			s.setScrollRegion(args)
		case "scroll":
			s.scroll(args)
		case "mode_change":
			arg := update[len(update)-1].([]interface{})
			w.mode = arg[0].(string)
		case "popupmenu_show":
			w.popup.showItems(args)
		case "popupmenu_hide":
			w.popup.hide()
		case "popupmenu_select":
			w.popup.selectItem(args)
		case "tabline_update":
			w.tabline.update(args)
		case "cmdline_show":
			w.cmdline.show(args)
		case "cmdline_pos":
			w.cmdline.changePos(args)
		case "cmdline_char":
			w.cmdline.putChar(args)
		case "cmdline_hide":
			w.cmdline.hide(args)
		case "cmdline_function_show":
			w.cmdline.functionShow()
		case "cmdline_function_hide":
			w.cmdline.functionHide()
		case "wildmenu_show":
			w.cmdline.wildmenuShow(args)
		case "wildmenu_select":
			w.cmdline.wildmenuSelect(args)
		case "wildmenu_hide":
			w.cmdline.wildmenuHide()
		case "msg_start_kind":
			if len(args) > 0 {
				kinds, ok := args[len(args)-1].([]interface{})
				if ok {
					if len(kinds) > 0 {
						kind, ok := kinds[len(kinds)-1].(string)
						if ok {
							w.message.kind = kind
						}
					}
				}
			}
		case "msg_chunk":
			w.message.chunk(args)
		case "msg_end":
		case "msg_showcmd":
		case "messages":
		case "busy_start":
		case "busy_stop":
		default:
			fmt.Println("Unhandle event", event)
		}
	}
	s.update()
	w.cursor.update()
	w.statusline.mode.redraw()
}

func (w *Workspace) handleRPCGui(updates []interface{}) {
	event := updates[0].(string)
	switch event {
	case "Font":
		w.guiFont(updates[1:])
	case "Linespace":
		w.guiLinespace(updates[1:])
	case "finder_pattern":
		w.finder.showPattern(updates[1:])
	case "finder_pattern_pos":
		w.finder.cursorPos(updates[1:])
	case "finder_show_result":
		w.finder.showResult(updates[1:])
	case "finder_hide":
		w.finder.hide()
	case "finder_select":
		w.finder.selectResult(updates[1:])
	case "signature_show":
		w.signature.showItem(updates[1:])
	case "signature_pos":
		w.signature.pos(updates[1:])
	case "signature_hide":
		w.signature.hide()
	case "gonvim_workspace_new":
		editor.workspaceNew()
	case "gonvim_workspace_next":
		editor.workspaceNext()
	case "gonvim_workspace_previous":
		editor.workspacePrevious()
	case "gonvim_workspace_switch":
		editor.workspaceSwitch(reflectToInt(updates[1]))
	case "gonvim_workspace_cwd":
		w.setCwd()
	case "gonvim_workspace_redrawSideItem":
		fl := editor.wsSide.items[editor.active].Filelist
		if fl.active != -1 {
			if editor.showWorkspaceside == true || (editor.showWorkspaceside == false && len(editor.workspaces) > 1) {
				fl.Fileitems[fl.active].updateModifiedbadge()
			}
		}
	case "gonvim_workspace_redrawSideItems":
		editor.wsSide.items[editor.active].setCurrentFileLabel()
	case GonvimMarkdownNewBufferEvent:
		go w.markdown.newBuffer()
	case GonvimMarkdownUpdateEvent:
		go w.markdown.update()
	case GonvimMarkdownToggleEvent:
		go w.markdown.toggle()
	case GonvimMarkdownScrollDownEvent:
		w.markdown.scrollDown()
	case GonvimMarkdownScrollUpEvent:
		w.markdown.scrollUp()
	default:
		fmt.Println("unhandled Gui event", event)
	}
}

func (w *Workspace) guiFont(args ...interface{}) {
	fontArg := args[0].([]interface{})
	parts := strings.Split(fontArg[0].(string), ":")
	if len(parts) < 1 {
		return
	}

	height := 14
	for _, p := range parts[1:] {
		if strings.HasPrefix(p, "h") {
			var err error
			height, err = strconv.Atoi(p[1:])
			if err != nil {
				return
			}
		}
	}

	w.font.change(parts[0], height)
	w.updateSize()
	w.popup.updateFont(w.font)
	w.screen.toolTipFont(w.font)
}

func (w *Workspace) guiLinespace(args ...interface{}) {
	fontArg := args[0].([]interface{})
	var lineSpace int
	var err error
	switch arg := fontArg[0].(type) {
	case string:
		lineSpace, err = strconv.Atoi(arg)
		if err != nil {
			return
		}
	case int32: // can't combine these in a type switch without compile error
		lineSpace = int(arg)
	case int64:
		lineSpace = int(arg)
	default:
		return
	}
	w.font.changeLineSpace(lineSpace)
	w.updateSize()
}

// InputMethodEvent is
func (w *Workspace) InputMethodEvent(event *gui.QInputMethodEvent) {
	if event.CommitString() != "" {
		w.nvim.Input(event.CommitString())
		w.screen.tooltip.Hide()
	} else {
		preeditString := event.PreeditString()
		if preeditString == "" {
			w.screen.tooltip.Hide()
			w.cursor.update()
		} else {
			w.screen.toolTip(preeditString)
		}
	}
}

// InputMethodQuery is
func (w *Workspace) InputMethodQuery(query core.Qt__InputMethodQuery) *core.QVariant {
	qv := core.NewQVariant()
	if query == core.Qt__ImCursorRectangle {
		imrect := core.NewQRect()
		row := w.screen.cursor[0]
		col := w.screen.cursor[1]
		x := int(float64(col)*w.font.truewidth) - 1
		y := row*w.font.lineHeight + w.tabline.height + w.tabline.marginTop + w.tabline.marginBottom
		imrect.SetRect(x, y, 1, w.font.lineHeight)
		return core.NewQVariant33(imrect)
	}
	return qv
}

// WorkspaceSide is
type WorkspaceSide struct {
	widget     *widgets.QWidget
	scrollarea *widgets.QScrollArea
	title      *widgets.QLabel
	items      []*WorkspaceSideItem
	fgcolor    *RGBA
	bgcolor    *RGBA
}

type Filelist struct {
	WSitem    *WorkspaceSideItem
	widget    *widgets.QWidget
	Fileitems []*Fileitem
	isload    bool
	active    int
}

type Fileitem struct {
	fl     *Filelist
	widget *widgets.QWidget
	//ID        int
	//Name      string
	//width     int
	//chars     int
	fileIcon *svg.QSvgWidget
	fileType string
	//closeIcon *svg.QSvgWidget
	file     *widgets.QLabel
	fileText string
	//hidden    bool
	path         string
	fileModified *svg.QSvgWidget
	isOpened     bool
}

func newWorkspaceSide() *WorkspaceSide {
	layout := newHFlowLayout(0, 0, 0, 0, 20)
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetSpacing(0)
	header := widgets.NewQLabel(nil, 0)
	header.SetContentsMargins(20, 15, 20, 10)
	header.SetText("Workspace")
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetLayout(layout)

	side := &WorkspaceSide{
		widget: widget,
		title:  header,
	}
	layout.AddWidget(header)

	items := []*WorkspaceSideItem{}
	side.items = items
	for i := 0; i < 20; i++ {
		item := newWorkspaceSideItem()
		side.items = append(side.items, item)
		side.items[len(side.items)-1].side = side
		layout.AddWidget(side.items[len(side.items)-1].widget)
		side.items[len(side.items)-1].hide()
	}

	//footer := widgets.NewQLabel(nil, 0)
	//footer.SetContentsMargins(20, 15, 20, 10)
	//footer.SetText("WorkspaceFooter")
	//layout.AddWidget(footer)

	return side
}

// WorkspaceSideItem is
type WorkspaceSideItem struct {
	hidden bool
	active bool
	side   *WorkspaceSide

	widget *widgets.QWidget
	layout *widgets.QBoxLayout
	//layout    *widgets.QLayout

	text    string
	cwdpath string

	Filelist       *Filelist
	label          *widgets.QLabel
	Filelistwidget *widgets.QWidget
}

func newWorkspaceSideItem() *WorkspaceSideItem {
	widget := widgets.NewQWidget(nil, 0)

	//layout := widgets.NewQVBoxLayout()
	layout := widgets.NewQBoxLayout(widgets.QBoxLayout__TopToBottom, widget)
	layout.SetContentsMargins(0, 5, 0, 5)

	//items := []*widgets.QLayoutItem{}

	//layout.ConnectSizeHint(func() *core.QSize {
	//	size := core.NewQSize()
	//	for _, item := range items {
	//		size = size.ExpandedTo(item.MinimumSize())
	//	}
	//	return size
	//})
	//layout.ConnectAddItem(func(item *widgets.QLayoutItem) {
	//	items = append(items, item)
	//})
	//layout.ConnectSetGeometry(func(r *core.QRect) {
	//	for i := 0; i < len(items); i++ {
	//		items[i].SetGeometry(core.NewQRect4(width*i, 0, width, r.Height()))
	//	}
	//})

	label := widgets.NewQLabel(nil, 0)
	label.SetContentsMargins(15, 6, 10, 6)
	label.SetMaximumWidth(editor.workspacewidth)
	label.SetMinimumWidth(editor.workspacewidth)

	flwidget := widgets.NewQWidget(nil, 0)

	filelist := &Filelist{
		widget: flwidget,
	}

	layout.AddWidget(label, 0, 0)
	layout.AddWidget(flwidget, 0, 0)
	//sideitem.Filelist.widget.Hide()

	sideitem := &WorkspaceSideItem{
		widget:         widget,
		layout:         layout,
		label:          label,
		Filelist:       filelist,
		Filelistwidget: flwidget,
	}
	sideitem.Filelist.WSitem = sideitem

	return sideitem
}

func (i *WorkspaceSideItem) setText(text string) {
	if i.text == text {
		return
	}
	i.text = text
	i.label.SetText(text)
	i.widget.Show()
}

func (i *WorkspaceSideItem) setActive() {
	if i.active {
		return
	}
	i.active = true
	if i.side.fgcolor == nil {
		return
	}
	bg := i.side.bgcolor
	fg := i.side.fgcolor
	i.label.SetStyleSheet(fmt.Sprintf("margin: 0px 10px 0px 10px; border-left: 5px solid rgba(81, 154, 186, 1);	background-color: rgba(%d, %d, %d, 1);	color: rgba(%d, %d, %d, 1);	", shiftColor(bg, 5).R, shiftColor(bg, 5).G, shiftColor(bg, 5).B, shiftColor(fg, 0).R, shiftColor(fg, 0).G, shiftColor(fg, 0).B))

	if i.Filelist.isload == false && editor.showWorkspaceside == false && len(editor.workspaces) > 1 {
		filelist := newFilelistwidget(i.cwdpath)
		i.setFilelistwidget(filelist)
	}

	i.Filelistwidget.Show()
}

func (i *WorkspaceSideItem) setInactive() {
	if !i.active {
		return
	}
	i.active = false
	if i.side.fgcolor == nil {
		return
	}
	bg := i.side.bgcolor
	fg := i.side.fgcolor
	i.label.SetStyleSheet(fmt.Sprintf("margin: 0px 10px 0px 15px; background-color: rgba(%d, %d, %d, 1);	color: rgba(%d, %d, %d, 1);	", shiftColor(bg, -5).R, shiftColor(bg, -5).G, shiftColor(bg, -5).B, shiftColor(fg, 0).R, shiftColor(fg, 0).G, shiftColor(fg, 0).B))

	i.Filelistwidget.Hide()
}

func (i *WorkspaceSideItem) show() {
	if !i.hidden {
		return
	}
	i.hidden = false
	i.label.Show()
	i.Filelistwidget.Show()
}

func (i *WorkspaceSideItem) hide() {
	if i.hidden {
		return
	}
	i.hidden = true
	i.label.Hide()
	i.Filelistwidget.Hide()
}

func (w *Workspace) setGuiColor() {
	if editor.fgcolor == nil || editor.bgcolor == nil {
		return
	}
	if w.setGuiFgColor == true && w.setGuiBgColor == true {
		return
	}

	fg := editor.fgcolor
	bg := editor.bgcolor

	// tab
	tabStyle := fmt.Sprintf("QWidget { color: rgba(%d, %d, %d, 0.8);	}", gradColor(fg).R, gradColor(fg).G, gradColor(fg).B)
	w.tabline.widget.SetStyleSheet(fmt.Sprintf(".QWidget {	border-left: 8px solid rgba(%d, %d, %d, 1); border-bottom: 0px solid;	border-right: 0px solid;	background-color: rgba(%d, %d, %d, 1);	}	", shiftColor(bg, 10).R, shiftColor(bg, 10).G, shiftColor(bg, 10).B, shiftColor(bg, 10).R, shiftColor(bg, 10).G, shiftColor(bg, 10).B) + tabStyle)

	// statusline
	statusStyle := fmt.Sprintf("	* {	color: rgba(%d, %d, %d, 1);	}", warpColor(fg, 10).R, warpColor(fg, 10).G, warpColor(fg, 10).B)
	w.statusline.file.folderLabel.SetStyleSheet(fmt.Sprintf("color: rgba(%d, %d, %d, 0.8);", gradColor(fg).R, gradColor(fg).G, gradColor(fg).B))
	w.statusline.widget.SetStyleSheet(fmt.Sprintf("QWidget#statusline {	border-top: 0px solid rgba(%d, %d, %d, 1);	background-color: rgba(%d, %d, %d, 1);	}", shiftColor(bg, 0).R, shiftColor(bg, 0).G, shiftColor(bg, 0).B, shiftColor(bg, 0).R, shiftColor(bg, 0).G, shiftColor(bg, 0).B) + statusStyle)
	//w.statusline.widget.SetStyleSheet(fmt.Sprintf("QWidget#statusline {	border-top: 0px solid rgba(%d, %d, %d, 1);	background-color: rgba(%d, %d, %d, 1);	}", shiftColor(bg, 10).R, shiftColor(bg, 10).G, shiftColor(bg, 10).B, shiftColor(bg, 10).R, shiftColor(bg, 10).G, shiftColor(bg, 10).B) + statusStyle)
	svgContent := w.getSvg("git", newRGBA(warpColor(fg, 10).R, warpColor(fg, 10).G, warpColor(fg, 10).B, 1))
	w.statusline.git.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))

	// for Gonvim UI Color form colorscheme
	tooltipStyle := fmt.Sprintf("color: rgba(%d, %d, %d, 1); }", shiftColor(fg, -40).R, shiftColor(fg, -40).G, shiftColor(fg, -40).B)

	paletteStyle := fmt.Sprintf(" * { color: rgba(%d, %d, %d, 1); }", shiftColor(fg, -30).R, shiftColor(fg, -30).G, shiftColor(fg, -30).B)
	w.palette.cursor.SetStyleSheet(fmt.Sprintf("background-color: rgba(%d, %d, %d, 1);", shiftColor(fg, -30).R, shiftColor(fg, -30).G, shiftColor(fg, -30).B))
	w.palette.widget.SetStyleSheet(fmt.Sprintf("QWidget#palette {border: 1px solid rgba(%d, %d, %d, 1);} .QWidget {background-color: rgba(%d, %d, %d, 1); }", shiftColor(bg, 25).R, shiftColor(bg, 25).G, shiftColor(bg, 25).B, shiftColor(bg, 15).R, shiftColor(bg, 15).G, shiftColor(bg, 15).B) + paletteStyle)
	w.palette.scrollBar.SetStyleSheet(fmt.Sprintf("background-color: rgba(%d, %d, %d, 1);", shiftColor(bg, -15).R, shiftColor(bg, -15).G, shiftColor(bg, -15).B))
	w.palette.pattern.SetStyleSheet(fmt.Sprintf("background-color: rgba(%d, %d, %d, 1);", shiftColor(bg, -15).R, shiftColor(bg, -15).G, shiftColor(bg, -15).B))

	// popup
	popupStyle := fmt.Sprintf("color: rgba(%d, %d, %d, 1);} #detailpopup {	color: rgba(%d, %d, %d, 1); }", shiftColor(fg, 5).R, shiftColor(fg, 5).G, shiftColor(fg, 5).B, gradColor(fg).R, gradColor(fg).G, gradColor(fg).B)
	w.popup.scrollBar.SetStyleSheet(fmt.Sprintf("background-color: rgba(%d, %d, %d, 1);", gradColor(bg).R, gradColor(bg).G, gradColor(bg).B))
	w.popup.widget.SetStyleSheet(fmt.Sprintf("* {background-color: rgba(%d, %d, %d, 1); ", shiftColor(bg, 15).R, shiftColor(bg, 15).G, shiftColor(bg, 15).B) + popupStyle)

	// loc
	locpopupStyle := fmt.Sprintf(" color: rgba(%d, %d, %d, 1); }", shiftColor(fg, 5).R, shiftColor(fg, 5).G, shiftColor(fg, 5).B)
	w.loc.widget.SetStyleSheet(fmt.Sprintf(".QWidget { border: 1px solid rgba(%d, %d, %d, 1); } * {background-color: rgba(%d, %d, %d, 1);", shiftColor(bg, 20).R, shiftColor(bg, 20).G, shiftColor(bg, 20).B, shiftColor(bg, 10).R, shiftColor(bg, 10).G, shiftColor(bg, 10).B) + locpopupStyle)

	// screan tooltip
	w.screen.tooltip.SetStyleSheet(fmt.Sprintf(" * {background-color: rgba(%d, %d, %d, 1); text-decoration: underline;", gradColor(bg).R, gradColor(bg).G, gradColor(bg).B) + tooltipStyle)

	// for Workspaceside
	wsSideStyle := fmt.Sprintf("QWidget {	color: rgba(%d, %d, %d, 1);		border-right: 0px solid;	}", gradColor(fg).R, gradColor(fg).G, gradColor(fg).B)
	editor.wsSide.widget.SetStyleSheet(fmt.Sprintf(".QWidget {	border-color: rgba(%d, %d, %d, 1); padding-top: 5px;	background-color: rgba(%d, %d, %d, 1);	}	", shiftColor(bg, 10).R, shiftColor(bg, 10).G, shiftColor(bg, 10).B, shiftColor(bg, -5).R, shiftColor(bg, -5).G, shiftColor(bg, -5).B) + wsSideStyle)
	editor.wsSide.scrollarea.SetStyleSheet(fmt.Sprintf(".QScrollBar { border-width: 0px; background-color: rgb(%d, %d, %d); width: 5px; margin: 0 0 0 0; } .QScrollBar::handle:vertical {background-color: rgb(%d, %d, %d); min-height: 25px;}  QScrollBar::add-line:vertical, QScrollBar::sub-line:vertical { border: none; background: none; }", shiftColor(bg, -5).R, shiftColor(bg, -5).G, shiftColor(bg, -5).B, gradColor(bg).R, gradColor(bg).G, gradColor(bg).B))

	// for WorkspaceSideItem
	if len(editor.workspaces) == 1 || len(editor.wsSide.items) == 1 {
		if editor.showWorkspaceside == true {
			editor.wsSide.items[0].label.SetStyleSheet(fmt.Sprintf("margin: 0px 10px 0px 10px; border-left: 5px solid rgba(81, 154, 186, 1);	background-color: rgba(%d, %d, %d, 1);	color: rgba(%d, %d, %d, 1);	", shiftColor(bg, 5).R, shiftColor(bg, 5).G, shiftColor(bg, 5).B, shiftColor(fg, 0).R, shiftColor(fg, 0).G, shiftColor(fg, 0).B))
			//// scrollarea's setWidget is brokean some magins
			editor.wsSide.items[0].label.SetContentsMargins(15+15, 6, 0, 6)
		}
	}

}

func (f *Fileitem) enterEvent(event *core.QEvent) {
	bg := editor.bgcolor
	var svgModified string
	cfn := ""
	editor.workspaces[editor.active].nvim.Eval("expand('%:t')", &cfn)
	if cfn == f.fileText {
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
	if cfn != f.fileText {
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
		if fileitem.fileText != cfn {
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

package gonvim

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/dzhou121/gonvim-fuzzy/rplugin/go/fzf"
	"github.com/neovim/go-client/nvim"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/widgets"
)

var editor *Editor

// Highlight is
type Highlight struct {
	foreground *RGBA
	background *RGBA
}

// Char is
type Char struct {
	normalWidth bool
	char        string
	highlight   Highlight
}

// Editor is the editor
type Editor struct {
	app              *widgets.QApplication
	nvim             *nvim.Nvim
	nvimAttached     bool
	mode             string
	font             *Font
	smallerFont      *Font
	rows             int
	cols             int
	cursorNew        *Cursor
	Foreground       *RGBA
	Background       *RGBA
	special          *RGBA
	screen           *Screen
	close            chan bool
	loc              *Locpopup
	signature        *Signature
	popup            *PopupMenu
	finder           *Finder
	cmdline          *Cmdline
	palette          *Palette
	tabline          *Tabline
	statusline       *Statusline
	drawStatusline   bool
	drawLint         bool
	statuslineHeight int
	width            int
	height           int
	tablineHeight    int
	selectedBg       *RGBA
	matchFg          *RGBA
	resizeMutex      sync.Mutex
	signal           *editorSignal
	redrawUpdates    chan [][]interface{}
	guiUpdates       chan []interface{}
}

type editorSignal struct {
	core.QObject
	_ func() `signal:"redrawSignal"`
	_ func() `signal:"guiSignal"`
	_ func() `signal:"statuslineSignal"`
	_ func() `signal:"locpopupSignal"`
	_ func() `signal:"lintSignal"`
	_ func() `signal:"gitSignal"`
}

func (e *Editor) handleNotification() {
	e.nvim.RegisterHandler("Gui", func(updates ...interface{}) {
		e.guiUpdates <- updates
		e.signal.GuiSignal()
	})
	e.nvim.RegisterHandler("redraw", func(updates ...[]interface{}) {
		e.redrawUpdates <- updates
		e.signal.RedrawSignal()
	})
}

func (e *Editor) handleRPCGui(updates []interface{}) {
	event := updates[0].(string)
	switch event {
	case "Font":
		e.guiFont(updates[1:])
	case "Linespace":
		e.guiLinespace(updates[1:])
	case "finder_pattern":
		e.finder.showPattern(updates[1:])
	case "finder_pattern_pos":
		e.finder.cursorPos(updates[1:])
	case "finder_show_result":
		e.finder.showResult(updates[1:])
	case "finder_hide":
		e.finder.hide()
	case "finder_select":
		e.finder.selectResult(updates[1:])
	case "signature_show":
		e.signature.showItem(updates[1:])
	case "signature_pos":
		e.signature.pos(updates[1:])
	case "signature_hide":
		e.signature.hide()
	default:
		fmt.Println("unhandled Gui event", event)
	}
}

func (e *Editor) handleRedraw(updates [][]interface{}) {
	s := e.screen
	for _, update := range updates {
		event := update[0].(string)
		args := update[1:]
		switch event {
		case "update_fg":
			args := update[1].([]interface{})
			color := reflectToInt(args[0])
			if color == -1 {
				editor.Foreground = newRGBA(255, 255, 255, 1)
			} else {
				editor.Foreground = calcColor(reflectToInt(args[0]))
			}
		case "update_bg":
			args := update[1].([]interface{})
			s.updateBg(args)
		case "update_sp":
			args := update[1].([]interface{})
			color := reflectToInt(args[0])
			if color == -1 {
				editor.special = newRGBA(255, 255, 255, 1)
			} else {
				editor.special = calcColor(reflectToInt(args[0]))
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
			editor.mode = arg[0].(string)
		case "popupmenu_show":
			editor.popup.showItems(args)
		case "popupmenu_hide":
			editor.popup.hide()
		case "popupmenu_select":
			editor.popup.selectItem(args)
		case "tabline_update":
			editor.tabline.update(args)
		case "cmdline_show":
			editor.cmdline.show(args)
		case "cmdline_pos":
			editor.cmdline.changePos(args)
		case "cmdline_char":
			editor.cmdline.putChar(args)
		case "cmdline_hide":
			editor.cmdline.hide(args)
		case "cmdline_function_show":
			editor.cmdline.functionShow()
		case "cmdline_function_hide":
			editor.cmdline.functionHide()
		case "wildmenu_show":
			editor.cmdline.wildmenuShow(args)
		case "wildmenu_select":
			editor.cmdline.wildmenuSelect(args)
		case "wildmenu_hide":
			editor.cmdline.wildmenuHide()
		case "busy_start":
		case "busy_stop":
		default:
			fmt.Println("Unhandle event", event)
		}
	}
	s.update()
	editor.cursorNew.update()
	editor.statusline.mode.redraw()
}

func (e *Editor) guiFont(args ...interface{}) {
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

	e.font.change(parts[0], height)
	e.nvimResize()
	e.popup.updateFont(e.font)
}

func (e *Editor) guiLinespace(args ...interface{}) {
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
	e.font.changeLineSpace(lineSpace)
	e.nvimResize()
}

func (e *Editor) nvimResize() {
	// e.screen.paintMutex.Lock()
	// defer e.screen.paintMutex.Unlock()
	width, height := e.screen.width, e.screen.height
	cols := int(float64(width) / editor.font.truewidth)
	rows := height / editor.font.lineHeight
	oldCols := editor.cols
	oldRows := editor.rows
	editor.cols = cols
	editor.rows = rows
	if oldCols > 0 && oldRows > 0 {
		if cols != oldCols || rows != oldRows {
			editor.nvim.TryResizeUI(cols, rows)
		}
	}
}

func (hl *Highlight) copy() Highlight {
	highlight := Highlight{}
	if hl.foreground != nil {
		highlight.foreground = hl.foreground.copy()
	}
	if hl.background != nil {
		highlight.background = hl.background.copy()
	}
	return highlight
}

func (e *Editor) configure() {
	var drawSplit interface{}
	e.nvim.Var("gonvim_draw_split", &drawSplit)
	if isZero(drawSplit) {
		e.screen.drawSplit = false
	} else {
		e.screen.drawSplit = true
	}

	var drawStatusline interface{}
	e.nvim.Var("gonvim_draw_statusline", &drawStatusline)
	if isZero(drawStatusline) {
		e.drawStatusline = false
	} else {
		e.drawStatusline = true
	}

	var drawLint interface{}
	e.nvim.Var("gonvim_draw_lint", &drawLint)
	if isZero(drawLint) {
		e.drawLint = false
	} else {
		e.drawLint = true
	}
}

// InitEditorNew is
func InitEditorNew() {
	app := widgets.NewQApplication(0, nil)
	devicePixelRatio := app.DevicePixelRatio()
	fontFamily := ""
	switch runtime.GOOS {
	case "windows":
		fontFamily = "Consolas"
	case "darwin":
		fontFamily = "Courier New"
	default:
		fontFamily = "Monospace"
	}
	font := initFontNew(fontFamily, 14, 6)

	width := 800
	height := 600

	//create a window
	window := widgets.NewQMainWindow(nil, 0)
	window.SetWindowTitle("Gonvim")
	window.SetContentsMargins(0, 0, 0, 0)
	window.SetMinimumSize2(width, height)

	tabline := initTablineNew()
	statusline := initStatuslineNew()
	screen := initScreenNew(devicePixelRatio)
	cursor := initCursorNew()
	cursor.widget.SetParent(screen.widget)
	popup := initPopupmenuNew(font)
	popup.widget.SetParent(screen.widget)
	finder := initFinder()
	palette := initPalette()
	palette.widget.SetParent(screen.widget)
	loc := initLocpopup()
	loc.widget.SetParent(screen.widget)
	signature := initSignature()
	signature.widget.SetParent(screen.widget)
	window.ConnectKeyPressEvent(screen.keyPress)

	layout := widgets.NewQVBoxLayout()
	widget := widgets.NewQWidget(nil, 0)
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetLayout(layout)
	layout.AddWidget(tabline.widget, 0, 0)
	layout.AddWidget(screen.widget, 1, 0)
	layout.AddWidget(statusline.widget, 0, 0)
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetSpacing(0)

	window.SetCentralWidget(widget)

	neovim, err := nvim.NewEmbedded(&nvim.EmbedOptions{
		Args: os.Args[1:],
		Path: "/Users/Lulu/neovim/build/bin/nvim",
	})
	if err != nil {
		fmt.Println("nvim start error", err)
		app.Quit()
		return
	}

	signal := NewEditorSignal(nil)

	editor = &Editor{
		app:           app,
		nvim:          neovim,
		nvimAttached:  false,
		screen:        screen,
		cursorNew:     cursor,
		mode:          "normal",
		close:         make(chan bool),
		popup:         popup,
		finder:        finder,
		cmdline:       initCmdline(),
		palette:       palette,
		loc:           loc,
		signature:     signature,
		tabline:       tabline,
		width:         width,
		height:        height,
		statusline:    statusline,
		font:          font,
		selectedBg:    newRGBA(81, 154, 186, 0.5),
		matchFg:       newRGBA(81, 154, 186, 1),
		signal:        signal,
		redrawUpdates: make(chan [][]interface{}, 1000),
		guiUpdates:    make(chan []interface{}, 1000),
	}

	signal.ConnectRedrawSignal(func() {
		updates := <-editor.redrawUpdates
		editor.handleRedraw(updates)
	})

	signal.ConnectGuiSignal(func() {
		updates := <-editor.guiUpdates
		editor.handleRPCGui(updates)
	})

	editor.handleNotification()
	// editor.finder.rePosition()
	go func() {
		err := neovim.Serve()
		if err != nil {
			fmt.Println(err)
		}
		editor.close <- true
	}()

	screen.updateSize()

	o := make(map[string]interface{})
	o["rgb"] = true
	o["ext_popupmenu"] = true
	o["ext_tabline"] = true
	o["ext_cmdline"] = true
	o["ext_wildmenu"] = true
	err = editor.nvim.AttachUI(editor.cols, editor.rows, o)
	if err != nil {
		fmt.Println("nvim attach UI error", err)
		app.Quit()
		return
	}
	editor.nvim.Subscribe("Gui")
	editor.nvim.Command("runtime plugin/nvim_gui_shim.vim")
	editor.nvim.Command("runtime! ginit.vim")
	editor.nvim.Command("let g:gonvim_running=1")
	editor.configure()
	fzf.RegisterPlugin(editor.nvim)
	statusline.subscribe()
	loc.subscribe()

	go func() {
		<-editor.close
		app.Quit()
	}()

	window.Show()
	popup.widget.Hide()
	palette.widget.Hide()
	loc.widget.Hide()
	signature.widget.Hide()
	widgets.QApplication_Exec()
}

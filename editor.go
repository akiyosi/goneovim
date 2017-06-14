package gonvim

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/dzhou121/neovim-fzf-shim/rplugin/go/fzf"
	"github.com/neovim/go-client/nvim"
	"github.com/therecipe/qt/gui"
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
	char      string
	highlight Highlight
}

// Editor is the editor
type Editor struct {
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
	tabline          *Tabline
	statusline       *Statusline
	statuslineHeight int
	width            int
	height           int
	tablineHeight    int
	selectedBg       *RGBA
	matchFg          *RGBA
	resizeMutex      sync.Mutex
}

func (e *Editor) handleNotification() {
	e.nvim.RegisterHandler("Gui", func(updates ...interface{}) {
		go e.handleRPCGui(updates...)
	})
	e.nvim.RegisterHandler("redraw", func(updates ...[]interface{}) {
		// e.screen.paintMutex.Lock()
		e.handleRedraw(updates...)
		// e.screen.paintMutex.Unlock()
	})
}

func (e *Editor) handleRPCGui(updates ...interface{}) {
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
		e.signature.show(updates[1:])
	case "signature_pos":
		e.signature.pos(updates[1:])
	case "signature_hide":
		e.signature.hide()
	default:
		fmt.Println("unhandled Gui event", event)
	}
}

func (e *Editor) handleRedraw(updates ...[]interface{}) {
	screen := e.screen
	// go screen.redrawWindows()
	screen.pixmapPainter = gui.NewQPainter2(screen.pixmap)
	screen.pixmapPainter.SetFont(e.font.fontNew)
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
			screen.updateBg(args)
		case "update_sp":
			args := update[1].([]interface{})
			color := reflectToInt(args[0])
			if color == -1 {
				editor.special = newRGBA(255, 255, 255, 1)
			} else {
				editor.special = calcColor(reflectToInt(args[0]))
			}
		case "cursor_goto":
			screen.cursorGoto(args)
		case "put":
			screen.put(args)
		case "eol_clear":
			screen.eolClear(args)
		case "clear":
			screen.clear(args)
		case "resize":
			screen.resize(args)
		case "highlight_set":
			screen.highlightSet(args)
		case "set_scroll_region":
			screen.setScrollRegion(args)
		case "scroll":
			screen.scroll(args)
		case "mode_change":
			arg := update[len(update)-1].([]interface{})
			editor.mode = arg[0].(string)
		case "popupmenu_show":
			editor.popup.show(args)
		case "popupmenu_hide":
			editor.popup.hide(args)
		case "popupmenu_select":
			editor.popup.selectItem(args)
		case "tabline_update":
			editor.tabline.update(args)
		case "busy_start":
		case "busy_stop":
		default:
			fmt.Println("Unhandle event", event)
		}
	}
	for _, win := range screen.curWins {
		win.drawBorder(screen.pixmapPainter)
	}
	screen.pixmapPainter.DestroyQPainter()
	screen.redraw()
	// editor.cursor.draw()
	editor.cursorNew.update()
	if !e.nvimAttached {
		e.nvimAttached = true
	}
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
	width, height := e.screen.size()
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
	finder.widget.SetParent(screen.widget)
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
	})
	if err != nil {
		fmt.Println("nvim start error", err)
		app.Quit()
		return
	}

	editor = &Editor{
		nvim:         neovim,
		nvimAttached: false,
		screen:       screen,
		cursorNew:    cursor,
		mode:         "normal",
		close:        make(chan bool),
		popup:        popup,
		finder:       finder,
		loc:          loc,
		signature:    signature,
		tabline:      tabline,
		width:        width,
		height:       height,
		statusline:   statusline,
		font:         font,
		selectedBg:   newRGBA(81, 154, 186, 0.5),
		matchFg:      newRGBA(81, 154, 186, 1),
	}

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
	fzf.RegisterPlugin(editor.nvim)
	screen.subscribe()
	statusline.subscribe()
	loc.subscribe()

	go func() {
		<-editor.close
		app.Quit()
	}()

	window.Show()
	widgets.QApplication_Exec()
}

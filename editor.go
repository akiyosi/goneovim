package gonvim

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/dzhou121/neovim-fzf-shim/rplugin/go/fzf"
	"github.com/dzhou121/neovim-locpopup/rplugin/go/locpopup"
	"github.com/dzhou121/ui"
	"github.com/neovim/go-client/nvim"
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
	cursor           *CursorBox
	Foreground       *RGBA
	Background       *RGBA
	window           *ui.Window
	screen           *Screen
	areaBox          *ui.Box
	close            chan bool
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
	redrawMutex      sync.Mutex
}

func initMainWindow(box *ui.Box, width, height int) *ui.Window {
	window := ui.NewWindow("Gonvim", width, height, false)
	window.SetChild(box)
	window.OnClosing(func(*ui.Window) bool {
		ui.Quit()
		return true
	})
	window.OnContentSizeChanged(func(w *ui.Window, data unsafe.Pointer) bool {
		if editor == nil {
			return true
		}
		width, height = window.ContentSize()
		fmt.Println("window changed", width, height)
		if width == editor.width && height == editor.height {
			return true
		}
		editor.resize(width, height)
		return true
	})
	window.Show()
	return window
}

func getTablineHeight(font *Font) int {
	return font.lineHeight + 12
}

func getStatuslineHeight(font *Font) int {
	return font.lineHeight + 6
}

// InitEditor inits the editor
func InitEditor() error {
	if editor != nil {
		return nil
	}

	fontFamily := ""
	switch runtime.GOOS {
	case "windows":
		fontFamily = "Consolas"
	case "darwin":
		fontFamily = "Courier New"
	default:
		fontFamily = "Monospace"
	}
	font := initFont(fontFamily, 14, 6)
	smallerFont := initFont(fontFamily, 12, 0)

	width := 800
	height := 600
	tablineHeight := getTablineHeight(font)
	statuslineHeight := getStatuslineHeight(font)
	screenHeight := height - tablineHeight - statuslineHeight

	screen := initScreen(width, screenHeight)
	cursor := initCursorBox(width, screenHeight)
	popupMenu := initPopupmenu()
	finder := initFinder()
	tabline := initTabline(width, tablineHeight)
	loc := initLocpopup()
	statusline := initStatusline(width, statuslineHeight)

	box := ui.NewHorizontalBox()
	areaBox := ui.NewHorizontalBox()
	areaBox.Append(screen.box, false)
	areaBox.Append(cursor.box, false)
	areaBox.Append(loc.box, false)
	areaBox.Append(popupMenu.box, false)
	areaBox.Append(finder.box, false)
	box.Append(tabline.box, false)
	box.Append(areaBox, false)
	box.Append(statusline.box, false)

	areaBox.SetSize(width, screenHeight)
	areaBox.SetPosition(0, tablineHeight)
	statusline.box.SetPosition(0, tablineHeight+screenHeight)
	window := initMainWindow(box, width, height)

	neovim, err := nvim.NewEmbedded(&nvim.EmbedOptions{
		Args: os.Args[1:],
	})
	if err != nil {
		fmt.Println("nvim start error", err)
		ui.Quit()
		return err
	}

	editor = &Editor{
		nvim:             neovim,
		nvimAttached:     false,
		window:           window,
		screen:           screen,
		areaBox:          areaBox,
		mode:             "normal",
		close:            make(chan bool),
		cursor:           cursor,
		popup:            popupMenu,
		finder:           finder,
		tabline:          tabline,
		width:            width,
		height:           height,
		tablineHeight:    tablineHeight,
		statusline:       statusline,
		statuslineHeight: statuslineHeight,
		font:             font,
		smallerFont:      smallerFont,
		selectedBg:       newRGBA(81, 154, 186, 0.5),
		matchFg:          newRGBA(81, 154, 186, 1),
	}

	editor.nvimResize()
	editor.handleNotification()
	editor.finder.rePosition()
	go func() {
		err := neovim.Serve()
		if err != nil {
			fmt.Println(err)
		}
		editor.close <- true
	}()

	o := make(map[string]interface{})
	o["rgb"] = true
	o["ext_popupmenu"] = true
	o["ext_tabline"] = true
	err = editor.nvim.AttachUI(editor.cols, editor.rows, o)
	if err != nil {
		fmt.Println("nvim attach UI error", err)
		ui.Quit()
		return nil
	}
	editor.nvim.Subscribe("Gui")
	editor.nvim.Command("runtime plugin/nvim_gui_shim.vim")
	editor.nvim.Command("runtime! ginit.vim")
	editor.nvim.Command("let g:gonvim_running=1")
	fzf.RegisterPlugin(editor.nvim)
	locpopup.RegisterPlugin(editor.nvim)

	go func() {
		<-editor.close
		ui.Quit()
	}()

	return nil
}

func (e *Editor) handleNotification() {
	e.nvim.RegisterHandler("Gui", func(updates ...interface{}) {
		go e.handleRPCGui(updates...)
	})
	e.nvim.RegisterHandler("redraw", func(updates ...[]interface{}) {
		e.redrawMutex.Lock()
		e.handleRedraw(updates...)
		e.redrawMutex.Unlock()
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
	case "finder_show":
		e.finder.show()
	case "finder_hide":
		e.finder.hide()
	case "finder_select":
		e.finder.selectResult(updates[1:])
	case "locpopup_show":
		arg, ok := updates[1].(map[string]interface{})
		if !ok {
			return
		}
		e.cursor.locpopup.show(arg)
	case "locpopup_hide":
		e.cursor.locpopup.hide()
	case "signature_show":
		e.cursor.signature.show(updates[1:])
	case "signature_pos":
		e.cursor.signature.pos(updates[1:])
	case "signature_hide":
		e.cursor.signature.hide()
	default:
		fmt.Println("unhandled Gui event", event)
	}
}

func (e *Editor) handleRedraw(updates ...[]interface{}) {
	screen := e.screen
	go screen.redrawWindows()
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
			color := reflectToInt(args[0])
			if color == -1 {
				editor.Background = newRGBA(0, 0, 0, 1)
			} else {
				bg := calcColor(reflectToInt(args[0]))
				editor.Background = bg
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
		default:
			fmt.Println("Unhandle event", event)
		}
	}
	screen.redraw()
	editor.cursor.draw()
	if !e.nvimAttached {
		e.nvimAttached = true
	}
	go editor.statusline.redraw(false)
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
	e.smallerFont.change(parts[0], height-2)
	e.resize(e.width, e.height)
}

func (e *Editor) guiLinespace(args ...interface{}) {
	fontArg := args[0].([]interface{})
	lineSpace, err := strconv.Atoi(fontArg[0].(string))
	if err != nil {
		return
	}
	e.font.changeLineSpace(lineSpace)
	e.smallerFont.changeLineSpace(lineSpace)
	e.resize(e.width, e.height)
}

func (e *Editor) resize(width, height int) {
	ui.QueueMain(func() {
		e.resizeMutex.Lock()
		defer e.resizeMutex.Unlock()
		e.tablineHeight = getTablineHeight(e.font)
		e.statuslineHeight = getStatuslineHeight(e.font)
		screenHeight := height - e.tablineHeight - e.statuslineHeight
		e.width = width
		e.height = height
		e.areaBox.SetSize(width, screenHeight)
		e.areaBox.SetPosition(0, e.tablineHeight)
		e.screen.box.SetSize(width, screenHeight)
		e.screen.setSize(width, screenHeight)
		e.cursor.setSize(width, screenHeight)
		e.tabline.resize(width, e.tablineHeight)
		e.statusline.redraw(true)
		e.statusline.box.SetSize(width, e.statuslineHeight)
		e.statusline.setSize(width, e.statuslineHeight)
		e.statusline.box.SetPosition(0, e.tablineHeight+screenHeight)
		e.finder.rePosition()
		e.nvimResize()
	})
}

func (e *Editor) nvimResize() {
	e.redrawMutex.Lock()
	defer e.redrawMutex.Unlock()
	width := e.screen.width
	height := e.screen.height
	// cols := width / editor.font.width
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

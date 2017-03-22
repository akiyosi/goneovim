package gonvim

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/dzhou121/neovim-fzf-shim/rplugin/go/fzf"
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
	nvim         *nvim.Nvim
	nvimAttached bool
	mode         string
	font         *Font
	rows         int
	cols         int
	cursor       *CursorHandler
	Foreground   RGBA
	Background   RGBA
	window       *ui.Window
	area         *ui.Area
	areaHandler  *AreaHandler
	close        chan bool
	popup        *PopupMenu
	finder       *Finder
	width        int
	height       int
}

func initWindow(box *ui.Box, width, height int) *ui.Window {
	window := ui.NewWindow("Gonvim", width, height, false)
	window.SetChild(box)
	window.OnClosing(func(*ui.Window) bool {
		ui.Quit()
		return true
	})
	window.OnContentSizeChanged(func(w *ui.Window, data unsafe.Pointer) bool {
		if editor == nil {
			return
		}
		width, height = window.ContentSize()
		editor.width = width
		editor.height = height
		editor.area.SetSize(width, height)
		editor.resize()
		editor.finder.rePosition()
		return true
	})
	window.Show()
	return window
}

// InitEditor inits the editor
func InitEditor() error {
	if editor != nil {
		return nil
	}
	width := 800
	height := 600
	ah := initArea()
	cursor := &CursorHandler{}
	cursorArea := ui.NewArea(cursor)
	cursor.area = cursorArea

	popupMenu := initPopupmenu()
	finder := initFinder()

	box := ui.NewHorizontalBox()
	box.Append(ah.area, false)
	box.Append(cursor.area, false)
	box.Append(popupMenu.box, false)
	box.Append(finder.box, false)

	ah.area.SetSize(width, height)
	// ah.area.SetPosition(100, 100)
	window := initWindow(box, width, height)

	neovim, err := nvim.NewEmbedded(&nvim.EmbedOptions{
		Args: os.Args[1:],
	})
	if err != nil {
		return err
	}

	font := initFont("", 14, 0)

	editor = &Editor{
		nvim:         neovim,
		nvimAttached: false,
		window:       window,
		area:         ah.area,
		areaHandler:  ah,
		mode:         "normal",
		close:        make(chan bool),
		cursor:       cursor,
		popup:        popupMenu,
		finder:       finder,
		width:        width,
		height:       height,
		font:         font,
		cols:         0,
		rows:         0,
	}

	editor.resize()
	editor.handleNotification()
	editor.finder.rePosition()
	go func() {
		neovim.Serve()
		editor.close <- true
	}()

	o := make(map[string]interface{})
	o["rgb"] = true
	o["popupmenu_external"] = true
	editor.nvim.AttachUI(editor.cols, editor.rows, o)
	editor.nvim.Subscribe("Gui")
	editor.nvim.Command("runtime plugin/nvim_gui_shim.vim")
	editor.nvim.Command("runtime! ginit.vim")
	fzf.RegisterPlugin(editor.nvim)

	go func() {
		<-editor.close
		ui.Quit()
	}()

	return nil
}

func (e *Editor) handleNotification() {
	ah := e.areaHandler
	e.nvim.RegisterHandler("Gui", func(updates ...interface{}) {
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
		default:
			fmt.Println("unhandled Gui event", event)
		}
	})
	mutex := &sync.Mutex{}
	e.nvim.RegisterHandler("redraw", func(updates ...[]interface{}) {
		mutex.Lock()
		for _, update := range updates {
			event := update[0].(string)
			args := update[1:]
			switch event {
			case "update_fg":
				args := update[1].([]interface{})
				editor.Foreground = calcColor(reflectToInt(args[0]))
			case "update_bg":
				args := update[1].([]interface{})
				bg := calcColor(reflectToInt(args[0]))
				editor.Background = bg
			case "cursor_goto":
				ah.cursorGoto(args)
			case "put":
				ah.put(args)
			case "eol_clear":
				ah.eolClear(args)
			case "clear":
				ah.clear(args)
			case "resize":
				ah.resize(args)
			case "highlight_set":
				ah.highlightSet(args)
			case "set_scroll_region":
				ah.setScrollRegion(args)
			case "scroll":
				ah.scroll(args)
			case "mode_change":
				ah.modeChange(args)
			case "popupmenu_show":
				editor.popup.show(args)
			case "popupmenu_hide":
				editor.popup.hide(args)
			case "popupmenu_select":
				editor.popup.selectItem(args)
			default:
				fmt.Println("Unhandle event", event)
			}
		}
		mutex.Unlock()
		if !e.nvimAttached {
			e.nvimAttached = true
		}
		drawCursor()
	})
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
	e.resize()
}

func (e *Editor) guiLinespace(args ...interface{}) {
	fontArg := args[0].([]interface{})
	lineSpace, err := strconv.Atoi(fontArg[0].(string))
	if err != nil {
		return
	}
	e.font.changeLineSpace(lineSpace)
	e.resize()
}

func (e *Editor) resize() {
	width := e.width
	height := e.height
	cols := width / editor.font.width
	rows := height / editor.font.lineHeight
	oldCols := editor.cols
	oldRows := editor.rows
	editor.cols = cols
	editor.rows = rows
	if oldCols > 0 && oldRows > 0 {
		editor.nvim.TryResizeUI(cols, rows)
	}
}

func drawCursor() {
	row := editor.areaHandler.cursor[0]
	col := editor.areaHandler.cursor[1]
	ui.QueueMain(func() {
		editor.cursor.area.SetPosition(col*editor.font.width, row*editor.font.lineHeight)
	})

	mode := editor.mode
	if mode == "normal" {
		ui.QueueMain(func() {
			editor.cursor.area.SetSize(editor.font.width, editor.font.lineHeight)
			editor.cursor.bg = newRGBA(255, 255, 255, 0.5)
		})
	} else if mode == "insert" {
		ui.QueueMain(func() {
			editor.cursor.area.SetSize(1, editor.font.lineHeight)
			editor.cursor.bg = newRGBA(255, 255, 255, 0.9)
		})
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

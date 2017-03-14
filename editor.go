package gonvim

import (
	"fmt"
	"os"
	"sync"
	"unsafe"

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
	font         *ui.Font
	rows         int
	cols         int
	cursor       *ui.Area
	fontWidth    int
	fontHeight   int
	LineHeight   int
	Foreground   RGBA
	Background   RGBA
	window       *ui.Window
	area         *ui.Area
	areaHandler  *AreaHandler
	close        chan bool
	popup        *PopupMenu
}

func initWindow(box *ui.Box, width, height int) *ui.Window {
	window := ui.NewWindow("Gonvim", width, height, false)
	window.SetChild(box)
	window.OnClosing(func(*ui.Window) bool {
		ui.Quit()
		return true
	})
	window.OnContentSizeChanged(func(w *ui.Window, data unsafe.Pointer) bool {
		width, height = window.ContentSize()
		editor.area.SetSize(width, height)
		cols := width / editor.fontWidth
		rows := height / editor.LineHeight
		if editor.cols != cols || editor.rows != rows {
			editor.cols = cols
			editor.rows = rows
			editor.nvim.TryResizeUI(cols, rows)
		}
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
	cursor := ui.NewArea(&AreaHandler{})
	popupMenu := initPopupmenu()

	box := ui.NewHorizontalBox()
	box.Append(ah.area, false)
	box.Append(cursor, false)
	box.Append(popupMenu.box, false)

	ah.area.SetSize(width, height)
	// ah.area.SetPosition(100, 100)
	window := initWindow(box, width, height)
	font := initFont()

	neovim, err := nvim.NewEmbedded(&nvim.EmbedOptions{
		Args: os.Args[1:],
	})
	if err != nil {
		return err
	}

	cols := int(width / font.width)
	rows := int(height / font.lineHeight)

	content := make([][]*Char, rows)
	for i := 0; i < rows; i++ {
		content[i] = make([]*Char, cols)
	}

	editor = &Editor{
		nvim:         neovim,
		nvimAttached: false,
		font:         font.font,
		fontWidth:    font.width,
		fontHeight:   font.height,
		rows:         rows,
		cols:         cols,
		LineHeight:   font.lineHeight,
		window:       window,
		area:         ah.area,
		areaHandler:  ah,
		mode:         "normal",
		close:        make(chan bool),
		cursor:       cursor,
		popup:        popupMenu,
	}

	editor.handleRedraw()
	go func() {
		neovim.Serve()
		editor.close <- true
	}()

	o := make(map[string]interface{})
	o["rgb"] = true
	o["popupmenu_external"] = true
	editor.nvim.AttachUI(cols, rows, o)

	go func() {
		<-editor.close
		ui.Quit()
	}()

	return nil
}

func (e *Editor) handleRedraw() {
	ah := e.areaHandler
	mutext := &sync.Mutex{}
	e.nvim.RegisterHandler("redraw", func(updates ...[]interface{}) {
		mutext.Lock()
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
		mutext.Unlock()
		if !e.nvimAttached {
			e.nvimAttached = true
		}
		drawCursor()
	})
}
func drawCursor() {
	row := editor.areaHandler.cursor[0]
	col := editor.areaHandler.cursor[1]
	ui.QueueMain(func() {
		editor.cursor.SetPosition(col*editor.fontWidth, row*editor.LineHeight)
	})

	mode := editor.mode
	if mode == "normal" {
		ui.QueueMain(func() {
			editor.cursor.SetSize(editor.fontWidth, editor.LineHeight)
			editor.cursor.SetBackground(&ui.Brush{
				Type: ui.Solid,
				R:    1,
				G:    1,
				B:    1,
				A:    0.5,
			})
		})
	} else if mode == "insert" {
		ui.QueueMain(func() {
			editor.cursor.SetSize(1, editor.LineHeight)
			editor.cursor.SetBackground(&ui.Brush{
				Type: ui.Solid,
				R:    1,
				G:    1,
				B:    1,
				A:    0.9,
			})
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

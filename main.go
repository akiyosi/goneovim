package main

import (
	"fmt"
	"math"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"unsafe"

	"github.com/dzhou121/ui"
	"github.com/neovim/go-client/nvim"

	"net/http"
	_ "net/http/pprof"
)

var editor *Editor

// AreaHandler is
type AreaHandler struct {
	area         *ui.Area
	dp           *ui.AreaDrawParams
	highlight    Highlight
	cursor       []int
	lastCursor   []int
	content      [][]*Char
	scrollRegion []int
	width        int
	height       int
	span         *ui.Area
}

// SpanHandler is
type SpanHandler struct {
	AreaHandler
	text          string
	bg            *RGBA
	color         *RGBA
	font          *ui.Font
	paddingLeft   int
	paddingRight  int
	paddingTop    int
	paddingBottom int
}

// Highlight is
type Highlight struct {
	foreground *RGBA
	background *RGBA
}

// RGBA is
type RGBA struct {
	R float64
	G float64
	B float64
	A float64
}

// Char is
type Char struct {
	char      string
	highlight Highlight
}

// Font is
type Font struct {
	font       *ui.Font
	width      int
	height     int
	lineHeight int
}

// PopupItem is
type PopupItem struct {
	kind *SpanHandler
	menu *SpanHandler
}

// Editor is the editor
type Editor struct {
	nvim            *nvim.Nvim
	nvimAttached    bool
	mode            string
	font            *ui.Font
	rows            int
	cols            int
	cursor          *ui.Area
	fontWidth       int
	fontHeight      int
	LineHeight      int
	Foreground      RGBA
	Background      RGBA
	window          *ui.Window
	area            *ui.Area
	areaHandler     *AreaHandler
	close           chan bool
	popup           *ui.Box
	popupItems      []*PopupItem
	totalPopupItems int
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

func initArea() *AreaHandler {
	ah := &AreaHandler{
		cursor:       []int{0, 0},
		lastCursor:   []int{0, 0},
		scrollRegion: []int{0, 0, 0, 0},
	}
	area := ui.NewArea(ah)
	ah.area = area
	return ah
}

func initFont() *Font {
	fontDesc := &ui.FontDescriptor{
		Family:  "InconsolataforPowerline Nerd Font",
		Size:    14,
		Weight:  ui.TextWeightNormal,
		Italic:  ui.TextItalicNormal,
		Stretch: ui.TextStretchNormal,
	}
	font := ui.LoadClosestFont(fontDesc)
	textLayout := ui.NewTextLayout("a", font, -1)
	w, h := textLayout.Extents()
	width := int(math.Ceil(w))
	height := int(math.Ceil(h))
	lineHeight := int(math.Ceil(h * 1.5))
	return &Font{
		font:       font,
		width:      width,
		height:     height,
		lineHeight: lineHeight,
	}
}

func newEditor() error {
	if editor != nil {
		return nil
	}
	width := 800
	height := 600
	ah := initArea()
	cursor := ui.NewArea(&AreaHandler{})
	popup := ui.NewHorizontalBox()
	var popupItems []*PopupItem
	totalPopupItems := 10
	for i := 0; i < totalPopupItems; i++ {
		kindSpanHandler := &SpanHandler{}
		kindSpan := ui.NewArea(kindSpanHandler)
		kindSpanHandler.span = kindSpan

		menuSpanHandler := &SpanHandler{}
		menuSpan := ui.NewArea(menuSpanHandler)
		menuSpanHandler.span = menuSpan

		popupItem := &PopupItem{
			kind: kindSpanHandler,
			menu: menuSpanHandler,
		}

		popupItems = append(popupItems, popupItem)
		popup.Append(kindSpan, false)
		popup.Append(menuSpan, false)
	}
	popup.SetSize(1000, 500)
	// popup.Hide()

	box := ui.NewHorizontalBox()
	box.Append(ah.area, false)
	box.Append(cursor, false)
	box.Append(popup, false)

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
		nvim:            neovim,
		nvimAttached:    false,
		font:            font.font,
		fontWidth:       font.width,
		fontHeight:      font.height,
		rows:            rows,
		cols:            cols,
		LineHeight:      font.lineHeight,
		window:          window,
		area:            ah.area,
		areaHandler:     ah,
		mode:            "normal",
		close:           make(chan bool),
		cursor:          cursor,
		popup:           popup,
		popupItems:      popupItems,
		totalPopupItems: totalPopupItems,
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
				// ui.QueueMain(func() {
				// 	editor.area.SetBackground(&ui.Brush{
				// 		Type: ui.Solid,
				// 		R:    bg.R,
				// 		G:    bg.G,
				// 		B:    bg.B,
				// 		A:    bg.A,
				// 	})
				// })
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
				ah.popupmenuShow(args)
			case "popupmenu_hide":
				ah.popupmenuHide(args)
			case "popupmenu_select":
				ah.popupmenuSelect(args)
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

func areaQueueRedraw(x, y, width, heigt int) {
	ui.QueueMain(func() {
		editor.area.QueueRedraw(
			float64(x*editor.fontWidth),
			float64(y*editor.LineHeight),
			float64(width*editor.fontWidth),
			float64(heigt*editor.LineHeight),
		)
	})
}

func areaScrollRect(x, y, width, heigt, offsetX, offsetY int) {
	ui.QueueMain(func() {
		editor.area.ScrollRect(
			float64(x*editor.fontWidth),
			float64(y*editor.LineHeight),
			float64(width*editor.fontWidth),
			float64(heigt*editor.LineHeight),
			float64(offsetX*editor.fontWidth),
			float64(offsetY*editor.LineHeight),
		)
	})
}

func (ah *AreaHandler) cursorGoto(args []interface{}) {
	pos, _ := args[0].([]interface{})
	ah.cursor[0] = reflectToInt(pos[0])
	ah.cursor[1] = reflectToInt(pos[1])
}

func (ah *AreaHandler) put(args []interface{}) {
	numChars := 0
	x := ah.cursor[1]
	y := ah.cursor[0]
	row := ah.cursor[0]
	col := ah.cursor[1]
	line := ah.content[row]
	for _, arg := range args {
		chars := arg.([]interface{})
		for _, c := range chars {
			char := line[col]
			if char == nil {
				char = &Char{}
				line[col] = char
			}
			char.char = c.(string)
			char.highlight = ah.highlight
			col++
			numChars++
		}
	}
	ah.cursor[1] = col
	// we redraw one character more than the chars put for double width characters
	areaQueueRedraw(x, y, numChars+1, 1)
}

func (ah *AreaHandler) resize(args []interface{}) {
	ah.cursor[0] = 0
	ah.cursor[1] = 0
	ah.content = make([][]*Char, editor.rows)
	for i := 0; i < editor.rows; i++ {
		ah.content[i] = make([]*Char, editor.cols)
	}
	ui.QueueMain(func() {
		editor.area.QueueRedrawAll()
	})
}

func (ah *AreaHandler) clear(args []interface{}) {
	ah.cursor[0] = 0
	ah.cursor[1] = 0
	ah.content = make([][]*Char, editor.rows)
	for i := 0; i < editor.rows; i++ {
		ah.content[i] = make([]*Char, editor.cols)
	}
	ui.QueueMain(func() {
		editor.area.QueueRedrawAll()
	})
}

func (ah *AreaHandler) eolClear(args []interface{}) {
	row := ah.cursor[0]
	col := ah.cursor[1]
	line := ah.content[row]
	numChars := 0
	for x := col; x < len(line); x++ {
		line[x] = nil
		numChars++
	}
	areaQueueRedraw(col, row, numChars, 1)
}

func (ah *AreaHandler) highlightSet(args []interface{}) {
	for _, arg := range args {
		hl := arg.([]interface{})[0].(map[string]interface{})
		highlight := Highlight{}
		for key, value := range hl {
			switch key {
			case "foreground":
				rgba := calcColor(reflectToInt(value))
				highlight.foreground = &rgba
			case "background":
				rgba := calcColor(reflectToInt(value))
				highlight.background = &rgba
			}
		}
		ah.highlight = highlight
	}
}

func (ah *AreaHandler) setScrollRegion(args []interface{}) {
	arg := args[0].([]interface{})
	top := reflectToInt(arg[0])
	bot := reflectToInt(arg[1])
	left := reflectToInt(arg[2])
	right := reflectToInt(arg[3])
	ah.scrollRegion = []int{int(top), int(bot), int(left), int(right)}
}

func (p *PopupItem) setItem(item []interface{}, selected bool) {
	text := item[0].(string)
	kindText := item[1].(string)
	p.setKind(kindText, selected)

	fg := newRGBA(205, 211, 222, 1)
	p.menu.SetColor(fg)
	if selected {
		p.menu.SetBackground(newRGBA(81, 154, 186, 1))
	} else {
		p.menu.SetBackground(newRGBA(14, 17, 18, 1))
	}
	p.menu.SetFont(editor.font)
	p.menu.SetText(text)
	p.menu.paddingLeft = 10
	p.menu.paddingRight = 10
	p.menu.paddingTop = 10
	p.menu.paddingBottom = 10
}

func (p *PopupItem) setKind(kindText string, selected bool) {
	switch kindText {
	case "function":
		kindText = "f"
		p.kind.SetColor(newRGBA(97, 174, 239, 1))
		p.kind.SetBackground(newRGBA(97, 174, 239, 0.2))
	default:
		kindText = "b"
		p.kind.SetColor(newRGBA(151, 195, 120, 1))
		p.kind.SetBackground(newRGBA(151, 195, 120, 0.2))
	}
	p.kind.SetText(kindText)
}

func (p *PopupItem) hide() {
	ui.QueueMain(func() {
		p.kind.span.Hide()
		p.menu.span.Hide()
	})
}

func (ah *AreaHandler) popupmenuShow(args []interface{}) {
	arg := args[0].([]interface{})
	items := arg[0].([]interface{})
	selected := reflectToInt(arg[1])
	row := reflectToInt(arg[2])
	col := reflectToInt(arg[3])
	popup := editor.popup
	popupItems := editor.popupItems
	i := 0
	widthMax := 0
	heightSum := 0
	for i = 0; i < editor.totalPopupItems; i++ {
		p := popupItems[i]
		if i >= len(items) {
			p.hide()
			continue
		}

		item := items[i].([]interface{})
		p.setItem(item, selected == i)

		width, height := p.menu.getSize()
		if width > widthMax {
			widthMax = width
		}
		y := heightSum
		heightSum += height
		ui.QueueMain(func() {
			p.menu.span.SetPosition(0, y)
		})
	}

	for i = 0; i < editor.totalPopupItems; i++ {
		if i >= len(items) {
			continue
		}
		p := popupItems[i]
		_, height := p.menu.getSize()
		ui.QueueMain(func() {
			p.menu.span.SetSize(widthMax, height)
			p.menu.span.Show()
			p.menu.span.QueueRedrawAll()
		})
	}

	ui.QueueMain(func() {
		popup.SetPosition(
			col*editor.fontWidth,
			(row+1)*editor.LineHeight,
		)
		popup.SetSize(widthMax, heightSum)
		popup.Show()
	})
}

func (ah *AreaHandler) popupmenuHide(args []interface{}) {
	ui.QueueMain(func() {
		editor.popup.Hide()
	})
}

func (ah *AreaHandler) popupmenuSelect(args []interface{}) {
	selected := reflectToInt(args[0].([]interface{})[0])
	for i := 0; i < editor.totalPopupItems; i++ {
		p := editor.popupItems[i]
		if selected == i {
			p.menu.SetBackground(newRGBA(81, 154, 186, 1))
			ui.QueueMain(func() {
				p.menu.span.QueueRedrawAll()
			})
		} else {
			p.menu.SetBackground(newRGBA(14, 17, 18, 1))
			ui.QueueMain(func() {
				p.menu.span.QueueRedrawAll()
			})
		}
	}
}

func (ah *AreaHandler) modeChange(args []interface{}) {
	arg := args[0].([]interface{})
	editor.mode = arg[0].(string)
}

func (ah *AreaHandler) scroll(args []interface{}) {
	count := int(args[0].([]interface{})[0].(int64))
	top := ah.scrollRegion[0]
	bot := ah.scrollRegion[1]
	left := ah.scrollRegion[2]
	right := ah.scrollRegion[3]

	//areaScrollRect(left, top, (right - left + 1), (bot - top + 1), 0, -count)
	areaQueueRedraw(left, top, (right - left + 1), (bot - top + 1))

	if count > 0 {
		for row := top; row <= bot-count; row++ {
			for col := left; col <= right; col++ {
				ah.content[row][col] = ah.content[row+count][col]
			}
		}
		for row := bot - count + 1; row <= bot; row++ {
			for col := left; col <= right; col++ {
				ah.content[row][col] = nil
			}
		}
		areaQueueRedraw(left, (bot - count + 1), (right - left), count)
		if top > 0 {
			areaQueueRedraw(left, (top - count), (right - left), count)
		}
	} else {
		for row := bot; row >= top-count; row-- {
			for col := left; col <= right; col++ {
				ah.content[row][col] = ah.content[row+count][col]
			}
		}
		for row := top; row < top-count; row++ {
			for col := left; col <= right; col++ {
				ah.content[row][col] = nil
			}
		}
		areaQueueRedraw(left, top, (right - left), -count)
		if bot < editor.rows-1 {
			areaQueueRedraw(left, bot+1, (right - left), -count)
		}
	}
}

// Draw is
func (ah *AreaHandler) Draw(a *ui.Area, dp *ui.AreaDrawParams) {
	if ah.area == nil {
		return
	}
	if editor == nil {
		return
	}
	if !editor.nvimAttached {
		return
	}

	row := int(math.Ceil(dp.ClipY / float64(editor.LineHeight)))
	col := int(math.Ceil(dp.ClipX / float64(editor.fontWidth)))
	rows := int(math.Ceil(dp.ClipHeight / float64(editor.LineHeight)))
	cols := int(math.Ceil(dp.ClipWidth / float64(editor.fontWidth)))

	p := ui.NewPath(ui.Winding)
	p.AddRectangle(dp.ClipX, dp.ClipY, dp.ClipWidth, dp.ClipHeight)
	p.End()

	bg := editor.Background

	dp.Context.Fill(p, &ui.Brush{
		Type: ui.Solid,
		R:    bg.R,
		G:    bg.G,
		B:    bg.B,
		A:    1,
	})
	p.Free()

	for y := row; y < row+rows; y++ {
		if y >= editor.rows {
			continue
		}
		ah.fillHightlight(dp, y, col, cols)
		ah.drawText(dp, y, col, cols)
	}
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

func (ah *AreaHandler) drawText(dp *ui.AreaDrawParams, y int, col int, cols int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r, debug.Stack())
		}
	}()
	if y >= len(ah.content) {
		return
	}
	line := ah.content[y]
	text := ""
	var specialChars []int
	start := -1
	end := col
	for x := col; x < col+cols; x++ {
		if x >= len(line) {
			continue
		}
		char := line[x]
		if char == nil {
			text += " "
			continue
		}
		if char.char == " " {
			text += " "
			continue
		}
		if char.char == "" {
			text += " "
			continue
		}
		if !isNormalWidth(char.char) {
			text += " "
			specialChars = append(specialChars, x)
			continue
		}
		text += char.char
		if start == -1 {
			start = x
		}
		end = x
	}
	if start == -1 {
		return
	}
	text = strings.TrimSpace(text)
	textLayout := ui.NewTextLayout(text, editor.font, -1)
	shift := (editor.LineHeight - editor.fontHeight) / 2

	for x := start; x <= end; x++ {
		char := line[x]
		if char == nil || char.char == " " {
			continue
		}
		fg := editor.Foreground
		if char.highlight.foreground != nil {
			fg = *(char.highlight.foreground)
		}
		textLayout.SetColor(x-start, x-start+1, fg.R, fg.G, fg.B, fg.A)
	}
	dp.Context.Text(float64(start*editor.fontWidth), float64(y*editor.LineHeight+shift), textLayout)
	textLayout.Free()

	for _, x := range specialChars {
		char := line[x]
		if char == nil || char.char == " " {
			continue
		}
		fg := editor.Foreground
		if char.highlight.foreground != nil {
			fg = *(char.highlight.foreground)
		}
		textLayout := ui.NewTextLayout(char.char, editor.font, -1)
		textLayout.SetColor(0, 1, fg.R, fg.G, fg.B, fg.A)
		dp.Context.Text(float64(x*editor.fontWidth), float64(y*editor.LineHeight+shift), textLayout)
		textLayout.Free()
	}
}

func (ah *AreaHandler) fillHightlight(dp *ui.AreaDrawParams, y int, col int, cols int) {
	if y >= len(ah.content) {
		return
	}
	line := ah.content[y]
	start := -1
	end := -1
	var lastBg *RGBA
	var bg *RGBA
	for x := col; x < col+cols; x++ {
		if x >= len(line) {
			continue
		}
		char := line[x]
		if char != nil {
			bg = char.highlight.background
		} else {
			bg = nil
		}
		if bg != nil {
			if lastBg == nil {
				start = x
				end = x
				lastBg = bg
			} else {
				if lastBg.equals(bg) {
					end = x
				} else {
					// last bg is different; draw the previous and start a new one
					p := ui.NewPath(ui.Winding)
					p.AddRectangle(
						float64(start*editor.fontWidth),
						float64(y*editor.LineHeight),
						float64((end-start+1)*editor.fontWidth),
						float64(editor.LineHeight),
					)
					p.End()
					dp.Context.Fill(p, &ui.Brush{
						Type: ui.Solid,
						R:    lastBg.R,
						G:    lastBg.G,
						B:    lastBg.B,
						A:    lastBg.A,
					})
					p.Free()

					// start a new one
					start = x
					end = x
					lastBg = bg
				}
			}
		} else {
			if lastBg != nil {
				p := ui.NewPath(ui.Winding)
				p.AddRectangle(
					float64(start*editor.fontWidth),
					float64(y*editor.LineHeight),
					float64((end-start+1)*editor.fontWidth),
					float64(editor.LineHeight),
				)
				p.End()
				dp.Context.Fill(p, &ui.Brush{
					Type: ui.Solid,
					R:    lastBg.R,
					G:    lastBg.G,
					B:    lastBg.B,
					A:    lastBg.A,
				})
				p.Free()

				// start a new one
				start = x
				end = x
				lastBg = nil
			}
		}
	}
	if lastBg != nil {
		p := ui.NewPath(ui.Winding)
		p.AddRectangle(
			float64(start*editor.fontWidth),
			float64(y*editor.LineHeight),
			float64((end-start+1)*editor.fontWidth),
			float64(editor.LineHeight),
		)
		p.End()

		dp.Context.Fill(p, &ui.Brush{
			Type: ui.Solid,
			R:    lastBg.R,
			G:    lastBg.G,
			B:    lastBg.B,
			A:    lastBg.A,
		})
		p.Free()
	}
}

// MouseEvent is
func (ah *AreaHandler) MouseEvent(a *ui.Area, me *ui.AreaMouseEvent) {
}

// MouseCrossed is
func (ah *AreaHandler) MouseCrossed(a *ui.Area, left bool) {
}

// DragBroken is
func (ah *AreaHandler) DragBroken(a *ui.Area) {
}

// KeyEvent is
func (ah *AreaHandler) KeyEvent(a *ui.Area, key *ui.AreaKeyEvent) (handled bool) {
	if key.Up {
		return false
	}
	if key.Key == 0 && key.ExtKey == 0 {
		return false
	}
	namedKey := ""
	mod := ""

	switch key.Modifiers {
	case ui.Ctrl:
		mod = "C-"
	case ui.Alt:
		mod = "A-"
	case ui.Super:
		mod = "M-"
	}

	switch key.ExtKey {
	case ui.Escape:
		namedKey = "Esc"
	case ui.Insert:
		namedKey = "Insert"
	case ui.Delete:
		namedKey = "Del"
	case ui.Left:
		namedKey = "Left"
	case ui.Right:
		namedKey = "Right"
	case ui.Down:
		namedKey = "Down"
	case ui.Up:
		namedKey = "Up"
	}

	char := ""
	char = string(key.Key)
	if char == "\n" || char == "\r" {
		namedKey = "Enter"
	} else if char == "\t" {
		namedKey = "Tab"
	} else if key.Key == 127 {
		namedKey = "BS"
	} else if char == "<" {
		namedKey = "LT"
	}

	input := ""
	if namedKey != "" {
		input = fmt.Sprintf("<%s>", namedKey)
	} else if mod != "" {
		input = fmt.Sprintf("<%s%s>", mod, char)
	} else {
		input = char
	}
	editor.nvim.Input(input)
	return true
}

// Draw the span
func (s *SpanHandler) Draw(a *ui.Area, dp *ui.AreaDrawParams) {
	if s.bg == nil {
		return
	}
	bg := s.bg
	p := ui.NewPath(ui.Winding)
	p.AddRectangle(dp.ClipX, dp.ClipY, dp.ClipWidth, dp.ClipHeight)
	p.End()
	dp.Context.Fill(p, &ui.Brush{
		Type: ui.Solid,
		R:    bg.R,
		G:    bg.G,
		B:    bg.B,
		A:    1,
	})
	p.Free()

	textLayout := s.getTextLayout()
	dp.Context.Text(
		float64(s.paddingLeft),
		float64(s.paddingTop),
		textLayout,
	)
	textLayout.Free()
}

// SetColor sets the color
func (s *SpanHandler) SetColor(rgba *RGBA) {
	s.color = rgba
}

// SetBackground sets the color
func (s *SpanHandler) SetBackground(rgba *RGBA) {
	s.bg = rgba
}

// SetFont sets the font
func (s *SpanHandler) SetFont(font *ui.Font) {
	s.font = font
}

// SetText sets the text
func (s *SpanHandler) SetText(text string) {
	s.text = text
}

func (s *SpanHandler) getTextLayout() *ui.TextLayout {
	textLayout := ui.NewTextLayout(s.text, s.font, -1)
	fg := s.color
	textLayout.SetColor(0, len(s.text), fg.R, fg.G, fg.B, fg.A)
	return textLayout
}

func (s *SpanHandler) getSize() (int, int) {
	textLayout := s.getTextLayout()
	w, h := textLayout.Extents()
	width := int(math.Ceil(w)) + s.paddingLeft + s.paddingRight
	height := int(math.Ceil(h)) + s.paddingTop + s.paddingBottom
	textLayout.Free()
	return width, height
}

func calcColor(c int) RGBA {
	b := float64(c&255) / 255
	g := float64((c>>8)&255) / 255
	r := float64((c>>16)&255) / 255
	return RGBA{
		R: r,
		G: g,
		B: b,
		A: 1,
	}
}

func (rgba *RGBA) copy() *RGBA {
	return &RGBA{
		R: rgba.R,
		G: rgba.G,
		B: rgba.B,
		A: rgba.A,
	}
}

func (rgba *RGBA) equals(other *RGBA) bool {
	return rgba.R == other.R && rgba.G == other.G && rgba.B == other.B && rgba.A == other.A
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

func isNormalWidth(char string) bool {
	if char[0] > 127 {
		return false
	}
	return true
}

func reflectToInt(iface interface{}) int {
	o, ok := iface.(int64)
	if ok {
		return int(o)
	}
	return int(iface.(uint64))
}

func main() {
	go func() {
		http.ListenAndServe("0.0.0.0:6060", nil)
	}()
	err := ui.Main(func() {
		newEditor()
	})
	if err != nil {
		panic(err)
	}
}

func newRGBA(r int, g int, b int, a float64) *RGBA {
	return &RGBA{
		R: float64(r) / 255,
		G: float64(g) / 255,
		B: float64(b) / 255,
		A: a,
	}
}

package gonvim

import (
	"strings"
	"sync"

	"github.com/dzhou121/ui"
)

// Finder is a fuzzy finder window
type Finder struct {
	box         *ui.Box
	pattern     *SpanHandler
	patternText string
	items       []*FinderItem
	mutex       *sync.Mutex
	width       int
	cursor      *ui.Area
	resultType  string
	agTypes     []string
}

// FinderItem is the result shown
type FinderItem struct {
	item *SpanHandler
}

func initFinder() *Finder {
	width := 600

	box := ui.NewHorizontalBox()
	box.SetSize(width, 500)

	patternHandler := &SpanHandler{}
	pattern := ui.NewArea(patternHandler)
	patternHandler.span = pattern
	patternHandler.paddingLeft = 10
	patternHandler.paddingRight = 10
	patternHandler.paddingTop = 8
	patternHandler.paddingBottom = 8

	cursor := ui.NewArea(&AreaHandler{})
	cursor.SetSize(1, 24)
	cursor.SetBackground(&ui.Brush{
		Type: ui.Solid,
		R:    1,
		G:    1,
		B:    1,
		A:    0.9,
	})

	box.Append(pattern, false)
	box.Append(cursor, false)
	box.SetShadow(0, 2, 0, 0, 0, 1, 4)
	box.Hide()

	f := &Finder{
		box:     box,
		pattern: patternHandler,
		items:   []*FinderItem{},
		mutex:   &sync.Mutex{},
		width:   width,
		cursor:  cursor,
	}
	return f
}

func (f *Finder) show() {
	ui.QueueMain(func() {
		f.box.Show()
	})
}

func (f *Finder) hide() {
	ui.QueueMain(func() {
		f.box.Hide()
	})
}

func (f *Finder) cursorPos(args []interface{}) {
	f.cursor.SetSize(1, editor.font.lineHeight)
	p := reflectToInt(args[0])
	x := p*editor.font.width + f.pattern.paddingLeft
	ui.QueueMain(func() {
		f.cursor.SetPosition(x, f.pattern.paddingTop/2)
	})
}

func (f *Finder) selectResult(args []interface{}) {
	selected := reflectToInt(args[0])
	if f.resultType == "ag" {
		n := 0
		for i := 0; i <= selected; i++ {
			for n++; n < len(f.agTypes) && f.agTypes[n] != "ag_line"; n++ {
			}
		}
		selected = n
	}
	for i := 0; i < len(f.items); i++ {
		item := f.items[i]
		if selected == i {
			item.item.SetBackground(newRGBA(81, 154, 186, 0.5))
			ui.QueueMain(func() {
				item.item.span.QueueRedrawAll()
			})
		} else {
			item.item.SetBackground(newRGBA(14, 17, 18, 1))
			ui.QueueMain(func() {
				item.item.span.QueueRedrawAll()
			})
		}
	}
}

func (f *Finder) showPattern(args []interface{}) {
	p := args[0].(string)
	f.pattern.span.SetSize(f.width, 8+8+editor.font.height)
	f.pattern.SetText(p)
	f.patternText = p
	f.pattern.SetFont(editor.font.font)
	fg := newRGBA(205, 211, 222, 1)
	f.pattern.SetColor(fg)
	f.pattern.SetBackground(newRGBA(14, 17, 18, 1))
	ui.QueueMain(func() {
		f.box.Show()
		f.pattern.span.Show()
		f.pattern.span.QueueRedrawAll()
	})
}

func (f *Finder) rePosition() {
	x := (editor.width - f.width) / 2
	ui.QueueMain(func() {
		f.box.SetPosition(x, 0)
	})
}

func (f *Finder) showResult(args []interface{}) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	selected := reflectToInt(args[1])

	match := [][]int{}
	for _, i := range args[2].([]interface{}) {
		m := []int{}
		for _, n := range i.([]interface{}) {
			m = append(m, reflectToInt(n))
		}
		match = append(match, m)
	}

	resultType := ""
	if args[3] != nil {
		resultType = args[3].(string)
	}
	f.resultType = resultType
	result := []string{}

	agLastFile := ""
	agTypes := []string{}
	agMatches := [][]int{}
	for i, item := range args[0].([]interface{}) {
		text := item.(string)
		if resultType == "ag" {
			parts := strings.SplitN(text, ":", 4)
			if len(parts) < 4 {
				continue
			}
			m := match[i]
			file := parts[0]
			if agLastFile != file {
				fileMatch := []int{}
				for n := range m {
					if m[n] < len(parts[0]) {
						fileMatch = append(fileMatch, m[n])
					}
				}
				result = append(result, parts[0])
				agTypes = append(agTypes, "ag_file")
				agLastFile = file
				agMatches = append(agMatches, fileMatch)
			}
			lineIndex := strings.Index(text, parts[3])
			lineMatch := []int{}
			for n := range m {
				if m[n] >= lineIndex {
					lineMatch = append(lineMatch, m[n]-lineIndex)
				}
			}
			result = append(result, parts[3])
			agTypes = append(agTypes, "ag_line")
			agMatches = append(agMatches, lineMatch)
		} else {
			result = append(result, text)
		}
	}
	f.agTypes = agTypes
	for i, item := range result {
		if i > len(f.items)-1 {
			height := 8 + 8 + editor.font.height
			width := f.width

			itemHandler := &SpanHandler{}
			itemSpan := ui.NewArea(itemHandler)
			itemHandler.span = itemSpan
			itemHandler.matchColor = newRGBA(81, 154, 186, 1)
			y := height * (i + 1)
			ui.QueueMain(func() {
				f.box.Append(itemSpan, false)
				itemSpan.SetSize(width, height)
				itemSpan.SetPosition(0, y)
			})

			f.items = append(f.items, &FinderItem{
				item: itemHandler,
			})
		}
		itemHandler := f.items[i]
		itemHandler.item.textType = resultType
		itemHandler.item.SetText(item)
		itemHandler.item.SetFont(editor.font.font)
		itemHandler.item.paddingLeft = 10
		itemHandler.item.paddingRight = 10
		itemHandler.item.paddingTop = 8
		itemHandler.item.paddingBottom = 8
		fg := newRGBA(205, 211, 222, 1)
		itemHandler.item.SetColor(fg)
		itemHandler.item.match = f.patternText
		if resultType == "ag" {
			itemHandler.item.textType = agTypes[i]
			itemHandler.item.matchIndex = agMatches[i]
		}
		if resultType != "ag" {
			itemHandler.item.matchIndex = match[i]
		}
		if i == selected {
			itemHandler.item.SetBackground(newRGBA(81, 154, 186, 0.5))
		} else {
			itemHandler.item.SetBackground(newRGBA(14, 17, 18, 1))
		}
		ui.QueueMain(func() {
			itemHandler.item.span.Show()
			itemHandler.item.span.QueueRedrawAll()
		})
	}
	for i := len(result); i < len(f.items); i++ {
		item := f.items[i]
		ui.QueueMain(func() {
			item.item.span.Hide()
		})
	}
}

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
	cursor      *CursorHandler
	resultType  string
	agTypes     []string
	hidden      bool
}

// FinderItem is the result shown
type FinderItem struct {
	icon *Svg
	item *SpanHandler
}

func initFinder() *Finder {
	width := 600

	box := ui.NewHorizontalBox()
	box.SetSize(width, 500)

	patternHandler := &SpanHandler{}
	pattern := ui.NewArea(patternHandler)
	patternHandler.area = pattern
	patternHandler.paddingLeft = 10
	patternHandler.paddingRight = 10
	patternHandler.paddingTop = 8
	patternHandler.paddingBottom = 8

	cursor := &CursorHandler{}
	cursorArea := ui.NewArea(cursor)
	cursorArea.SetSize(1, 24)
	cursor.area = cursorArea
	cursor.bg = newRGBA(255, 255, 255, 0.9)

	box.Append(pattern, false)
	box.Append(cursorArea, false)
	box.SetShadow(0, 2, 0, 0, 0, 1, 4)
	box.Hide()

	f := &Finder{
		box:     box,
		pattern: patternHandler,
		items:   []*FinderItem{},
		mutex:   &sync.Mutex{},
		width:   width,
		cursor:  cursor,
		hidden:  true,
	}
	return f
}

func (f *Finder) show() {
	f.hidden = false
	// ui.QueueMain(func() {
	// 	f.box.Show()
	// })
}

func (f *Finder) hide() {
	f.hidden = true
	ui.QueueMain(func() {
		f.box.Hide()
		f.pattern.area.Hide()
		f.cursor.area.Hide()
	})
}

func (f *Finder) cursorPos(args []interface{}) {
	f.cursor.area.SetSize(1, editor.font.lineHeight)
	p := reflectToInt(args[0])
	x := p*editor.font.width + f.pattern.paddingLeft
	ui.QueueMain(func() {
		f.cursor.area.Show()
		f.cursor.area.SetPosition(x, f.pattern.paddingTop/2)
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
			item.item.SetBackground(editor.selectedBg)
			ui.QueueMain(func() {
				item.item.area.QueueRedrawAll()
			})
		} else {
			item.item.SetBackground(newRGBA(14, 17, 18, 1))
			ui.QueueMain(func() {
				item.item.area.QueueRedrawAll()
			})
		}
	}
}

func (f *Finder) showPattern(args []interface{}) {
	p := args[0].(string)
	f.pattern.area.SetSize(f.width, 8+8+editor.font.height)
	f.pattern.SetText(p)
	f.patternText = p
	f.pattern.SetFont(editor.font)
	fg := newRGBA(205, 211, 222, 1)
	f.pattern.SetColor(fg)
	f.pattern.SetBackground(newRGBA(14, 17, 18, 1))
	ui.QueueMain(func() {
		// f.box.Show()
		f.pattern.area.Show()
		f.pattern.area.QueueRedrawAll()
	})
}

func (f *Finder) rePosition() {
	x := (editor.width - f.width) / 2
	ui.QueueMain(func() {
		f.box.SetPosition(x, 0)
	})
}

func (f *Finder) showResult(args []interface{}) {
	if f.hidden {
		return
	}
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
	paddingLeft := 10
	paddingTop := 8
	for i, item := range result {
		if i > len(f.items)-1 {
			height := 8 + 8 + editor.font.height
			width := f.width

			itemHandler := &SpanHandler{}
			itemSpan := ui.NewArea(itemHandler)
			itemHandler.area = itemSpan
			itemHandler.matchColor = editor.matchFg
			itemHandler.paddingLeft = paddingLeft
			itemHandler.paddingRight = paddingLeft
			itemHandler.paddingTop = paddingTop
			itemHandler.paddingBottom = paddingTop
			iconWidth := editor.font.width * 2
			icon := newSvg("default", iconWidth, iconWidth, nil, nil)
			y := height * (i + 1)
			ui.QueueMain(func() {
				f.box.Append(itemSpan, false)
				f.box.Append(icon.area, false)
				itemSpan.SetSize(width, height)
				itemSpan.SetPosition(0, y)
				icon.setPosition(paddingLeft, y+paddingTop)
			})

			f.items = append(f.items, &FinderItem{
				item: itemHandler,
				icon: icon,
			})
		}
		itemHandler := f.items[i]
		itemHandler.item.textType = resultType
		itemHandler.item.SetText(item)
		itemHandler.item.SetFont(editor.font)
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
		if resultType == "file" {
			itemHandler.icon.name = getFileType(item)
			itemHandler.item.paddingLeft = paddingLeft + itemHandler.icon.width + editor.font.width
			ui.QueueMain(func() {
				itemHandler.icon.area.Show()
				itemHandler.icon.area.QueueRedrawAll()
			})
		} else if resultType == "dir" {
			itemHandler.icon.name = "folder"
			itemHandler.item.paddingLeft = paddingLeft + itemHandler.icon.width + editor.font.width
			ui.QueueMain(func() {
				itemHandler.icon.area.Show()
				itemHandler.icon.area.QueueRedrawAll()
			})
		} else {
			itemHandler.item.paddingLeft = paddingLeft
			ui.QueueMain(func() {
				itemHandler.icon.area.Hide()
			})
		}
		if i == selected {
			itemHandler.item.SetBackground(editor.selectedBg)
		} else {
			itemHandler.item.SetBackground(newRGBA(14, 17, 18, 1))
		}
		ui.QueueMain(func() {
			itemHandler.item.area.Show()
			itemHandler.item.area.QueueRedrawAll()
		})
	}
	for i := len(result); i < len(f.items); i++ {
		item := f.items[i]
		ui.QueueMain(func() {
			item.item.area.Hide()
			item.icon.area.Hide()
		})
	}
	ui.QueueMain(func() {
		f.box.Show()
	})
}

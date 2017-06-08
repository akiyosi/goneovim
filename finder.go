package gonvim

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/dzhou121/neovim-fzf-shim/rplugin/go/fzf"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/svg"
	"github.com/therecipe/qt/widgets"
)

// Finder is a fuzzy finder window
type Finder struct {
	// box         *ui.Box
	// pattern     *SpanHandler
	widget      *widgets.QWidget
	patternText string
	resultItems []*FinderResultItem
	mutex       sync.Mutex
	width       int
	cursor      *widgets.QWidget
	resultType  string
	agTypes     []string
	hidden      bool
	max         int
	showTotal   int
	pattern     *widgets.QLabel
	fzfShim     *fzf.Shim
}

// FinderResultItem is the result shown
type FinderResultItem struct {
	icon *svg.QSvgWidget
	base *widgets.QLabel
	// folder *widgets.QLabel
	widget   *widgets.QWidget
	selected bool
}

// FinderPattern is
type FinderPattern struct {
}

// FinderResult is
type FinderResult struct {
}

func initFinder() *Finder {
	width := 600
	mainLayout := widgets.NewQVBoxLayout()
	mainLayout.SetContentsMargins(0, 0, 0, 0)
	mainLayout.SetSpacing(0)
	widget := widgets.NewQWidget(nil, 0)
	widget.SetLayout(mainLayout)
	widget.SetContentsMargins(0, 0, 0, 0)
	widget.SetFixedWidth(width)
	widget.SetStyleSheet(".QWidget {background-color: rgba(21, 25, 27, 1); } * { color: rgba(205, 211, 222, 1); }")
	shadow := widgets.NewQGraphicsDropShadowEffect(nil)
	shadow.SetBlurRadius(20)
	shadow.SetColor(gui.NewQColor3(0, 0, 0, 255))
	shadow.SetOffset3(0, 2)
	widget.SetGraphicsEffect(shadow)

	resultMainLayout := widgets.NewQHBoxLayout()
	resultMainLayout.SetContentsMargins(0, 0, 0, 0)
	resultMainLayout.SetSpacing(0)

	padding := 8
	resultLayout := widgets.NewQVBoxLayout()
	resultLayout.SetContentsMargins(0, 0, 0, 0)
	resultLayout.SetSpacing(0)
	resultWidget := widgets.NewQWidget(nil, 0)
	resultWidget.SetLayout(resultLayout)
	resultWidget.SetContentsMargins(0, 0, 0, 0)

	scrollCol := widgets.NewQWidget(nil, 0)
	scrollCol.SetContentsMargins(0, 0, 0, 0)
	scrollCol.SetFixedWidth(5)
	scrollBar := widgets.NewQWidget(scrollCol, 0)
	scrollBar.SetFixedWidth(5)
	scrollBar.SetStyleSheet("background-color: rgba(255,255,255,0.5);")

	resultMainWidget := widgets.NewQWidget(nil, 0)
	resultMainWidget.SetContentsMargins(0, 0, 0, 0)
	resultMainLayout.AddWidget(resultWidget, 0, 0)
	resultMainLayout.AddWidget(scrollCol, 0, 0)
	resultMainWidget.SetLayout(resultMainLayout)

	pattern := widgets.NewQLabel(nil, 0)
	pattern.SetContentsMargins(padding, padding, padding, padding)
	pattern.SetStyleSheet("background-color: #3c3c3c;")
	patternLayout := widgets.NewQVBoxLayout()
	patternLayout.AddWidget(pattern, 0, 0)
	patternLayout.SetContentsMargins(0, 0, 0, 0)
	patternLayout.SetSpacing(0)
	patternWidget := widgets.NewQWidget(nil, 0)
	patternWidget.SetLayout(patternLayout)
	patternWidget.SetContentsMargins(padding, padding, padding, padding)

	mainLayout.AddWidget(patternWidget, 0, 0)
	mainLayout.AddWidget(resultMainWidget, 0, 0)

	resultItems := []*FinderResultItem{}
	max := 20
	for i := 0; i < max; i++ {
		itemWidget := widgets.NewQWidget(nil, 0)
		itemWidget.SetContentsMargins(0, 0, 0, 0)
		itemLayout := newVFlowLayout(padding, padding*2)
		itemWidget.SetLayout(itemLayout)
		resultLayout.AddWidget(itemWidget, 0, 0)
		icon := svg.NewQSvgWidget(nil)
		icon.SetFixedWidth(14)
		icon.SetFixedHeight(14)
		icon.SetContentsMargins(0, 0, 0, 0)
		base := widgets.NewQLabel(nil, 0)
		base.SetText("base")
		base.SetContentsMargins(0, padding, 0, padding)
		base.SetStyleSheet("background-color: none; white-space: pre-wrap;")
		// folder := widgets.NewQLabel(nil, 0)
		// folder.SetContentsMargins(0, 10, 0, 10)
		// folder.SetStyleSheet("color: rgba(131, 131, 131, 1);")
		itemLayout.AddWidget(icon)
		itemLayout.AddWidget(base)
		// itemLayout.AddWidget(folder)
		resultItem := &FinderResultItem{
			widget: itemWidget,
			icon:   icon,
			base:   base,
			// folder: folder,
		}
		resultItems = append(resultItems, resultItem)
	}

	// 	box := ui.NewHorizontalBox()
	// 	box.SetSize(width, 500)

	// 	patternHandler := &SpanHandler{}
	// 	pattern := ui.NewArea(patternHandler)
	// 	patternHandler.area = pattern
	// 	patternHandler.paddingLeft = 10
	// 	patternHandler.paddingRight = 10
	// 	patternHandler.paddingTop = 8
	// 	patternHandler.paddingBottom = 8

	// 	cursor := &CursorHandler{}
	// 	cursorArea := ui.NewArea(cursor)
	// 	cursor.area = cursorArea
	// 	cursor.bg = newRGBA(255, 255, 255, 0.9)

	// 	box.Append(pattern, false)
	// 	box.Append(cursorArea, false)
	// 	box.SetShadow(0, 2, 0, 0, 0, 1, 4)
	// 	cursorArea.Hide()
	// 	box.Hide()

	// 	f := &Finder{
	// 		box:     box,
	// 		pattern: patternHandler,
	// 		items:   []*FinderItem{},
	// 		mutex:   &sync.Mutex{},
	// 		width:   width,
	// 		cursor:  cursor,
	// 		hidden:  true,
	// 	}
	// 	return f
	widget.Hide()
	return &Finder{
		width:       width,
		widget:      widget,
		resultItems: resultItems,
		max:         max,
		pattern:     pattern,
	}
}

func (f *Finder) resize() {
	x := (editor.screen.width - f.width) / 2
	f.widget.Move2(x, 0)
	itemHeight := f.resultItems[0].widget.SizeHint().Height()
	f.showTotal = int(float64(editor.screen.height)/float64(itemHeight)*0.6) - 1
	f.fzfShim.SetMax(f.showTotal)

	for i := f.showTotal; i < len(f.resultItems); i++ {
		f.resultItems[i].widget.Hide()
	}
}

func (f *Finder) show() {
	f.hidden = false
	// f.widget.Show()
	// ui.QueueMain(func() {
	// 	f.box.Show()
	// })
}

func (f *Finder) hide() {
	f.hidden = true
	f.widget.Hide()
	// ui.QueueMain(func() {
	// 	f.box.Hide()
	// 	f.pattern.area.Hide()
	// 	f.cursor.area.Hide()
	// })
}

func (f *Finder) cursorPos(args []interface{}) {
	// _, h := f.pattern.getSize()
	// f.cursor.setSize(1, editor.font.lineHeight)
	// p := reflectToInt(args[0])
	// x := int(float64(p)*editor.font.truewidth) + f.pattern.paddingLeft
	// y := (h - editor.font.lineHeight) / 2
	// ui.QueueMain(func() {
	// 	f.cursor.area.Show()
	// 	f.cursor.area.SetPosition(x, y)
	// })
}

func (f *Finder) showSelected(selected int) {
	for i, resultItem := range f.resultItems {
		if i >= f.showTotal {
			break
		}
		if selected == i {
			if !resultItem.selected {
				resultItem.selected = true
				resultItem.widget.SetStyleSheet(fmt.Sprintf(".QWidget {background-color: %s;}", editor.selectedBg))
			}
		} else {
			if resultItem.selected {
				resultItem.selected = false
				resultItem.widget.SetStyleSheet("")
			}
		}
	}
}

func (f *Finder) selectResult(args []interface{}) {
	selected := reflectToInt(args[0])
	f.showSelected(selected)
	// if f.resultType == "ag" {
	// 	n := 0
	// 	for i := 0; i <= selected; i++ {
	// 		for n++; n < len(f.agTypes) && f.agTypes[n] != "ag_line"; n++ {
	// 		}
	// 	}
	// 	selected = n
	// }
	// for i := 0; i < len(f.items); i++ {
	// 	item := f.items[i]
	// 	if selected == i {
	// 		item.item.SetBackground(editor.selectedBg)
	// 		ui.QueueMain(func() {
	// 			item.item.area.QueueRedrawAll()
	// 		})
	// 	} else {
	// 		item.item.SetBackground(newRGBA(14, 17, 18, 1))
	// 		ui.QueueMain(func() {
	// 			item.item.area.QueueRedrawAll()
	// 		})
	// 	}
	// }
}

func (f *Finder) showPattern(args []interface{}) {
	p := args[0].(string)
	f.pattern.SetText(p)
	// _, height := f.pattern.getSize()
	// f.cursor.setSize(1, editor.font.lineHeight)
	// f.pattern.area.SetSize(f.width, height)
	// f.pattern.SetText(p)
	// f.patternText = p
	// f.pattern.SetFont(editor.font)
	// fg := newRGBA(205, 211, 222, 1)
	// f.pattern.SetColor(fg)
	// f.pattern.SetBackground(newRGBA(14, 17, 18, 1))
	// ui.QueueMain(func() {
	// 	// f.box.Show()
	// 	f.cursor.area.Show()
	// 	f.pattern.area.Show()
	// 	f.pattern.area.QueueRedrawAll()
	// })
}

func (f *Finder) rePosition() {
	// x := (editor.width - f.width) / 2
	// ui.QueueMain(func() {
	// 	f.box.SetPosition(x, 0)
	// })
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
	// f.resultType = resultType

	rawItems := args[0].([]interface{})
	for i, resultItem := range f.resultItems {
		if i >= f.showTotal {
			break
		}
		if i >= len(rawItems) {
			resultItem.widget.Hide()
			continue
		}
		item := rawItems[i]
		text := item.(string)
		if resultType == "file" {
			svgContent := getSvg(getFileType(text), nil)
			resultItem.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
			resultItem.icon.Show()
			resultItem.base.SetText(formatText(text, match[i], true))
		} else if resultType == "dir" {
			svgContent := getSvg("folder", nil)
			resultItem.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
			resultItem.icon.Show()
			resultItem.base.SetText(formatText(text, match[i], true))
		} else {
			resultItem.base.SetText(formatText(text, match[i], false))
			resultItem.icon.Hide()
			// resultItem.folder.Hide()
		}
		resultItem.widget.Show()
	}
	f.showSelected(selected)
	f.widget.Hide()
	f.widget.Show()

	// for i, item := range args[0].([]interface{}) {
	// 	text := item.(string)
	// 	resultItem := f.resultItems[i]

	// 	if resultType == "file" {
	// 		svgContent := getSvg(getFileType(text), nil)
	// 		resultItem.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
	// 		resultItem.icon.Show()
	// 		resultItem.base.SetText(formatPath(text, match[i]))
	// 		// dir := filepath.Dir(text)
	// 		// if dir != "." {
	// 		// 	// resultItem.folder.SetText(dir)
	// 		// 	resultItem.folder.SetText(filepath.Dir(text))
	// 		// 	resultItem.folder.Show()
	// 		// } else {
	// 		// 	resultItem.folder.Hide()
	// 		// }
	// 	} else {
	// 		resultItem.base.SetText(text)
	// 		resultItem.icon.Hide()
	// 		resultItem.folder.Hide()
	// 	}
	// }

	// match := [][]int{}
	// for _, i := range args[2].([]interface{}) {
	// 	m := []int{}
	// 	for _, n := range i.([]interface{}) {
	// 		m = append(m, reflectToInt(n))
	// 	}
	// 	match = append(match, m)
	// }

	// resultType := ""
	// if args[3] != nil {
	// 	resultType = args[3].(string)
	// }
	// f.resultType = resultType
	// result := []string{}

	// agLastFile := ""
	// agTypes := []string{}
	// agMatches := [][]int{}
	// for i, item := range args[0].([]interface{}) {
	// 	text := item.(string)
	// 	if resultType == "ag" {
	// 		parts := strings.SplitN(text, ":", 4)
	// 		if len(parts) < 4 {
	// 			continue
	// 		}
	// 		m := match[i]
	// 		file := parts[0]
	// 		if agLastFile != file {
	// 			fileMatch := []int{}
	// 			for n := range m {
	// 				if m[n] < len(parts[0]) {
	// 					fileMatch = append(fileMatch, m[n])
	// 				}
	// 			}
	// 			result = append(result, parts[0])
	// 			agTypes = append(agTypes, "ag_file")
	// 			agLastFile = file
	// 			agMatches = append(agMatches, fileMatch)
	// 		}
	// 		lineIndex := strings.Index(text, parts[3])
	// 		lineMatch := []int{}
	// 		for n := range m {
	// 			if m[n] >= lineIndex {
	// 				lineMatch = append(lineMatch, m[n]-lineIndex)
	// 			}
	// 		}
	// 		result = append(result, parts[3])
	// 		agTypes = append(agTypes, "ag_line")
	// 		agMatches = append(agMatches, lineMatch)
	// 	} else {
	// 		result = append(result, text)
	// 	}
	// }
	// f.agTypes = agTypes
	// paddingLeft := 10
	// paddingTop := 8
	// for i, item := range result {
	// 	if i > len(f.items)-1 {
	// 		height := 8 + 8 + editor.font.height
	// 		width := f.width

	// 		itemHandler := &SpanHandler{}
	// 		itemSpan := ui.NewArea(itemHandler)
	// 		itemHandler.area = itemSpan
	// 		itemHandler.matchColor = editor.matchFg
	// 		itemHandler.paddingLeft = paddingLeft
	// 		itemHandler.paddingRight = paddingLeft
	// 		itemHandler.paddingTop = paddingTop
	// 		itemHandler.paddingBottom = paddingTop
	// 		iconWidth := int(editor.font.truewidth * 2)
	// 		icon := newSvg("default", iconWidth, iconWidth, nil, nil)
	// 		y := height * (i + 1)
	// 		ui.QueueMain(func() {
	// 			f.box.Append(itemSpan, false)
	// 			f.box.Append(icon.area, false)
	// 			itemSpan.SetSize(width, height)
	// 			itemSpan.SetPosition(0, y)
	// 			icon.setPosition(paddingLeft, y+paddingTop)
	// 		})

	// 		f.items = append(f.items, &FinderItem{
	// 			item: itemHandler,
	// 			icon: icon,
	// 		})
	// 	}
	// 	itemHandler := f.items[i]
	// 	itemHandler.item.textType = resultType
	// 	itemHandler.item.SetText(item)
	// 	itemHandler.item.SetFont(editor.font)
	// 	fg := newRGBA(205, 211, 222, 1)
	// 	itemHandler.item.SetColor(fg)
	// 	itemHandler.item.match = f.patternText
	// 	if resultType == "ag" {
	// 		itemHandler.item.textType = agTypes[i]
	// 		itemHandler.item.matchIndex = agMatches[i]
	// 	}
	// 	if resultType != "ag" {
	// 		itemHandler.item.matchIndex = match[i]
	// 	}
	// 	if resultType == "file" {
	// 		itemHandler.icon.name = getFileType(item)
	// 		itemHandler.item.paddingLeft = paddingLeft + itemHandler.icon.width + editor.font.width
	// 		ui.QueueMain(func() {
	// 			itemHandler.icon.area.Show()
	// 			itemHandler.icon.area.QueueRedrawAll()
	// 		})
	// 	} else if resultType == "dir" {
	// 		itemHandler.icon.name = "folder"
	// 		itemHandler.item.paddingLeft = paddingLeft + itemHandler.icon.width + editor.font.width
	// 		ui.QueueMain(func() {
	// 			itemHandler.icon.area.Show()
	// 			itemHandler.icon.area.QueueRedrawAll()
	// 		})
	// 	} else {
	// 		itemHandler.item.paddingLeft = paddingLeft
	// 		ui.QueueMain(func() {
	// 			itemHandler.icon.area.Hide()
	// 		})
	// 	}
	// 	if i == selected {
	// 		itemHandler.item.SetBackground(editor.selectedBg)
	// 	} else {
	// 		itemHandler.item.SetBackground(newRGBA(14, 17, 18, 1))
	// 	}
	// 	ui.QueueMain(func() {
	// 		itemHandler.item.area.Show()
	// 		itemHandler.item.area.QueueRedrawAll()
	// 	})
	// }
	// for i := len(result); i < len(f.items); i++ {
	// 	item := f.items[i]
	// 	ui.QueueMain(func() {
	// 		item.item.area.Hide()
	// 		item.icon.area.Hide()
	// 	})
	// }
	// ui.QueueMain(func() {
	// 	f.box.Show()
	// })
}

func formatText(text string, matchIndex []int, path bool) string {
	sort.Ints(matchIndex)

	color := ""
	if editor != nil && editor.matchFg != nil {
		color = editor.matchFg.Hex()
	}

	match := len(matchIndex) > 0
	if !path {
		formattedText := ""
		for i, char := range text {
			if color != "" && len(matchIndex) > 0 && i == matchIndex[0] {
				formattedText += fmt.Sprintf("<font color='%s'>%s</font>", color, string(char))
				matchIndex = matchIndex[1:]
			} else if color != "" && match && string(char) == " " {
				formattedText += "&nbsp;"
			} else if color != "" && match && string(char) == "\t" {
				formattedText += "&nbsp;&nbsp;&nbsp;&nbsp;"
			} else {
				formattedText += string(char)
			}
		}
		return formattedText
	}

	dirText := ""
	dir := filepath.Dir(text)
	if dir == "." {
		dir = ""
	}
	if dir != "" {
		i := strings.Index(text, dir)
		if i != -1 {
			for j, char := range dir {
				if color != "" && len(matchIndex) > 0 && i+j == matchIndex[0] {
					dirText += fmt.Sprintf("<font color='%s'>%s</font>", color, string(char))
					matchIndex = matchIndex[1:]
				} else {
					dirText += string(char)
				}
			}
		}
	}

	baseText := ""
	base := filepath.Base(text)
	if base != "" {
		i := strings.LastIndex(text, base)
		if i != -1 {
			for j, char := range base {
				if color != "" && len(matchIndex) > 0 && i+j == matchIndex[0] {
					baseText += fmt.Sprintf("<font color='%s'>%s</font>", color, string(char))
					matchIndex = matchIndex[1:]
				} else {
					baseText += string(char)
				}
			}
		}
	}

	return fmt.Sprintf("%s <font color='#838383'>%s</font>", baseText, dirText)
}

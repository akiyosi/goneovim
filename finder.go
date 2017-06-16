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
	widget         *widgets.QWidget
	patternText    string
	resultItems    []*FinderResultItem
	resultWidget   *widgets.QWidget
	itemHeight     int
	mutex          sync.Mutex
	width          int
	cursor         *widgets.QWidget
	cursorX        int
	resultType     string
	agTypes        []string
	max            int
	showTotal      int
	pattern        *widgets.QLabel
	patternPadding int
	scrollBar      *widgets.QWidget
	scrollBarPos   int
	scrollCol      *widgets.QWidget
}

// FinderResultItem is the result shown
type FinderResultItem struct {
	hidden     bool
	icon       *svg.QSvgWidget
	iconType   string
	iconHidden bool
	base       *widgets.QLabel
	baseText   string
	widget     *widgets.QWidget
	selected   bool
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
	scrollBar.SetStyleSheet("background-color: #3c3c3c;")

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

	cursor := widgets.NewQWidget(nil, 0)
	cursor.SetParent(pattern)
	cursor.SetFixedSize2(1, pattern.SizeHint().Height()-padding*2)
	cursor.Move2(padding, padding)
	cursor.SetStyleSheet("background-color: rgba(205, 211, 222, 1);")

	mainLayout.AddWidget(patternWidget, 0, 0)
	mainLayout.AddWidget(resultMainWidget, 0, 0)

	resultItems := []*FinderResultItem{}
	max := 30
	for i := 0; i < max; i++ {
		itemWidget := widgets.NewQWidget(nil, 0)
		itemWidget.SetContentsMargins(0, 0, 0, 0)
		itemLayout := newVFlowLayout(padding, padding*2, 0, 0)
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
		itemLayout.AddWidget(icon)
		itemLayout.AddWidget(base)
		resultItem := &FinderResultItem{
			widget: itemWidget,
			icon:   icon,
			base:   base,
		}
		resultItems = append(resultItems, resultItem)
	}
	finder := &Finder{
		width:          width,
		widget:         widget,
		resultItems:    resultItems,
		resultWidget:   resultWidget,
		max:            max,
		pattern:        pattern,
		patternPadding: padding,
		scrollCol:      scrollCol,
		scrollBar:      scrollBar,
		cursor:         cursor,
	}
	return finder
}

func (f *FinderResultItem) update() {
	if f.selected {
		f.widget.SetStyleSheet(fmt.Sprintf(".QWidget {background-color: %s;}", editor.selectedBg))
	} else {
		f.widget.SetStyleSheet("")
	}
}

func (f *FinderResultItem) setSelected(selected bool) {
	if f.selected == selected {
		return
	}
	f.selected = selected
	f.update()
}

func (f *FinderResultItem) show() {
	if f.hidden {
		f.hidden = false
		f.widget.Show()
	}
}

func (f *FinderResultItem) hide() {
	if !f.hidden {
		f.hidden = true
		f.widget.Hide()
	}
}

func (f *FinderResultItem) updateIcon() {
	svgContent := getSvg(f.iconType, nil)
	f.icon.Load2(core.NewQByteArray2(svgContent, len(svgContent)))
}

func (f *FinderResultItem) showIcon() {
	if f.iconHidden {
		f.iconHidden = false
		f.icon.Show()
	}
}

func (f *FinderResultItem) hideIcon() {
	if !f.iconHidden {
		f.iconHidden = true
		f.icon.Hide()
	}
}

func (f *Finder) show() {
	f.widget.Show()
}

func (f *Finder) hide() {
	f.widget.Hide()
}

func (f *Finder) resize() {
	x := (editor.screen.width - f.width) / 2
	f.widget.Move2(x, 0)
	itemHeight := f.resultItems[0].widget.SizeHint().Height()
	f.itemHeight = itemHeight
	f.showTotal = int(float64(editor.screen.height)/float64(itemHeight)*0.5) - 1
	fzf.UpdateMax(editor.nvim, f.showTotal)

	for i := f.showTotal; i < len(f.resultItems); i++ {
		f.resultItems[i].hide()
	}
}

func (f *Finder) cursorPos(args []interface{}) {
	p := reflectToInt(args[0])
	f.cursorMove(p)
}

func (f *Finder) cursorMove(p int) {
	f.cursorX = int(editor.font.defaultFontMetrics.Width(string(f.patternText[:p])))
	f.cursor.Move2(f.cursorX+f.patternPadding, f.patternPadding)
}

func (f *Finder) showSelected(selected int) {
	for i, resultItem := range f.resultItems {
		if i >= f.showTotal {
			break
		}
		resultItem.setSelected(selected == i)
	}
}

func (f *Finder) selectResult(args []interface{}) {
	selected := reflectToInt(args[0])
	f.showSelected(selected)
}

func (f *Finder) showPattern(args []interface{}) {
	p := args[0].(string)
	f.patternText = p
	f.pattern.SetText(f.patternText)
	f.cursorMove(reflectToInt(args[1]))
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
	// f.resultType = resultType

	rawItems := args[0].([]interface{})
	for i, resultItem := range f.resultItems {
		if i >= f.showTotal {
			break
		}
		if i >= len(rawItems) {
			resultItem.hide()
			continue
		}
		item := rawItems[i]
		text := item.(string)
		if resultType == "file" {
			iconType := getFileType(text)
			if iconType != resultItem.iconType {
				resultItem.iconType = iconType
				resultItem.updateIcon()
			}
			formattedText := formatText(text, match[i], true)
			if formattedText != resultItem.baseText {
				resultItem.baseText = formattedText
				resultItem.base.SetText(resultItem.baseText)
			}
			resultItem.showIcon()
		} else if resultType == "dir" {
			if resultItem.iconType != "folder" {
				resultItem.iconType = "folder"
				resultItem.updateIcon()
			}
			formattedText := formatText(text, match[i], true)
			if formattedText != resultItem.baseText {
				resultItem.baseText = formattedText
				resultItem.base.SetText(resultItem.baseText)
			}
			resultItem.showIcon()
		} else {
			formattedText := formatText(text, match[i], false)
			if formattedText != resultItem.baseText {
				resultItem.baseText = formattedText
				resultItem.base.SetText(resultItem.baseText)
			}
			resultItem.hideIcon()
		}
		resultItem.show()
	}
	f.showSelected(selected)

	start := reflectToInt(args[4])
	total := reflectToInt(args[5])

	// if len(rawItems) == f.showTotal {
	// 	f.scrollCol.Show()
	// } else {
	// 	f.scrollCol.Hide()
	// }
	f.resultWidget.Hide()
	f.resultWidget.Show()

	if total > f.showTotal {
		height := int(float64(f.showTotal) / float64(total) * float64(f.itemHeight*f.showTotal))
		if height == 0 {
			height = 1
		}
		f.scrollBar.SetFixedHeight(height)
		f.scrollBarPos = int(float64(start) / float64(total) * (float64(f.itemHeight * f.showTotal)))
		f.scrollBar.Move2(0, f.scrollBarPos)
		f.scrollCol.Show()
	} else {
		f.scrollCol.Hide()
	}

	f.hide()
	f.show()
	f.hide()
	f.show()
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

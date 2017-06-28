package gonvim

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// Finder is a fuzzy finder window
type Finder struct {
}

func initFinder() *Finder {
	return &Finder{}
}

func (f *Finder) hide() {
	editor.palette.hide()
}

func (f *Finder) cursorPos(args []interface{}) {
	x := reflectToInt(args[0])
	editor.palette.cursorMove(x)
}

func (f *Finder) selectResult(args []interface{}) {
	selected := reflectToInt(args[0])
	editor.palette.showSelected(selected)
}

func (f *Finder) showPattern(args []interface{}) {
	palette := editor.palette
	p := args[0].(string)
	palette.patternText = p
	palette.pattern.SetText(palette.patternText)
	palette.cursorMove(reflectToInt(args[1]))
}

func (f *Finder) showResult(args []interface{}) {
	palette := editor.palette
	palette.mutex.Lock()
	defer palette.mutex.Unlock()
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
	results := []string{}
	palette.resultType = resultType

	rawItems := args[0].([]interface{})

	lastFile := ""
	itemTypes := []string{}
	itemMatches := [][]int{}
	for i, item := range rawItems {
		text := item.(string)
		if resultType == "file_line" {
			parts := strings.SplitN(text, ":", 2)
			if len(parts) < 2 {
				continue
			}
			m := match[i]
			file := parts[0]
			if lastFile != file {
				fileMatch := []int{}
				for n := range m {
					if m[n] < len(parts[0]) {
						fileMatch = append(fileMatch, m[n])
					}
				}
				results = append(results, parts[0])
				itemTypes = append(itemTypes, "file")
				lastFile = file
				itemMatches = append(itemMatches, fileMatch)
			}
			line := parts[len(parts)-1]
			lineIndex := strings.Index(text, line)
			lineMatch := []int{}
			for n := range m {
				if m[n] >= lineIndex {
					lineMatch = append(lineMatch, m[n]-lineIndex)
				}
			}
			results = append(results, line)
			itemTypes = append(itemTypes, "file_line")
			itemMatches = append(itemMatches, lineMatch)
		} else if resultType == "buffer" {
			n := strings.Index(text, "]")
			if n > -1 {
				text = text[n+1:]
			}
			results = append(results, text)
		} else {
			results = append(results, text)
		}
	}
	palette.itemTypes = itemTypes

	for i, resultItem := range palette.resultItems {
		if i >= len(results) {
			resultItem.hide()
			continue
		}
		text := results[i]
		if resultType == "file" {
			resultItem.setItem(text, "file", match[i])
		} else if resultType == "buffer" {
			resultItem.setItem(text, "file", match[i])
		} else if resultType == "dir" {
			resultItem.setItem(text, "dir", match[i])
		} else if resultType == "file_line" {
			resultItem.setItem(text, itemTypes[i], itemMatches[i])
		} else {
			resultItem.setItem(text, "", match[i])
		}
		resultItem.show()
	}
	palette.showSelected(selected)

	start := reflectToInt(args[4])
	total := reflectToInt(args[5])

	// if len(rawItems) == f.showTotal {
	// 	f.scrollCol.Show()
	// } else {
	// 	f.scrollCol.Hide()
	// }

	if total > palette.showTotal {
		height := int(float64(palette.showTotal) / float64(total) * float64(palette.itemHeight*palette.showTotal))
		if height == 0 {
			height = 1
		}
		palette.scrollBar.SetFixedHeight(height)
		palette.scrollBarPos = int(float64(start) / float64(total) * (float64(palette.itemHeight * palette.showTotal)))
		palette.scrollBar.Move2(0, palette.scrollBarPos)
		palette.scrollCol.Show()
	} else {
		palette.scrollCol.Hide()
	}

	palette.refresh()
}

func formatText(text string, matchIndex []int, path bool) string {
	sort.Ints(matchIndex)

	color := ""
	if editor != nil && editor.matchFg != nil {
		color = editor.matchFg.Hex()
	}

	match := len(matchIndex) > 0
	if !path || strings.HasPrefix(text, "term://") {
		formattedText := ""
		i := 0
		for _, char := range text {
			if color != "" && len(matchIndex) > 0 && i == matchIndex[0] {
				formattedText += fmt.Sprintf("<font color='%s'>%s</font>", color, string(char))
				matchIndex = matchIndex[1:]
			} else if color != "" && match {
				switch string(char) {
				case " ":
					formattedText += "&nbsp;"
				case "\t":
					formattedText += "&nbsp;&nbsp;&nbsp;&nbsp;"
				case "<":
					formattedText += "&lt;"
				case ">":
					formattedText += "&gt;"
				default:
					formattedText += string(char)
				}
			} else {
				formattedText += string(char)
			}
			i++
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

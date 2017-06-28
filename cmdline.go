package gonvim

import (
	"fmt"
	"strings"
)

// CmdContent is the content of the cmdline
type CmdContent struct {
	indent  int
	firstc  string
	prompt  string
	content string
}

// Cmdline is the cmdline
type Cmdline struct {
	pos           int
	content       *CmdContent
	preContent    *CmdContent
	function      []*CmdContent
	inFunction    bool
	rawItems      []interface{}
	wildmenuShown bool
	top           int
}

func initCmdline() *Cmdline {
	return &Cmdline{
		content:  &CmdContent{},
		rawItems: make([]interface{}, 0),
	}
}

func (c *CmdContent) getText() string {
	indentStr := ""
	for i := 0; i < c.indent; i++ {
		indentStr += " "
	}
	return fmt.Sprintf("%s%s", indentStr, c.content)
}

func (c *Cmdline) getText(ch string) string {
	indentStr := ""
	for i := 0; i < c.content.indent; i++ {
		indentStr += " "
	}
	return fmt.Sprintf("%s%s%s", c.content.firstc, indentStr, c.content.content[:c.pos]+ch+c.content.content[c.pos:])
}

func (c *Cmdline) show(args []interface{}) {
	arg := args[0].([]interface{})
	content := arg[0].([]interface{})[0].([]interface{})[1].(string)
	pos := reflectToInt(arg[1])
	firstc := arg[2].(string)
	prompt := arg[3].(string)
	indent := reflectToInt(arg[4])
	// level := reflectToInt(arg[5])
	// fmt.Println("cmdline show", content, pos, firstc, prompt, indent, level)

	c.pos = pos
	c.content.firstc = firstc
	c.content.content = content
	c.content.indent = indent
	c.content.prompt = prompt
	text := c.getText("")
	palette := editor.palette
	palette.setPattern(text)
	c.cursorMove()
	if !c.wildmenuShown {
		c.showAddition()
		palette.scrollCol.Hide()
	}
	palette.refresh()
}

func (c *Cmdline) showAddition() {
	lines := append(c.getPromptLines(), c.getFunctionLines()...)
	palette := editor.palette
	for i, resultItem := range palette.resultItems {
		if i >= len(lines) || i >= palette.showTotal {
			resultItem.hide()
			continue
		}
		resultItem.setItem(lines[i], "", []int{})
		resultItem.setSelected(false)
		resultItem.show()
	}
}

func (c *Cmdline) getPromptLines() []string {
	result := []string{}
	if c.content.prompt == "" {
		return result
	}

	lines := strings.Split(c.content.prompt, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		result = append(result, line)
	}
	return result
}

func (c *Cmdline) getFunctionLines() []string {
	result := []string{}
	if !c.inFunction {
		return result
	}

	for _, content := range c.function {
		result = append(result, content.getText())
	}
	return result
}

func (c *Cmdline) cursorMove() {
	editor.palette.cursorMove(c.pos + len(c.content.firstc) + c.content.indent)
}

func (c *Cmdline) hide(args []interface{}) {
	palette := editor.palette
	palette.hide()
	if c.inFunction {
		c.function = append(c.function, c.content)
	}
	c.preContent = c.content
	c.content = &CmdContent{}
}

func (c *Cmdline) functionShow() {
	c.inFunction = true
	c.function = []*CmdContent{c.preContent}
}

func (c *Cmdline) functionHide() {
	c.inFunction = false
}

func (c *Cmdline) changePos(args []interface{}) {
	args = args[0].([]interface{})
	pos := reflectToInt(args[0])
	// level := reflectToInt(args[1])
	// fmt.Println("change pos", pos, level)
	c.pos = pos
	c.cursorMove()
}

func (c *Cmdline) putChar(args []interface{}) {
	args = args[0].([]interface{})
	ch := args[0].(string)
	// shift := reflectToInt(args[1])
	// level := reflectToInt(args[2])
	// fmt.Println("putChar", ch, shift, level)
	text := c.getText(ch)
	palette := editor.palette
	palette.setPattern(text)
}

func (c *Cmdline) wildmenuShow(args []interface{}) {
	c.wildmenuShown = true
	args = args[0].([]interface{})
	c.rawItems = args[0].([]interface{})
	palette := editor.palette
	c.top = 0
	for i := 0; i < palette.showTotal; i++ {
		resultItem := palette.resultItems[i]
		if i >= len(c.rawItems) {
			resultItem.hide()
			continue
		}
		text := c.rawItems[i].(string)
		resultItem.setItem(text, "", []int{})
		resultItem.show()
		resultItem.setSelected(false)
	}

	total := len(c.rawItems)
	if total > palette.showTotal {
		height := int(float64(palette.showTotal) / float64(total) * float64(palette.itemHeight*palette.showTotal))
		if height == 0 {
			height = 1
		}
		palette.scrollBar.SetFixedHeight(height)
		palette.scrollBarPos = 0
		palette.scrollBar.Move2(0, palette.scrollBarPos)
		palette.scrollCol.Show()
	} else {
		palette.scrollCol.Hide()
	}
}

func (c *Cmdline) wildmenuSelect(args []interface{}) {
	selected := reflectToInt(args[0].([]interface{})[0])
	// fmt.Println("selected is", selected)
	showTotal := editor.palette.showTotal
	if selected == -1 && c.top > 0 {
		c.wildmenuScroll(-c.top)
	}
	if selected-c.top >= showTotal {
		c.wildmenuScroll(selected - c.top - showTotal + 1)
	}
	if selected >= 0 && selected-c.top < 0 {
		c.wildmenuScroll(-1)
	}
	palette := editor.palette
	for i := 0; i < palette.showTotal; i++ {
		item := palette.resultItems[i]
		item.setSelected(selected == i+c.top)
	}
}

func (c *Cmdline) wildmenuScroll(n int) {
	c.top += n
	palette := editor.palette
	for i := 0; i < palette.showTotal; i++ {
		resultItem := palette.resultItems[i]
		if i >= len(c.rawItems) {
			resultItem.hide()
			continue
		}
		text := c.rawItems[i+c.top].(string)
		resultItem.setItem(text, "", []int{})
		resultItem.show()
		resultItem.setSelected(false)
	}
	palette.scrollBarPos = int((float64(c.top) / float64(len(c.rawItems))) * float64(palette.itemHeight*palette.showTotal))
	palette.scrollBar.Move2(0, palette.scrollBarPos)
}

func (c *Cmdline) wildmenuHide() {
	c.wildmenuShown = false
}

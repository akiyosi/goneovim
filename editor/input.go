package editor

import (
	"fmt"
	"runtime"
	"strings"
	"unicode"

	"github.com/akiyosi/qt/core"
	"github.com/akiyosi/qt/gui"
)

func (e *Editor) keyRelease(event *gui.QKeyEvent) {
	if !e.isKeyAutoRepeating {
		return
	}

	// if editor.config.Editor.SmoothScroll {
	// 	ws := e.workspaces[e.active]
	// 	win, ok := ws.screen.getWindow(ws.cursor.gridid)
	// 	if !ok {
	// 		return
	// 	}
	// 	if win.scrollPixels2 != 0 {
	// 		return
	// 	}
	// 	win.grabScreenSnapshot()
	// }

	e.isKeyAutoRepeating = false
}

func (e *Editor) keyPress(event *gui.QKeyEvent) {
	if len(e.workspaces) <= e.active {
		return
	}
	ws := e.workspaces[e.active]
	if ws.nvim == nil {
		return
	}
	if event.IsAutoRepeat() {
		e.isKeyAutoRepeating = true
	}
	// if !e.isKeyAutoRepeating {
	// 	ws.getSnapshot()
	// }
	if !e.isHideMouse && e.config.Editor.HideMouseWhenTyping {
		bc := gui.NewQCursor2(core.Qt__BlankCursor)
		gui.QGuiApplication_SetOverrideCursor(bc)
		gui.QGuiApplication_ChangeOverrideCursor(bc)
		e.isHideMouse = true
	}

	input := e.convertKey(event)

	e.putLog("key input for nvim::", "input:", input)
	if input != "" {
		ws.nvim.Input(input)
	}
}

func keyToText(key int, mod core.Qt__KeyboardModifier) string {
	text := string(rune(key))
	if !(mod&core.Qt__ShiftModifier > 0) {
		text = strings.ToLower(text)
	}

	return text
}

// controlModifier is
func controlModifier() (controlModifier core.Qt__KeyboardModifier) {
	if runtime.GOOS == "windows" {
		controlModifier = core.Qt__ControlModifier
	}
	if runtime.GOOS == "linux" {
		controlModifier = core.Qt__ControlModifier
	}
	if runtime.GOOS == "darwin" {
		controlModifier = core.Qt__MetaModifier
	}

	return
}

// cmdModifier is
func cmdModifier() (cmdModifier core.Qt__KeyboardModifier) {
	if runtime.GOOS == "windows" {
		cmdModifier = core.Qt__NoModifier
	}
	if runtime.GOOS == "linux" {
		cmdModifier = core.Qt__MetaModifier
	}
	if runtime.GOOS == "darwin" {
		cmdModifier = core.Qt__ControlModifier
	}

	return
}

func isAsciiCharRequiringAlt(key int, mod core.Qt__KeyboardModifier, c rune) bool {
	// Ignore all key events where Alt is not pressed
	if !(mod&core.Qt__AltModifier > 0) {
		return false
	}

	// These low-ascii characters may require AltModifier on MacOS
	if (c == '[' /* && key != int(core.Qt__Key_BracketLeft) */) ||
		(c == ']' && key != int(core.Qt__Key_BracketRight)) ||
		(c == '{' /* && key != int(core.Qt__Key_BraceLeft) */) ||
		(c == '}' && key != int(core.Qt__Key_BraceRight)) ||
		(c == '|' && key != int(core.Qt__Key_Bar)) ||
		(c == '~' && key != int(core.Qt__Key_AsciiTilde)) ||
		(c == '@' && key != int(core.Qt__Key_At)) ||
		(c == '#' && key != int(core.Qt__Key_NumberSign)) {
		return true
	}

	return false
}

func isControlCaretKeyEvent(key int, mod core.Qt__KeyboardModifier, text string) bool {
	if key != int(core.Qt__Key_6) && key != int(core.Qt__Key_AsciiCircum) {
		return false
	}

	if !(mod&controlModifier() > 0) {
		return false
	}

	if text != "\u001E" && text != "^" && text != "6" && text != "" {
		return false
	}

	return true
}

func (e *Editor) convertKey(event *gui.QKeyEvent) string {
	text := event.Text()
	key := event.Key()
	mod := event.Modifiers()

	if e.opts.Debug != "" {
		e.putLog("key input(raw)::", fmt.Sprintf("text: %s, key: %d, mod: %v", text, key, mod))
	}

	// this is macmeta alternatively
	if e.config.Editor.Macmeta {
		if mod&core.Qt__AltModifier > 0 && mod&core.Qt__ShiftModifier > 0 {
			text = string(rune(key))
		} else if mod&core.Qt__AltModifier > 0 && !(mod&core.Qt__ShiftModifier > 0) {
			text = strings.ToLower(string(rune(key)))
		}
	}

	keypadKeys := map[core.Qt__Key]string{
		core.Qt__Key_Home:     "kHome",
		core.Qt__Key_End:      "kEnd",
		core.Qt__Key_PageUp:   "kPageUp",
		core.Qt__Key_PageDown: "kPageDown",
		core.Qt__Key_Plus:     "kPlus",
		core.Qt__Key_Minus:    "kMinus",
		core.Qt__Key_multiply: "kMultiply",
		core.Qt__Key_division: "kDivide",
		core.Qt__Key_Enter:    "kEnter",
		core.Qt__Key_Period:   "kPoint",
		core.Qt__Key_0:        "k0",
		core.Qt__Key_1:        "k1",
		core.Qt__Key_2:        "k2",
		core.Qt__Key_3:        "k3",
		core.Qt__Key_4:        "k4",
		core.Qt__Key_5:        "k5",
		core.Qt__Key_6:        "k6",
		core.Qt__Key_7:        "k7",
		core.Qt__Key_8:        "k8",
		core.Qt__Key_9:        "k9",
	}

	if mod&core.Qt__KeypadModifier > 0 {
		if value, ok := keypadKeys[core.Qt__Key(key)]; ok {
			return fmt.Sprintf("<%s%s>", e.modPrefix(mod), value)
		}
	}

	if key == int(core.Qt__Key_Space) && len([]rune(text)) > 0 && !unicode.IsPrint([]rune(text)[0]) {
		text = " "
	}

	if key == int(core.Qt__Key_Space) && len([]rune(text)) > 0 && text != " " {
		if mod != core.Qt__NoModifier {
			return fmt.Sprintf("<%s%s>", e.modPrefix(mod), text)
		}
	}

	specialKey, ok := e.specialKeys[core.Qt__Key(key)]
	if ok {
		if key == int(core.Qt__Key_Space) && text != " " {
			return text
		}
		if key == int(core.Qt__Key_Space) || key == int(core.Qt__Key_Backspace) {
			mod &= ^core.Qt__ShiftModifier
		}
		return fmt.Sprintf("<%s%s>", e.modPrefix(mod), specialKey)
	}

	if text == "<" {
		modNoShift := mod & ^core.Qt__ShiftModifier
		return fmt.Sprintf("<%s%s>", e.modPrefix(modNoShift), "lt")
	}

	// neovim-qt issue#720
	if key == int(core.Qt__Key_AsciiCircum) && text == "[" {
		modNoAlt := mod & ^core.Qt__AltModifier
		if modNoAlt == core.Qt__NoModifier {
			return "["
		}

		return fmt.Sprintf("<%s%s>", e.modPrefix(modNoAlt), "[")
	}

	// Normalize modifiers, CTRL+^ always sends as <C-^>
	if isControlCaretKeyEvent(key, mod, text) {
		modNoShiftMeta := mod & ^core.Qt__ShiftModifier & ^cmdModifier()
		return fmt.Sprintf("<%s%s>", e.modPrefix(modNoShiftMeta), "^")
	}

	if text == "\\" {
		return fmt.Sprintf("<%s%s>", e.modPrefix(mod), "Bslash")
	}

	rtext := []rune(text)
	isGraphic := false
	if len(rtext) > 0 {
		isGraphic = unicode.IsGraphic(rtext[0])
	}
	if text == "" || !isGraphic {
		if key == int(core.Qt__Key_Alt) ||
			key == int(core.Qt__Key_AltGr) ||
			key == int(core.Qt__Key_CapsLock) ||
			key == int(core.Qt__Key_Control) ||
			key == int(core.Qt__Key_Meta) ||
			key == int(core.Qt__Key_Shift) ||
			key == int(core.Qt__Key_Super_L) ||
			key == int(core.Qt__Key_Super_R) {
			return ""
		}

		// Ignore special keys
		if key == int(core.Qt__Key_VolumeDown) ||
			key == int(core.Qt__Key_VolumeMute) ||
			key == int(core.Qt__Key_VolumeUp) {
			return ""
		}

		text = keyToText(key, mod)
	}
	c := text
	if c == "" {
		return ""
	}

	char := []rune(c)[0]
	// char := core.NewQChar11(c)

	// Remove SHIFT
	if (int(char) >= 0x80 || unicode.IsPrint(char)) && !(mod&controlModifier() > 0) && !(mod&cmdModifier() > 0) {
		mod &= ^core.Qt__ShiftModifier
	}
	// if (char.Unicode() >= 0x80 || char.IsPrint()) && !(mod&controlModifier() > 0) && !(mod&cmdModifier() > 0) {
	// 	mod &= ^core.Qt__ShiftModifier
	// }

	// Remove CTRL
	if int(char) < 0x20 {
		text = keyToText(key, mod)
	}
	// if char.Unicode() < 0x20 {
	// 	text = keyToText(key, mod)
	// }

	if runtime.GOOS == "darwin" {
		// Remove ALT/OPTION
		if int(char) >= 0x80 && unicode.IsPrint(char) {
			mod &= ^core.Qt__AltModifier
		}
		// if char.Unicode() >= 0x80 && char.IsPrint() {
		// 	mod &= ^core.Qt__AltModifier
		// }

		// Some locales require Alt for basic low-ascii characters,
		// remove AltModifer. Ex) German layouts use Alt for "{".
		if isAsciiCharRequiringAlt(key, mod, []rune(c)[0]) {
			mod &= ^core.Qt__AltModifier
		}

	}

	prefix := e.modPrefix(mod)
	if prefix != "" {
		return fmt.Sprintf("<%s%s>", prefix, c)
	}

	return c
}

func (e *Editor) modPrefix(mod core.Qt__KeyboardModifier) string {
	prefix := ""
	if runtime.GOOS == "windows" {
		if mod&controlModifier() > 0 && !(mod&core.Qt__AltModifier > 0) {
			prefix += "C-"
		}
		if mod&core.Qt__ShiftModifier > 0 {
			prefix += "S-"
		}
		if mod&core.Qt__AltModifier > 0 && !(mod&controlModifier() > 0) {
			prefix += e.prefixToMapMetaKey
		}
	} else {
		if mod&cmdModifier() > 0 {
			prefix += "D-"
		}
		if mod&controlModifier() > 0 {
			prefix += "C-"
		}
		if mod&core.Qt__ShiftModifier > 0 {
			prefix += "S-"
		}
		if mod&core.Qt__AltModifier > 0 {
			prefix += e.prefixToMapMetaKey
		}
	}

	return prefix
}

func (e *Editor) initSpecialKeys() {
	e.specialKeys = map[core.Qt__Key]string{}
	e.specialKeys[core.Qt__Key_Up] = "Up"
	e.specialKeys[core.Qt__Key_Down] = "Down"
	e.specialKeys[core.Qt__Key_Left] = "Left"
	e.specialKeys[core.Qt__Key_Right] = "Right"

	e.specialKeys[core.Qt__Key_F1] = "F1"
	e.specialKeys[core.Qt__Key_F2] = "F2"
	e.specialKeys[core.Qt__Key_F3] = "F3"
	e.specialKeys[core.Qt__Key_F4] = "F4"
	e.specialKeys[core.Qt__Key_F5] = "F5"
	e.specialKeys[core.Qt__Key_F6] = "F6"
	e.specialKeys[core.Qt__Key_F7] = "F7"
	e.specialKeys[core.Qt__Key_F8] = "F8"
	e.specialKeys[core.Qt__Key_F9] = "F9"
	e.specialKeys[core.Qt__Key_F10] = "F10"
	e.specialKeys[core.Qt__Key_F11] = "F11"
	e.specialKeys[core.Qt__Key_F12] = "F12"
	e.specialKeys[core.Qt__Key_F13] = "F13"
	e.specialKeys[core.Qt__Key_F14] = "F14"
	e.specialKeys[core.Qt__Key_F15] = "F15"
	e.specialKeys[core.Qt__Key_F16] = "F16"
	e.specialKeys[core.Qt__Key_F17] = "F17"
	e.specialKeys[core.Qt__Key_F18] = "F18"
	e.specialKeys[core.Qt__Key_F19] = "F19"
	e.specialKeys[core.Qt__Key_F20] = "F20"
	e.specialKeys[core.Qt__Key_F21] = "F21"
	e.specialKeys[core.Qt__Key_F22] = "F22"
	e.specialKeys[core.Qt__Key_F23] = "F23"
	e.specialKeys[core.Qt__Key_F24] = "F24"
	e.specialKeys[core.Qt__Key_Backspace] = "BS"
	e.specialKeys[core.Qt__Key_Delete] = "Del"
	e.specialKeys[core.Qt__Key_Insert] = "Insert"
	e.specialKeys[core.Qt__Key_Home] = "Home"
	e.specialKeys[core.Qt__Key_End] = "End"
	e.specialKeys[core.Qt__Key_PageUp] = "PageUp"
	e.specialKeys[core.Qt__Key_PageDown] = "PageDown"
	e.specialKeys[core.Qt__Key_Return] = "Enter"
	e.specialKeys[core.Qt__Key_Enter] = "Enter"
	e.specialKeys[core.Qt__Key_Tab] = "Tab"
	e.specialKeys[core.Qt__Key_Backtab] = "Tab"
	e.specialKeys[core.Qt__Key_Escape] = "Esc"
	e.specialKeys[core.Qt__Key_Backslash] = "Bslash"
	e.specialKeys[core.Qt__Key_Space] = "Space"

	if runtime.GOOS == "darwin" {
		e.keyControl = core.Qt__Key_Meta
		e.keyCmd = core.Qt__Key_Control
	} else if runtime.GOOS == "windows" {
		e.keyControl = core.Qt__Key_Control
		e.keyCmd = (core.Qt__Key)(0)
	} else {
		e.keyControl = core.Qt__Key_Control
		e.keyCmd = core.Qt__Key_Meta
	}

	e.prefixToMapMetaKey = "A-"
}

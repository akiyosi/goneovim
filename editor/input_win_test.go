//go:build windows
// +build windows

package editor

import (
	"fmt"
	"testing"

	"github.com/akiyosi/qt/core"
	"github.com/akiyosi/qt/gui"
)

func TestWindowsEditor_convertKey(t *testing.T) {
	tests := []struct {
		name string
		args *gui.QKeyEvent
		want string
	}{
		{
			`convertKey() Windows LessThan modifier keys 1`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Less), core.Qt__ShiftModifier|core.Qt__ControlModifier, "<", false, 1),
			"<C-lt>",
		},
		{
			`convertKey() Windows LessThan modifier keys 2`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Less), core.Qt__ShiftModifier|core.Qt__AltModifier, "<", false, 1),
			"<A-lt>",
		},
		{
			`convertKey() Windows LessThan modifier keys 3`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Less), core.Qt__ShiftModifier|core.Qt__MetaModifier, "<", false, 1),
			"<lt>",
		},
		{
			`convertKey() Windows Ctrl Caret WellFormed 1`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_6), core.Qt__ControlModifier, "6", false, 1),
			"<C-^>",
		},
		{
			`convertKey() Windows Ctrl Caret WellFormed 2`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_AsciiCircum), core.Qt__ShiftModifier|core.Qt__ControlModifier, "\u001E", false, 1),
			"<C-^>",
		},
		{
			`convertKey() Windows ShiftModifier Letter 1`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_B), core.Qt__ControlModifier, "\u0002", false, 1),
			"<C-b>",
		},
		{
			`convertKey() Windows ShiftModifier Letter 2`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_B), core.Qt__ShiftModifier|core.Qt__ControlModifier, "\u0002", false, 1),
			"<C-S-B>",
		},
		{
			`convertKey() Windows German keyboardlayout 1`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_BraceLeft), core.Qt__ControlModifier|core.Qt__AltModifier, "{", false, 1),
			"{",
		},
		{
			`convertKey() Windows German keyboardlayout 2`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_BracketLeft), core.Qt__ControlModifier|core.Qt__AltModifier, "[", false, 1),
			"[",
		},
		{
			`convertKey() Windows German keyboardlayout 3`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_BracketRight), core.Qt__ControlModifier|core.Qt__AltModifier, "]", false, 1),
			"]",
		},
		{
			`convertKey() Windows German keyboardlayout 4`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_BraceRight), core.Qt__ControlModifier|core.Qt__AltModifier, "}", false, 1),
			"}",
		},
		{
			`convertKey() Windows German keyboardlayout 5`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_At), core.Qt__ControlModifier|core.Qt__AltModifier, "@", false, 1),
			"@",
		},
		{
			`convertKey() Windows German keyboardlayout 6`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Backslash), core.Qt__ControlModifier|core.Qt__AltModifier, "\\", false, 1),
			"<Bslash>",
		},
		{
			`convertKey() Windows German keyboardlayout 7`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_AsciiTilde), core.Qt__ControlModifier|core.Qt__AltModifier, "~", false, 1),
			"~",
		},
		{
			`convertKey() Windows German keyboardlayout 8`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_8), core.Qt__AltModifier, "8", false, 1),
			"<A-8>",
		},
		{
			`convertKey() Windows Spanish keyboardlayout 1`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Space), core.Qt__NoModifier, "`", false, 1),
			"`",
		},
		// TODO  Windows ``: two events are sent on the second key event. Prints: ``
		{
			`convertKey() Windows Spanish keyboardlayout 3`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_AsciiCircum), core.Qt__AltModifier, "[", false, 1),
			"[",
		},
		{
			`convertKey() Windows Spanish keyboardlayout 4`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Space), core.Qt__NoModifier, "^", false, 1),
			"^",
		},

		{
			`convertKey() Windows Spanish keyboardlayout 4`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_AsciiCircum), core.Qt__ShiftModifier, "^", false, 1),
			"^",
		},
		{
			`convertKey() Windows Spanish keyboardlayout 5`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, 0, core.Qt__ShiftModifier, "^", false, 1),
			"^",
		},
		{
			`convertKey() Windows Spanish keyboardlayout 6`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_E), core.Qt__NoModifier, "ê", false, 1),
			"ê",
		},
	}
	e := &Editor{}
	e.InitSpecialKeys()
	for key, value := range e.specialKeys {

		text := ""
		if key == core.Qt__Key_Space {
			text = " "
		}
		tests = append(
			tests,
			[]struct {
				name string
				args *gui.QKeyEvent
				want string
			}{
				{
					`convertKey() Windows special keys`,
					gui.NewQKeyEvent(core.QEvent__KeyPress, int(key), core.Qt__NoModifier, text, false, 1),
					fmt.Sprintf("<%s>", value),
				},
				{
					`convertKey() Windows special keys with Ctrl`,
					gui.NewQKeyEvent(core.QEvent__KeyPress, int(key), core.Qt__ControlModifier, text, false, 1),
					fmt.Sprintf("<C-%s>", value),
				},
				{
					`convertKey() Windows special keys with Alt`,
					gui.NewQKeyEvent(core.QEvent__KeyPress, int(key), core.Qt__AltModifier, text, false, 1),
					fmt.Sprintf("<A-%s>", value),
				},
				{
					`convertKey() Windows special keys with Meta`,
					gui.NewQKeyEvent(core.QEvent__KeyPress, int(key), core.Qt__MetaModifier, text, false, 1),
					fmt.Sprintf("<%s>", value),
				},
			}...,
		)

	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if got := e.convertKey(tt.args); got != tt.want {
				t.Errorf("Editor.convertKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

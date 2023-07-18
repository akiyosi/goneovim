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
	}
	e := &Editor{}
	e.InitSpecialKeys()
	for key, value := range e.specialKeys {
		tests = append(
			tests,
			[]struct {
				name string
				args *gui.QKeyEvent
				want string
			}{
				{
					`convertKey() Windows special keys`,
					gui.NewQKeyEvent(core.QEvent__KeyPress, int(key), core.Qt__NoModifier, value, false, 1),
					fmt.Sprintf("<%s>", value),
				},
				{
					`convertKey() Windows special keys with Ctrl`,
					gui.NewQKeyEvent(core.QEvent__KeyPress, int(key), core.Qt__ControlModifier, value, false, 1),
					fmt.Sprintf("<C-%s>", value),
				},
				{
					`convertKey() Windows special keys with Alt`,
					gui.NewQKeyEvent(core.QEvent__KeyPress, int(key), core.Qt__AltModifier, value, false, 1),
					fmt.Sprintf("<A-%s>", value),
				},
				{
					`convertKey() Windows special keys with Meta`,
					gui.NewQKeyEvent(core.QEvent__KeyPress, int(key), core.Qt__MetaModifier, value, false, 1),
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

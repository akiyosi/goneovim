// +build linux

package editor

import (
	"fmt"
	"testing"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
)

func TestLinuxEditor_convertKey(t *testing.T) {
	tests := []struct {
		name string
		args *gui.QKeyEvent
		want string
	}{
		{
			`convertKey() Linux LessThan modifier keys 1`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Less), core.Qt__ShiftModifier|core.Qt__ControlModifier, "<", false, 1),
			"<C-lt>",
		},
		{
			`convertKey() Linux LessThan modifier keys 2`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Less), core.Qt__ShiftModifier|core.Qt__AltModifier, "<", false, 1),
			"<A-lt>",
		},
		{
			`convertKey() Linux LessThan modifier keys 3`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Less), core.Qt__ShiftModifier|core.Qt__MetaModifier, "<", false, 1),
			"<D-lt>",
		},
		{
			`convertKey() Linux Ctrl Caret WellFormed 1`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_6), core.Qt__ControlModifier, string('\u001E'), false, 1),
			"<C-^>",
		},
		{
			`convertKey() Linux Ctrl Caret WellFormed 2`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_AsciiCircum), core.Qt__ShiftModifier|core.Qt__ControlModifier, string('\u001E'), false, 1),
			"<C-^>",
		},
		{
			`convertKey() Linux Ctrl Caret WellFormed 3`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_AsciiCircum), core.Qt__ShiftModifier|core.Qt__ControlModifier|core.Qt__MetaModifier, string('\u001E'), false, 1),
			"<C-^>",
		},
		{
			`convertKey() Linux ShiftModifier Letter 1`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_B), core.Qt__ControlModifier, string('\u0002'), false, 1),
			"<C-b>",
		},
		{
			`convertKey() Linux ShiftModifier Letter 2`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_B), core.Qt__ShiftModifier|core.Qt__ControlModifier, string('\u0002'), false, 1),
			"<C-S-B>",
		},
		{
			`convertKey() Linux German keyboardlayout 1`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_BraceLeft), core.Qt__GroupSwitchModifier, "{", false, 1),
			"{",
		},
		{
			`convertKey() Linux German keyboardlayout 2`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_BracketLeft), core.Qt__GroupSwitchModifier, "[", false, 1),
			"[",
		},
		{
			`convertKey() Linux German keyboardlayout 3`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_BracketRight), core.Qt__GroupSwitchModifier, "]", false, 1),
			"]",
		},
		{
			`convertKey() Linux German keyboardlayout 4`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_BraceRight), core.Qt__GroupSwitchModifier, "}", false, 1),
			"}",
		},
		{
			`convertKey() Linux German keyboardlayout 5`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_At), core.Qt__GroupSwitchModifier, "@", false, 1),
			"@",
		},
		{
			`convertKey() Linux German keyboardlayout 6`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Backslash), core.Qt__GroupSwitchModifier, "\\", false, 1),
			"<Bslash>",
		},
		{
			`convertKey() Linux German keyboardlayout 7`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_AsciiTilde), core.Qt__GroupSwitchModifier, "~", false, 1),
			"~",
		},
		{
			`convertKey() Linux Control+Space`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Space), core.Qt__ControlModifier, string('\u0000'), false, 1),
			"<C-Space>",
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
					`convertKey() Linux special keys`,
					gui.NewQKeyEvent(core.QEvent__KeyPress, int(key), core.Qt__NoModifier, value, false, 1),
					fmt.Sprintf("<%s>", value),
				},
				{
					`convertKey() Linux special keys with Ctrl`,
					gui.NewQKeyEvent(core.QEvent__KeyPress, int(key), core.Qt__ControlModifier, value, false, 1),
					fmt.Sprintf("<C-%s>", value),
				},
				{
					`convertKey() Linux special keys with Alt`,
					gui.NewQKeyEvent(core.QEvent__KeyPress, int(key), core.Qt__AltModifier, value, false, 1),
					fmt.Sprintf("<A-%s>", value),
				},
				{
					`convertKey() Linux special keys with Meta`,
					gui.NewQKeyEvent(core.QEvent__KeyPress, int(key), core.Qt__MetaModifier, value, false, 1),
					fmt.Sprintf("<D-%s>", value),
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

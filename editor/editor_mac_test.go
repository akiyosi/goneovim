// +build darwin

package editor

import (
	"fmt"
	"testing"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
)

func TestDarwinEditor_convertKey(t *testing.T) {
	tests := []struct {
		name string
		args *gui.QKeyEvent
		want string
	}{
		{
			`convertKey() MacOS Alt special key input å`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_A), core.Qt__AltModifier, "å", false, 1),
			"å",
		},
		{
			`convertKey() MacOS Alt special key input Å`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_A), core.Qt__ShiftModifier|core.Qt__AltModifier, "Å", false, 1),
			"Å",
		},
		{
			`convertKey() MacOS Alt special key input Ò`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_L), core.Qt__ShiftModifier|core.Qt__AltModifier, "Ò", false, 1),
			"Ò",
		},
		{
			`convertKey() MacOS LessThan modifier keys 1`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Less), core.Qt__ShiftModifier|core.Qt__ControlModifier, "<", false, 1),
			"<D-lt>",
		},
		{
			`convertKey() MacOS LessThan modifier keys 2`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Less), core.Qt__ShiftModifier|core.Qt__AltModifier, "<", false, 1),
			"<A-lt>",
		},
		{
			`convertKey() MacOS LessThan modifier keys 3`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Less), core.Qt__ShiftModifier|core.Qt__MetaModifier, "<", false, 1),
			"<C-lt>",
		},
		{
			`convertKey() MacOS keyboardlayout unicode hex input 1`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_A), core.Qt__AltModifier, "", false, 1),
			"<A-a>",
		},
		{
			`convertKey() MacOS keyboardlayout unicode hex input 2`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_A), core.Qt__AltModifier|core.Qt__ShiftModifier, "", false, 1),
			"<A-A>",
		},
		{
			`convertKey() MacOS keyboardlayout unicode hex input 3`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_A), core.Qt__MetaModifier|core.Qt__AltModifier, "", false, 1),
			"<C-A-a>",
		},
		{
			`convertKey() MacOS keyboardlayout unicode hex input 4`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_A), core.Qt__MetaModifier|core.Qt__AltModifier|core.Qt__ShiftModifier, "", false, 1),
			"<C-A-A>",
		},
	}
	e := &Editor{}
	e.InitSpecialKeys()
	e.config = gonvimConfig{}
	e.config.Editor.Macmeta = false

	for key, value := range e.specialKeys {
		tests = append(
			tests,
			[]struct {
				name string
				args *gui.QKeyEvent
				want string
			}{
				{
					`convertKey() MacOS special keys`,
					gui.NewQKeyEvent(core.QEvent__KeyPress, int(key), core.Qt__NoModifier, value, false, 1),
					fmt.Sprintf("<%s>", value),
				},
				{
					`convertKey() MacOS special keys with Ctrl`,
					gui.NewQKeyEvent(core.QEvent__KeyPress, int(key), core.Qt__ControlModifier, value, false, 1),
					fmt.Sprintf("<D-%s>", value),
				},
				{
					`convertKey() MacOS special keys with Alt`,
					gui.NewQKeyEvent(core.QEvent__KeyPress, int(key), core.Qt__AltModifier, value, false, 1),
					fmt.Sprintf("<A-%s>", value),
				},
				{
					`convertKey() MacOS special keys with Meta`,
					gui.NewQKeyEvent(core.QEvent__KeyPress, int(key), core.Qt__MetaModifier, value, false, 1),
					fmt.Sprintf("<C-%s>", value),
				},
			}...,
		)

	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if got := e.convertKey(tt.args); got != tt.want {
				t.Errorf("Editor.convertKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

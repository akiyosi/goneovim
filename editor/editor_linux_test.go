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

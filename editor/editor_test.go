package editor

import (
	"testing"

	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
)

func TestEditor_convertKey(t *testing.T) {
	tests := []struct {
		name string
		args *gui.QKeyEvent
		want string
	}{
		{
			`convertKey() Shift is implied with "<", send "<lt>" instead`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Less), core.Qt__ShiftModifier, "<", false, 1),
			"<lt>",
		},
		{
			`convertKey() Ignore Meta modifier only key events`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Meta), core.Qt__MetaModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Ctrl modifier only key events`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Control), core.Qt__ControlModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Alt modifier only key events`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Alt), core.Qt__AltModifier, "", false, 1),
			"",
		},
		{
			`convertKey() <C-S-> is ""`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Control), core.Qt__ShiftModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Pressing Control + Super is ""`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Super_L), core.Qt__ControlModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Prevent pressing AltGr inserts weird char`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_AltGr), core.Qt__GroupSwitchModifier, "", false, 1),
			"",
		},
		{
			`convertKey() AltGr key well formed`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_AsciiTilde), core.Qt__GroupSwitchModifier, "", false, 1),
			"~",
		},
		{
			`convertKey() ShiftKey event well formed }`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_BraceRight), core.Qt__ShiftModifier, "}", false, 1),
			"}",
		},
		{
			`convertKey() ShiftKey event well formed "`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_QuoteDbl), core.Qt__ShiftModifier, `"`, false, 1),
			`"`,
		},
		{
			`convertKey() ShiftKey event well formed :`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Colon), core.Qt__ShiftModifier, `:`, false, 1),
			`:`,
		},
		{
			`convertKey() ShiftKey event well formed A`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_A), core.Qt__ShiftModifier, `A`, false, 1),
			`A`,
		},
		{
			`convertKey() ShiftKey event well formed B`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_B), core.Qt__ShiftModifier, `B`, false, 1),
			`B`,
		},
		{
			`convertKey() ShiftKey event well formed C`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_C), core.Qt__ShiftModifier, `C`, false, 1),
			`C`,
		},
		{
			`convertKey() Ignore capslock 1`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_CapsLock), core.Qt__NoModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore capslock 2`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_CapsLock), core.Qt__MetaModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore capslock 3`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_CapsLock), core.Qt__ControlModifier, "", false, 1),
			"",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			e := &Editor{}
			e.InitSpecialKeys()

			if got := e.convertKey(tt.args); got != tt.want {
				t.Errorf("Editor.convertKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

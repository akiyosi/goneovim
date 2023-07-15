package editor

import (
	"testing"

	"github.com/akiyosi/qt/core"
	"github.com/akiyosi/qt/gui"
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
			`convertKey() Ignore Modifier Only Key Events 1`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Alt), core.Qt__NoModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 2`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_AltGr), core.Qt__NoModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 3`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Control), core.Qt__NoModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 4`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Meta), core.Qt__NoModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 5`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Shift), core.Qt__NoModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 6`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Super_L), core.Qt__NoModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 7`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Super_R), core.Qt__NoModifier, "", false, 1),
			"",
		},

		{
			`convertKey() Ignore Modifier Only Key Events 8`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Alt), core.Qt__AltModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 9`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_AltGr), core.Qt__AltModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 10`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Control), core.Qt__AltModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 11`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Meta), core.Qt__AltModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 12`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Shift), core.Qt__AltModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 13`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Super_L), core.Qt__AltModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 14`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Super_R), core.Qt__AltModifier, "", false, 1),
			"",
		},

		{
			`convertKey() Ignore Modifier Only Key Events 15`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Alt), core.Qt__ControlModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 16`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_AltGr), core.Qt__ControlModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 17`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Control), core.Qt__ControlModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 18`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Meta), core.Qt__ControlModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 19`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Shift), core.Qt__ControlModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 20`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Super_L), core.Qt__ControlModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 21`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Super_R), core.Qt__ControlModifier, "", false, 1),
			"",
		},

		{
			`convertKey() Ignore Modifier Only Key Events 22`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Alt), core.Qt__MetaModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 23`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_AltGr), core.Qt__MetaModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 24`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Control), core.Qt__MetaModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 25`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Meta), core.Qt__MetaModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 26`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Shift), core.Qt__MetaModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 27`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Super_L), core.Qt__MetaModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 28`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Super_R), core.Qt__MetaModifier, "", false, 1),
			"",
		},

		{
			`convertKey() Ignore Modifier Only Key Events 29`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Alt), core.Qt__ShiftModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 30`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_AltGr), core.Qt__ShiftModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 31`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Control), core.Qt__ShiftModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 32`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Meta), core.Qt__ShiftModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 33`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Shift), core.Qt__ShiftModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 34`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Super_L), core.Qt__ShiftModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key Events 35`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Super_R), core.Qt__ShiftModifier, "", false, 1),
			"",
		},

		{
			`convertKey() Ignore Modifier Only Key  Events 36`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Alt), core.Qt__GroupSwitchModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key  Events 37`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_AltGr), core.Qt__GroupSwitchModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key  Events 38`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Control), core.Qt__GroupSwitchModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key  Events 39`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Meta), core.Qt__GroupSwitchModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key  Events 40`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Shift), core.Qt__GroupSwitchModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key  Events 41`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Super_L), core.Qt__GroupSwitchModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Modifier Only Key  Events 42`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Super_R), core.Qt__GroupSwitchModifier, "", false, 1),
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
			`convertKey() Ignore Volume keys`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_VolumeDown), core.Qt__NoModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Volume keys`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_VolumeMute), core.Qt__NoModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Ignore Volume keys`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_VolumeUp), core.Qt__NoModifier, "", false, 1),
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
		{
			`convertKey() Shift space well formed 1`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Shift), core.Qt__ShiftModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Shift space well formed 2`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Space), core.Qt__ShiftModifier, " ", false, 1),
			"<Space>",
		},

		{
			`convertKey() Shift back space well formed 1`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Shift), core.Qt__ShiftModifier, "", false, 1),
			"",
		},
		{
			`convertKey() Shift back space well formed 2`,
			gui.NewQKeyEvent(core.QEvent__KeyPress, int(core.Qt__Key_Backspace), core.Qt__ShiftModifier, "\b", false, 1),
			"<BS>",
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

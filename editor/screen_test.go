package editor

import (
	"reflect"
	"testing"
)

func TestHighlight_fg(t *testing.T) {
	type fields struct {
		id            int
		kind          string
		uiName        string
		hlName        string
		foreground    *RGBA
		background    *RGBA
		special       *RGBA
		reverse       bool
		italic        bool
		bold          bool
		underline     bool
		undercurl     bool
		strikethrough bool
	}
	tests := []struct {
		name   string
		fields fields
		want   *RGBA
	}{
		// TODO: Add test cases.
		{
			"1",
			fields{
				foreground: &RGBA{
					R: 10,
					G: 20,
					B: 30,
					A: 1.0,
				},
				background: nil,
				reverse:    false,
			},
			&RGBA{
				R: 10,
				G: 20,
				B: 30,
				A: 1.0,
			},
		},
		{
			"2",
			fields{
				foreground: &RGBA{
					R: 10,
					G: 20,
					B: 30,
					A: 1.0,
				},
				background: &RGBA{
					R: 30,
					G: 40,
					B: 50,
					A: 1.0,
				},
				reverse: true,
			},
			&RGBA{
				R: 30,
				G: 40,
				B: 50,
				A: 1.0,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hl := &Highlight{
				id:            tt.fields.id,
				kind:          tt.fields.kind,
				uiName:        tt.fields.uiName,
				hlName:        tt.fields.hlName,
				foreground:    tt.fields.foreground,
				background:    tt.fields.background,
				special:       tt.fields.special,
				reverse:       tt.fields.reverse,
				italic:        tt.fields.italic,
				bold:          tt.fields.bold,
				underline:     tt.fields.underline,
				undercurl:     tt.fields.undercurl,
				strikethrough: tt.fields.strikethrough,
			}
			if got := hl.fg(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Highlight.fg() = %v, want %v", got, tt.want)
			}
		})
	}
}


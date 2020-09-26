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
			"test_highlight_fg() 1",
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
			"test_highlight_fg() 2",
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
		tt := tt
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

func TestWindow_updateLine(t *testing.T) {
	type fields struct {
		//	rwMutex          sync.RWMutex
		//	paintMutex       sync.Mutex
		//	redrawMutex      sync.Mutex
		s       *Screen
		content [][]*Cell
		//	lenLine          []int
		//	lenContent       []int
		//	lenOldContent    []int
		grid gridId
		//	isGridDirty      bool
		//	id               nvim.Window
		//	bufName          string
		//	pos              [2]int
		//	anchor           string
		cols int
		rows int
		//	isMsgGrid        bool
		//	isFloatWin       bool
		//	widget           *widgets.QWidget
		//	shown            bool
		//	queueRedrawArea  [4]int
		//	scrollRegion     []int
		//	devicePixelRatio float64
		//	textCache        gcache.Cache
		//	font             *Font
		//	fobackground       *RGBA
		//	width            float64
		//	height           int
		//	localWindows     *[4]localWindow
	}
	type args struct {
		col   int
		row   int
		cells []interface{}
	}

	// Def grid for test
	gridid := 6
	row := 1
	rows := 2
	cols := 5

	// Def hlAttrDef for test
	hldef := make(map[int]*Highlight)
	hldef[0] = &Highlight{
		id: 0,
		foreground: &RGBA{
			0,
			0,
			0,
			1.0,
		},
		background: &RGBA{
			0,
			0,
			0,
			1.0,
		},
	}
	hldef[6] = &Highlight{
		id: 6,
		foreground: &RGBA{
			6,
			6,
			0,
			1.0,
		},
		background: &RGBA{
			0,
			6,
			6,
			1.0,
		},
	}
	hldef[7] = &Highlight{
		id: 7,
		foreground: &RGBA{
			7,
			7,
			0,
			1.0,
		},
		background: &RGBA{
			0,
			7,
			7,
			1.0,
		},
	}

	// Init grid content
	content := make([][]*Cell, rows)
	for i := 0; i < rows; i++ {
		content[i] = make([]*Cell, cols)
	}

	// Def tests
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []Cell
	}{
		// TODO: Add test cases.
		{
			"test_updateline() 1",
			fields{
				s:       &Screen{hlAttrDef: hldef},
				content: content,
				grid:    gridid,
				cols:    cols,
				rows:    rows,
			},
			args{
				col: 0,
				row: row,
				cells: []interface{}{
					[]interface{}{"~", 7},
					[]interface{}{" ", 7, 4},
				},
			},
			[]Cell{
				Cell{true, "~", hldef[7]},
				Cell{true, " ", hldef[7]},
				Cell{true, " ", hldef[7]},
				Cell{true, " ", hldef[7]},
				Cell{true, " ", hldef[7]},
			},
		},
		{
			"test_updateline() 2",
			fields{
				s:       &Screen{hlAttrDef: hldef},
				content: content,
				grid:    6,
				cols:    cols,
				rows:    rows,
			},
			args{
				col: 3,
				row: row,
				cells: []interface{}{
					[]interface{}{"*", 6, 2},
				},
			},
			[]Cell{
				Cell{true, "~", hldef[7]},
				Cell{true, " ", hldef[7]},
				Cell{true, " ", hldef[7]},
				Cell{true, "*", hldef[6]},
				Cell{true, "*", hldef[6]},
			},
		},
		{
			"test_updateline() 3",
			fields{
				s:       &Screen{hlAttrDef: hldef},
				content: content,
				grid:    6,
				cols:    cols,
				rows:    rows,
			},
			args{
				col: 1,
				row: row,
				cells: []interface{}{
					[]interface{}{"@", 6},
					[]interface{}{"v"},
					[]interface{}{"i"},
					[]interface{}{"m"},
				},
			},
			[]Cell{
				Cell{true, "~", hldef[7]},
				Cell{true, "@", hldef[6]},
				Cell{true, "v", hldef[6]},
				Cell{true, "i", hldef[6]},
				Cell{true, "m", hldef[6]},
			},
		},
		{
			"test_updateline() 4",
			fields{
				s:       &Screen{hlAttrDef: hldef},
				content: content,
				grid:    6,
				cols:    cols,
				rows:    rows,
			},
			args{
				col: 0,
				row: row,
				cells: []interface{}{
					[]interface{}{" ", 7, 2},
					[]interface{}{"J"},
				},
			},
			[]Cell{
				Cell{true, " ", hldef[7]},
				Cell{true, " ", hldef[7]},
				Cell{true, "J", hldef[7]},
				Cell{true, "i", hldef[6]},
				Cell{true, "m", hldef[6]},
			},
		},
	}

	// Do tests
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			w := &Window{
				s:       tt.fields.s,
				content: tt.fields.content,
				grid:    tt.fields.grid,
				cols:    tt.fields.cols,
				rows:    tt.fields.rows,
			}
			w.updateLine(tt.args.col, tt.args.row, tt.args.cells)

			got := w.content[row]
			for i, cell := range got {
				if cell == nil {
					continue
				}

				if cell.char != tt.want[i].char {
					t.Errorf("col: %v, actual: %v, want: %v", i, cell.char, tt.want[i].char)
				}
				if cell.highlight.id != tt.want[i].highlight.id {
					t.Errorf("col: %v, actual: %v, want: %v", i, cell.highlight.id, tt.want[i].highlight.id)
				}
				if cell.normalWidth != tt.want[i].normalWidth {
					t.Errorf("col: %v, actual: %v, want: %v", i, cell.normalWidth, tt.want[i].normalWidth)
				}
			}
		})
	}
}

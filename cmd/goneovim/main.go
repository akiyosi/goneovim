package main

import (
	// "net/http"
	// _ "net/http/pprof"

	"github.com/akiyosi/goneovim/editor"
)

func main() {
	// go func() {
	// 	http.ListenAndServe("localhost:6080", nil)
	// }()
	editor.InitEditor()
}

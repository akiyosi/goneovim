package main

import (
	"net/http"

	_ "net/http/pprof"

	"github.com/dzhou121/gonvim"
)

func main() {
	go func() {
		http.ListenAndServe("localhost:6080", nil)
	}()
	gonvim.InitEditorNew()
}

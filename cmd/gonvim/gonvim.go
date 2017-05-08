package main

import (
	"net/http"

	_ "net/http/pprof"

	"github.com/dzhou121/gonvim"
	"github.com/dzhou121/ui"
)

func main() {
	go func() {
		http.ListenAndServe("0.0.0.0:6080", nil)
	}()
	err := ui.Main(func() {
		gonvim.InitEditor()
	})
	if err != nil {
		panic(err)
	}
}

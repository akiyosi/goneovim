package main

import (
	"net/http"

	"github.com/dzhou121/gonvim"
	"github.com/dzhou121/ui"
)

func main() {
	go func() {
		http.ListenAndServe("0.0.0.0:6060", nil)
	}()
	err := ui.Main(func() {
		gonvim.InitEditor()
	})
	if err != nil {
		panic(err)
	}
}

package main

import (
	"github.com/akiyosi/goneovim/editor"
)

func main() {
	// // profile the application
	// //  https://blog.golang.org/pprof
	// // After running the app, do the following:
	// //  $ go tool pprof -http=localhost:9090 cpuprofile
	// f, err := os.Create("cpuprofile")
	// if err != nil {
	// 	os.Exit(1)
	// }
	// defer f.Close()
	// pprof.StartCPUProfile(f)
	// defer pprof.StopCPUProfile()

	editor.InitEditor()
}

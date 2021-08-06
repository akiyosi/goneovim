package main

import (
	"fmt"
	"os"

	// "runtime/pprof"
	// "github.com/felixge/fgprof"

	"github.com/akiyosi/goneovim/editor"
	"github.com/jessevdk/go-flags"
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

	// // fgprof
	// f, err := os.Create("cpuprofile")
	// if err != nil {
	// 	os.Exit(1)
	// }
	// fgprofStop := fgprof.Start(f, fgprof.FormatPprof)
	// defer func() {
	// 	err = fgprofStop()
	// 	if err != nil {
	// 		fmt.Println(err)
	// 	}
	// 	err = f.Close()
	// 	if err != nil {
	// 		fmt.Println(err)
	// 	}
	// }()

	// parse args
	options, args := parseArgs()
	if options.Version {
			fmt.Println(editor.Version)
			os.Exit(0)
	}

	// start editor
	editor.InitEditor(options, args)
}

// parsArgs parse args
func parseArgs() (editor.Options, []string) {
	var options editor.Options
	parser := flags.NewParser(&options, flags.HelpFlag|flags.PassDoubleDash)
	args, err := parser.ParseArgs(os.Args[1:])
	if flagsErr, ok := err.(*flags.Error); ok {
		switch flagsErr.Type {
		case flags.ErrDuplicatedFlag:
		case flags.ErrHelp:
			fmt.Println(err)
			os.Exit(1)
		}
	}

	return options, args
}

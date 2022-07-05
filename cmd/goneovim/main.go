package main

import (
	"fmt"
	"os"
	"runtime"

	// "runtime/pprof"
	// "github.com/felixge/fgprof"

	"github.com/akiyosi/goneovim/editor"
	"github.com/jessevdk/go-flags"
	"github.com/mattn/go-isatty"
	"github.com/therecipe/qt/core"
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

	nofork := options.Nofork

	// In Windows, nofork always true
	if runtime.GOOS == "windows" {
		nofork = true
	}
	// In MacOS X do not fork when the process is not launched from a tty
	if runtime.GOOS == "darwin" {
		if !isatty.IsTerminal(os.Stdin.Fd()) {
			nofork = true
		}
	}
	// If ssh option specified
	if options.Ssh != "" {
		nofork = true
	}
	// start editor
	if nofork {
		editor.InitEditor(options, args)
	} else {
		p := core.NewQProcess(nil)
		var pid int64
		goneovim := core.NewQCoreApplication(len(os.Args), os.Args)
		if !p.StartDetached2(
			goneovim.ApplicationFilePath(),
			append([]string{"--nofork"}, goneovim.Arguments()[1:]...),
			"",
			pid,
		) {
			fmt.Println("Unable to fork into background")
			os.Exit(-1)
		}
	}
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

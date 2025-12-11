//go:build linux || freebsd
// +build linux freebsd

package editor

import (
	"C"

	frameless "github.com/akiyosi/goqtframelesswindow"
	"github.com/akiyosi/qt/widgets"
)

func GetOpeningFilepath(str *C.char) {
}

func setMyApplicationDelegate() {
}

func setNativeTitlebarColor(window *frameless.QFramelessWindow, colorStr string) error {
	// Not implemented (yet)
	return nil
}

func setNativeTitleTextColor(window *frameless.QFramelessWindow, colorStr string) error {
	// Not implemented (yet)
	return nil
}

func setIMEOff(_ *widgets.QWidget) {
	// There is no general, OS-level way to turn off the IME on Unix.
}

//go:build linux || freebsd
// +build linux freebsd

package editor

import (
	"C"

	frameless "github.com/akiyosi/goqtframelesswindow"
)

func GetOpeningFilepath(str *C.char) {
}

func setMyApplicationDelegate() {
}

func setNativeTitleBarColor(window *frameless.QFramelessWindow, colorStr string) error {
	// Not implemented (yet)
	return nil
}

func setNativeTitleTextColor(window *frameless.QFramelessWindow, colorStr string) error {
	// Not implemented (yet)
	return nil
}

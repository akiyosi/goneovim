package editor

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include "objcbridge.h"
#include <stdlib.h>
*/
import "C"
import (
	"unsafe"

	frameless "github.com/akiyosi/goqtframelesswindow"
)

//export GetOpeningFilepath
func GetOpeningFilepath(str *C.char) {
	goStr := C.GoString(str)
	C.free(unsafe.Pointer(str))

	if editor.openingFileCh == nil {
		editor.openingFileCh = make(chan string, 2)
	}
	editor.openingFileCh <- goStr
}

func setMyApplicationDelegate() {
	C.SetMyApplicationDelegate()
}

func setNativeTitlebarColor(window *frameless.QFramelessWindow, colorStr string) error {
	// Not implemented (yet)
	return nil
}

func setNativeTitleTextColor(window *frameless.QFramelessWindow, colorStr string) error {
	// Not implemented (yet)
	return nil
}

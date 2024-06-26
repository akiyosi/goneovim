package editor

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include "objcbridge.h"
#include <stdlib.h>
*/
import "C"
import "unsafe"

//export GetOpeningFilepath
func GetOpeningFilepath(str *C.char) {
	goStr := C.GoString(str)
	C.free(unsafe.Pointer(str))
	editor.openingFileCh <- goStr
}

func setMyApplicationDelegate() {
	C.SetMyApplicationDelegate()
}

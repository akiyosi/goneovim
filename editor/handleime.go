package editor

//#include <stdint.h>
//#include "handleime.h"
import "C"
import (
	"runtime"

	"github.com/therecipe/qt/gui"
)

func selectionPosInPreeditStr(event *gui.QInputMethodEvent) (cursorPos, selectionLength int) {
	cursorPos = int(C.cursorPosInPreeditStr(event.Pointer()))

	if runtime.GOOS == "darwin" {
		selectionLength = int(C.selectionLengthInPreeditStrOnDarwin(event.Pointer(), C.int(cursorPos)))
	} else {
		selectionLength = int(C.selectionLengthInPreeditStr(event.Pointer(), C.int(cursorPos)))
	}

	return
}

package editor

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

void setNoButtonWindow(long *wid) {
    NSView* view = (NSView*)wid;
    NSWindow *window = view.window;

    // Style
    window.styleMask |= NSWindowStyleMaskTitled;
    window.styleMask |= NSWindowStyleMaskResizable;
    window.styleMask |= NSWindowStyleMaskMiniaturizable;
    window.styleMask |= NSWindowStyleMaskFullSizeContentView;

    // Appearance
    window.opaque = NO;
    window.backgroundColor = [NSColor clearColor];
    window.hasShadow = YES;

    // Don't show title bar
    window.titlebarAppearsTransparent = YES;
    // window.titleVisibility = NSWindowTitleHidden;

    // Hidden native buttons
    [[window standardWindowButton:NSWindowCloseButton] setHidden:YES];
    [[window standardWindowButton:NSWindowMiniaturizeButton] setHidden:YES];
    [[window standardWindowButton:NSWindowZoomButton] setHidden:YES];
    return;
}

*/
import "C"

import (
	"unsafe"
)

func createExternalWin() *ExternalWin {
	extwin := NewExternalWin(nil, 0)
	extwin.SetContentsMargins(5, 5, 5, 5)
	wid := extwin.WinId()

	C.setNoButtonWindow((*C.long)(unsafe.Pointer(wid)))

	return extwin
}

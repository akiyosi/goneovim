package editor

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

@interface MyApplicationDelegate : NSObject <NSApplicationDelegate>
@end

@implementation MyApplicationDelegate

- (BOOL)applicationSupportsSecureRestorableState:(NSApplication *)app {
    return YES;
}

@end

void SetMyApplicationDelegate() {
    NSApplication *app = [NSApplication sharedApplication];
    app.delegate = [[MyApplicationDelegate alloc] init];
    [app activateIgnoringOtherApps:YES]; // make application foreground
}

*/
import "C"

func setMyApplicationDelegate() {
	C.SetMyApplicationDelegate()
}

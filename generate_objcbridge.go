//go:build generate

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"text/template"
)

const objcbridgeH = `void SetMyApplicationDelegate();`

const objcbridgeM = `#import <Cocoa/Cocoa.h>

// Forward declaration of the Go function to be called from C
extern void GetOpeningFilepath(char* str);

@interface MyApplicationDelegate : NSObject <NSApplicationDelegate>
@end

@implementation MyApplicationDelegate

- (BOOL)applicationSupportsSecureRestorableState:(NSApplication *)app {
    return YES;
}

- (BOOL)application:(NSApplication *)theApplication openFile:(NSString *)filename {
    const char *utf8String = [filename UTF8String];
    char *cStr = strdup(utf8String);
    GetOpeningFilepath(cStr);
    return YES;
}

@end

void SetMyApplicationDelegate() {
    NSApplication *app = [NSApplication sharedApplication];
    app.delegate = [[MyApplicationDelegate alloc] init];
    [app activateIgnoringOtherApps:YES]; // make application foreground
}`

func main() {
	if runtime.GOOS != "darwin" {
		return
	}

	generateFile(filepath.Join("editor", "objcbridge.h"), objcbridgeH)
	generateFile(filepath.Join("editor", "objcbridge.m"), objcbridgeM)
}

func generateFile(filename, content string) {
	if _, err := os.Stat(filename); err == nil {
		err = os.Remove(filename)
		if err != nil {
			fmt.Println("Error removing existing file:", err)
			return
		}
	} else if !os.IsNotExist(err) {
		fmt.Println("Error checking file existence:", err)
		return
	}

	file, err := os.Create(filename)
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close()

	tmpl, err := template.New("file").Parse(content)
	if err != nil {
		fmt.Println("Error parsing template:", err)
		return
	}

	err = tmpl.Execute(file, nil)
	if err != nil {
		fmt.Println("Error executing template:", err)
	}
}

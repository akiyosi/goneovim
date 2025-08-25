package editor

import (
	"C"
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	frameless "github.com/akiyosi/goqtframelesswindow"
)

var (
	dwmapi                = syscall.NewLazyDLL("dwmapi.dll")
	dwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
)

const (
	DWMWA_CAPTION_COLOR = 35
	DWMWA_TEXT_COLOR    = 36
)

func GetOpeningFilepath(str *C.char) {
}

func setMyApplicationDelegate() {
}

func checkWindowsVersion() bool {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getVersionEx := kernel32.NewProc("GetVersionExW")

	type OSVERSIONINFO struct {
		OSVersionInfoSize uint32
		MajorVersion      uint32
		MinorVersion      uint32
		BuildNumber       uint32
		PlatformId        uint32
		CSDVersion        [128]uint16
	}

	var osvi OSVERSIONINFO
	osvi.OSVersionInfoSize = uint32(unsafe.Sizeof(osvi))

	ret, _, _ := getVersionEx.Call(uintptr(unsafe.Pointer(&osvi)))
	if ret != 0 {
		// Windows 11 Build 22000 or later supports title bar customization
		return osvi.BuildNumber >= 22000
	}

	return false
}

func setNativeTitlebarColor(window *frameless.QFramelessWindow, colorStr string) error {
	if !checkWindowsVersion() {
		return fmt.Errorf("unsupported Windows version")
	}

	color, err := parseColor(colorStr)
	if err != nil {
		return err
	}

	hwnd := window.WindowWidget.Window().EffectiveWinId()

	ret, _, _ := dwmSetWindowAttribute.Call(
		hwnd,
		DWMWA_CAPTION_COLOR,
		uintptr(unsafe.Pointer(&color)),
		unsafe.Sizeof(color),
	)

	if ret != 0 {
		return syscall.Errno(ret)
	}

	return nil
}

func setNativeTitleTextColor(window *frameless.QFramelessWindow, colorStr string) error {
	if !checkWindowsVersion() {
		return fmt.Errorf("unsupported Windows version")
	}

	color, err := parseColor(colorStr)
	if err != nil {
		return err
	}

	hwnd := window.WindowWidget.Window().EffectiveWinId()

	ret, _, _ := dwmSetWindowAttribute.Call(
		hwnd,
		DWMWA_TEXT_COLOR,
		uintptr(unsafe.Pointer(&color)),
		unsafe.Sizeof(color),
	)

	if ret != 0 {
		return syscall.Errno(ret)
	}

	return nil
}

func parseColor(colorStr string) (uint32, error) {
	if colorStr == "" {
		return 0, nil
	}

	colorStr = strings.TrimPrefix(colorStr, "#")

	var r, g, b uint64 = 0, 0, 0
	var err error

	switch len(colorStr) {
	case 6: // RGB
		r, err = strconv.ParseUint(colorStr[0:2], 16, 8)
		if err != nil {
			return 0, err
		}
		g, err = strconv.ParseUint(colorStr[2:4], 16, 8)
		if err != nil {
			return 0, err
		}
		b, err = strconv.ParseUint(colorStr[4:6], 16, 8)
		if err != nil {
			return 0, err
		}
	default:
		return 0, syscall.EINVAL
	}

	// Convert to Windows COLORREF format
	return uint32(b<<16 | g<<8 | r), nil
}

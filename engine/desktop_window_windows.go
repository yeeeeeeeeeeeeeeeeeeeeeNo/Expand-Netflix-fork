//go:build windows

package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/jchv/go-webview2"
)

var (
	modDwmapi                 = syscall.NewLazyDLL("dwmapi.dll")
	procDwmSetWindowAttr      = modDwmapi.NewProc("DwmSetWindowAttribute")
	modUser32                 = syscall.NewLazyDLL("user32.dll")
	procFindWindowW           = modUser32.NewProc("FindWindowW")
	procShowWindow            = modUser32.NewProc("ShowWindow")
	procSetForegroundWindow   = modUser32.NewProc("SetForegroundWindow")
	procSetProcessDpiAwareCtx = modUser32.NewProc("SetProcessDpiAwarenessContext")
	procGetWindowLongPtrW     = modUser32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW     = modUser32.NewProc("SetWindowLongPtrW")
	procGetWindowRect         = modUser32.NewProc("GetWindowRect")
	procSetWindowPos          = modUser32.NewProc("SetWindowPos")
	procMonitorFromWindow     = modUser32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW       = modUser32.NewProc("GetMonitorInfoW")
	procIsZoomed              = modUser32.NewProc("IsZoomed")
	procSendMessageW          = modUser32.NewProc("SendMessageW")
	modShell32                = syscall.NewLazyDLL("shell32.dll")
	procExtractIconExW        = modShell32.NewProc("ExtractIconExW")
	modOle32                  = syscall.NewLazyDLL("ole32.dll")
	procCoInitializeEx        = modOle32.NewProc("CoInitializeEx")
	procCoUninitialize        = modOle32.NewProc("CoUninitialize")
)

const (
	dwmwaUseImmersiveDarkMode = 20
	dwmwaBorderColor          = 34
	dwmwaCaptionColor         = 35
	swRestore                 = 9
	swMaximize                = 3
	coinitApartmentThreaded   = 0x2

	wsOverlappedWindow  = 0x00CF0000
	wsPopup             = 0x80000000
	wsVisible           = 0x10000000
	swpFrameChanged     = 0x0020
	swpShowWindow       = 0x0040
	swpNoOwnerZOrder    = 0x0200
	monitorDefaultToNea = 2
)

type winRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type winMonitorInfo struct {
	CbSize    uint32
	RcMonitor winRect
	RcWork    winRect
	DwFlags   uint32
}

type nativeFullscreenState struct {
	active       bool
	savedStyle   uintptr
	savedRect    winRect
	wasMaximized bool
}

func focusExistingNoirWindow() bool {
	title, err := syscall.UTF16PtrFromString("NOCTURNE")
	if err != nil {
		return false
	}
	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(title)))
	if hwnd == 0 {
		return false
	}
	_, _, _ = procShowWindow.Call(hwnd, swRestore)
	_, _, _ = procSetForegroundWindow.Call(hwnd)
	return true
}

func applyNocturneWindowIcon(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	exePath, err := os.Executable()
	if err != nil || exePath == "" {
		return
	}
	exePtr, err := syscall.UTF16PtrFromString(exePath)
	if err != nil {
		return
	}
	var hLarge, hSmall uintptr
	count, _, _ := procExtractIconExW.Call(
		uintptr(unsafe.Pointer(exePtr)),
		0,
		uintptr(unsafe.Pointer(&hLarge)),
		uintptr(unsafe.Pointer(&hSmall)),
		1,
	)
	if count == 0 {
		return
	}
	const wmSetIcon = 0x0080
	if hLarge != 0 {
		_, _, _ = procSendMessageW.Call(hwnd, wmSetIcon, 1, hLarge) // ICON_BIG (Taskbar / Alt-Tab)
	}
	if hSmall != 0 {
		_, _, _ = procSendMessageW.Call(hwnd, wmSetIcon, 0, hSmall) // ICON_SMALL (Titlebar)
	}
}

func applyDarkNoirTitleBar(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	var dark uint32 = 1
	_, _, _ = procDwmSetWindowAttr.Call(
		hwnd,
		dwmwaUseImmersiveDarkMode,
		uintptr(unsafe.Pointer(&dark)),
		unsafe.Sizeof(dark),
	)
	var captionBlack uint32 = 0x00000000
	_, _, _ = procDwmSetWindowAttr.Call(
		hwnd,
		dwmwaCaptionColor,
		uintptr(unsafe.Pointer(&captionBlack)),
		unsafe.Sizeof(captionBlack),
	)
	var borderSlate uint32 = 0x0030231E
	_, _, _ = procDwmSetWindowAttr.Call(
		hwnd,
		dwmwaBorderColor,
		uintptr(unsafe.Pointer(&borderSlate)),
		unsafe.Sizeof(borderSlate),
	)
}

func setWin32BorderlessFullscreen(hwnd uintptr, state *nativeFullscreenState, enable bool) {
	if hwnd == 0 {
		return
	}
	gwlStyle := int(-16)
	if enable {
		if state.active {
			return
		}
		zoomed, _, _ := procIsZoomed.Call(hwnd)
		state.wasMaximized = zoomed != 0
		if state.wasMaximized {
			_, _, _ = procShowWindow.Call(hwnd, swRestore)
		}
		style, _, _ := procGetWindowLongPtrW.Call(hwnd, uintptr(gwlStyle))
		state.savedStyle = style
		_, _, _ = procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&state.savedRect)))

		hmon, _, _ := procMonitorFromWindow.Call(hwnd, monitorDefaultToNea)
		var mi winMonitorInfo
		mi.CbSize = uint32(unsafe.Sizeof(mi))
		ok, _, _ := procGetMonitorInfoW.Call(hmon, uintptr(unsafe.Pointer(&mi)))
		if ok == 0 {
			return
		}

		// Strip titlebar, borders, and window chrome -> true edge-to-edge monitor fullscreen over taskbar
		newStyle := (style &^ wsOverlappedWindow) | wsPopup | wsVisible
		_, _, _ = procSetWindowLongPtrW.Call(hwnd, uintptr(gwlStyle), newStyle)
		rc := mi.RcMonitor
		_, _, _ = procSetWindowPos.Call(
			hwnd,
			0, // HWND_TOP
			uintptr(rc.Left),
			uintptr(rc.Top),
			uintptr(rc.Right-rc.Left),
			uintptr(rc.Bottom-rc.Top),
			swpFrameChanged|swpShowWindow|swpNoOwnerZOrder,
		)
		state.active = true
	} else {
		if !state.active {
			return
		}
		restoreStyle := state.savedStyle
		if restoreStyle == 0 {
			restoreStyle = wsOverlappedWindow | wsVisible
		}
		_, _, _ = procSetWindowLongPtrW.Call(hwnd, uintptr(gwlStyle), restoreStyle)
		rc := state.savedRect
		w := rc.Right - rc.Left
		h := rc.Bottom - rc.Top
		if w < 640 || h < 400 {
			rc = winRect{Left: 120, Top: 80, Right: 1480, Bottom: 900}
			w = rc.Right - rc.Left
			h = rc.Bottom - rc.Top
		}
		_, _, _ = procSetWindowPos.Call(
			hwnd,
			0,
			uintptr(rc.Left),
			uintptr(rc.Top),
			uintptr(w),
			uintptr(h),
			swpFrameChanged|swpShowWindow|swpNoOwnerZOrder,
		)
		if state.wasMaximized {
			_, _, _ = procShowWindow.Call(hwnd, swMaximize)
		}
		applyDarkNoirTitleBar(hwnd)
		state.active = false
	}
}

func runNativeDesktopWindow(appURL string, onClose func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Enable Per-Monitor V2 DPI awareness for razor-sharp native HiDPI rendering (e.g. Legion 5 Pro 2560x1600)
	neg4 := int(-4)
	_, _, _ = procSetProcessDpiAwareCtx.Call(uintptr(neg4))

	// Initialize COM STA (required by WebView2 CreateCoreWebView2Controller)
	_, _, _ = procCoInitializeEx.Call(0, coinitApartmentThreaded)
	defer procCoUninitialize.Call()

	// Allow instant video/live TV autoplay with audio + high-performance GPU WebGL2
	_ = os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "--autoplay-policy=no-user-gesture-required --ignore-gpu-blocklist --enable-webgl --use-angle=d3d11")

	localApp := os.Getenv("LOCALAPPDATA")
	if localApp == "" {
		localApp = os.TempDir()
	}
	dataPath := filepath.Join(localApp, "NOCTURNE", "webview2_data")
	_ = os.MkdirAll(dataPath, 0755)

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		DataPath:  dataPath,
		WindowOptions: webview2.WindowOptions{
			Title:  "NOCTURNE",
			Width:  1360,
			Height: 820,
			Center: true,
		},
	})
	if w == nil {
		log.Println("[NOCTURNE] Failed to initialize native WebView2 window.")
		if onClose != nil {
			onClose()
		}
		return
	}
	defer func() {
		w.Destroy()
		if onClose != nil {
			onClose()
		}
	}()

	hwnd := uintptr(w.Window())
	applyDarkNoirTitleBar(hwnd)
	applyNocturneWindowIcon(hwnd)

	fsState := &nativeFullscreenState{}
	_ = w.Bind("noirSetNativeFullscreen", func(enable bool) bool {
		w.Dispatch(func() {
			setWin32BorderlessFullscreen(hwnd, fsState, enable)
		})
		return enable
	})

	w.Navigate(appURL)
	w.Run()
}

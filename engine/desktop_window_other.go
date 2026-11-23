//go:build !windows

package main

func focusExistingNoirWindow() bool {
	return false
}

func runNativeDesktopWindow(appURL string, onClose func()) {
	if onClose != nil {
		onClose()
	}
}

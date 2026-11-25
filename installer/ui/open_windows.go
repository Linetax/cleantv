package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"
)

// openWindow shows url in an Edge app window (no address bar, its own profile), else
// Chrome's, else the default browser. Edge ships with Windows 10 and 11.
func openWindow(url, profile string) bool {
	var exes []string
	for _, root := range []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles"), os.Getenv("LocalAppData")} {
		if root != "" {
			exes = append(exes,
				filepath.Join(root, `Microsoft\Edge\Application\msedge.exe`),
				filepath.Join(root, `Google\Chrome\Application\chrome.exe`))
		}
	}
	for _, exe := range exes {
		if _, err := os.Stat(exe); err != nil {
			continue
		}
		cmd := exec.Command(exe, "--app="+url, "--user-data-dir="+profile,
			"--no-first-run", "--no-default-browser-check", "--window-size=1080,760")
		if cmd.Start() == nil {
			return true
		}
	}
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start() == nil
}

// alert shows a Windows message box: this program has no console to print to.
func alert(title, msg string) {
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(msg)
	syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0,
		uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x40) // MB_ICONINFORMATION
}

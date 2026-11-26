//go:build !windows

package ui

import (
	"os/exec"
	"runtime"
)

// openWindow shows url in a Chromium app window when there is one, else the default browser.
func openWindow(url, profile string) bool {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", url).Start() == nil
	}
	for _, b := range []string{"google-chrome", "chromium", "chromium-browser", "microsoft-edge"} {
		if p, err := exec.LookPath(b); err == nil &&
			exec.Command(p, "--app="+url, "--user-data-dir="+profile, "--no-first-run",
				"--no-default-browser-check", "--window-size=1080,760").Start() == nil {
			return true
		}
	}
	return exec.Command("xdg-open", url).Start() == nil
}

// alert: the URL is already printed on the terminal.
func alert(title, msg string) {}

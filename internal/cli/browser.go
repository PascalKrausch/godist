package cli

import (
	"os/exec"
	"runtime"
)

// OpenBrowser öffnet url im Standardbrowser des Systems.
func OpenBrowser(url string) error {
	name, args := browserCommand(runtime.GOOS, url)
	return exec.Command(name, args...).Start()
}

// browserCommand liefert das Kommando zum Öffnen einer URL für das angegebene Betriebssystem.
func browserCommand(goos, url string) (string, []string) {
	switch goos {
	case "darwin":
		return "open", []string{url}
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default:
		return "xdg-open", []string{url}
	}
}

package chrome

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/bismitpanda/cc-util/internal/paths"
)

const ExtensionID = "mmlmfjhmonkocbjadbfplnigmagldckm"

const WebStoreURL = "https://chromewebstore.google.com/detail/" + ExtensionID

func UserDataDir() string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(paths.HomeDir(), "Library", "Application Support", "Google", "Chrome")
	case "windows":
		local := os.Getenv("LOCALAPPDATA")
		if local == "" {
			local = filepath.Join(paths.HomeDir(), "AppData", "Local")
		}
		return filepath.Join(local, "Google", "Chrome", "User Data")
	default:
		config := os.Getenv("CHROME_CONFIG_HOME")
		if config == "" {
			config = os.Getenv("XDG_CONFIG_HOME")
		}
		if config == "" {
			config = filepath.Join(paths.HomeDir(), ".config")
		}
		return filepath.Join(config, "google-chrome")
	}
}

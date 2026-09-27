package claude

import (
	"os"
	"path/filepath"

	"github.com/bismitpanda/cc-util/internal/paths"
)

func ClaudeDir() string {
	dir := os.Getenv("CLAUDE_CONFIG_DIR")
	if dir == "" {
		dir = filepath.Join(paths.HomeDir(), ".claude")
	}
	return dir
}

func CredFile() string {
	return filepath.Join(ClaudeDir(), ".credentials.json")
}

func GlobalFile() string {
	return filepath.Join(paths.HomeDir(), ".claude.json")
}

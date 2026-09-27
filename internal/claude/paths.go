package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

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

func ProjectDir(projectPath string) string {
	return filepath.Join(ClaudeDir(), "projects", projectKey(projectPath))
}

func MemoryDir(projectPath string) string {
	if dir := autoMemoryDirectory(projectPath); dir != "" {
		return dir
	}
	return filepath.Join(ProjectDir(projectPath), "memory")
}

func projectKey(projectPath string) string {
	var b strings.Builder
	for _, r := range projectPath {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}

func autoMemoryDirectory(projectPath string) string {
	candidates := []string{
		filepath.Join(projectPath, ".claude", "settings.local.json"),
		filepath.Join(projectPath, ".claude", "settings.json"),
		filepath.Join(ClaudeDir(), "settings.json"),
	}
	for _, path := range candidates {
		if dir := readAutoMemoryDirectory(path); dir != "" {
			return dir
		}
	}
	return ""
}

func readAutoMemoryDirectory(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var cfg struct {
		AutoMemoryDirectory string `json:"autoMemoryDirectory"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	dir := strings.TrimSpace(cfg.AutoMemoryDirectory)
	if dir == "" {
		return ""
	}
	if dir == "~" {
		return paths.HomeDir()
	}
	if strings.HasPrefix(dir, "~/") {
		return filepath.Join(paths.HomeDir(), dir[2:])
	}
	return dir
}

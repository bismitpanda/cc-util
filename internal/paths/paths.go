package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

func HomeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot determine home directory: %v\n", err)
		os.Exit(1)
	}
	return h
}

func RootDir() string {
	return filepath.Join(HomeDir(), ".cc-util")
}

func StoreDir() string {
	return filepath.Join(RootDir(), "accounts")
}

func SwitchesLog() string {
	return filepath.Join(RootDir(), "switches.jsonl")
}

package claude

import (
	"encoding/json"
	"os"
	"sort"
	"time"
)

type Project struct {
	Path          string
	OnDisk        bool
	Trusted       bool
	LastStart     time.Time
	LinesAdded    int
	LinesRemoved  int
	LastCost      *float64
	LastSessionID string
	Models        []string
	MCPServers    int
}

type projectEntry struct {
	HasTrustDialogAccepted bool                       `json:"hasTrustDialogAccepted"`
	LastCost               *float64                   `json:"lastCost"`
	LastStartTime          int64                      `json:"lastStartTime"`
	LastLinesAdded         int                        `json:"lastLinesAdded"`
	LastLinesRemoved       int                        `json:"lastLinesRemoved"`
	LastSessionID          string                     `json:"lastSessionId"`
	LastModelUsage         map[string]json.RawMessage `json:"lastModelUsage"`
	MCPServers             map[string]json.RawMessage `json:"mcpServers"`
}

type globalConfig struct {
	Projects map[string]projectEntry `json:"projects"`
}

func ListProjects() ([]Project, error) {
	data, err := os.ReadFile(GlobalFile())
	if err != nil {
		return nil, err
	}
	var cfg globalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	out := make([]Project, 0, len(cfg.Projects))
	for path, entry := range cfg.Projects {
		info, statErr := os.Stat(path)
		models := make([]string, 0, len(entry.LastModelUsage))
		for name := range entry.LastModelUsage {
			models = append(models, name)
		}
		sort.Strings(models)
		p := Project{
			Path:          path,
			OnDisk:        statErr == nil && info.IsDir(),
			Trusted:       entry.HasTrustDialogAccepted,
			LinesAdded:    entry.LastLinesAdded,
			LinesRemoved:  entry.LastLinesRemoved,
			LastCost:      entry.LastCost,
			LastSessionID: entry.LastSessionID,
			Models:        models,
			MCPServers:    len(entry.MCPServers),
		}
		if entry.LastStartTime > 0 {
			p.LastStart = time.UnixMilli(entry.LastStartTime)
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].LastStart.Equal(out[j].LastStart) {
			return out[i].LastStart.After(out[j].LastStart)
		}
		return out[i].Path < out[j].Path
	})
	return out, nil
}

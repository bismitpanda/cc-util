package claude

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/bismitpanda/cc-util/internal/paths"
)

type modelNameCache struct {
	Names map[string]string `json:"names"`
}

type modelInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

func LoadModelNames(token string, ids []string) (map[string]string, error) {
	cache := readModelNameCache()
	changed := false
	seen := map[string]bool{}
	var failed []string
	for _, id := range ids {
		if _, ok := DisplayModel(id, cache.Names); ok {
			continue
		}
		fetchID := id
		if base, _, ok := strings.Cut(id, "["); ok {
			fetchID = base
		}
		if seen[fetchID] {
			continue
		}
		seen[fetchID] = true
		if token == "" {
			failed = append(failed, fetchID)
			continue
		}
		name, err := fetchModelName(token, fetchID)
		if err != nil {
			failed = append(failed, fetchID)
			continue
		}
		cache.Names[fetchID] = name
		changed = true
	}
	if changed {
		_ = writeModelNameCache(cache)
	}
	if len(failed) > 0 {
		return cache.Names, fmt.Errorf("model name lookup failed for %s", strings.Join(failed, ", "))
	}
	return cache.Names, nil
}

func DisplayModel(id string, names map[string]string) (string, bool) {
	if name, ok := names[id]; ok && name != "" {
		return name, true
	}
	base, suffix, ok := strings.Cut(id, "[")
	if !ok {
		return "", false
	}
	name, found := names[base]
	if !found || name == "" {
		return "", false
	}
	suffix = strings.TrimSuffix(suffix, "]")
	if strings.EqualFold(suffix, "1m") {
		return name + " (1M)", true
	}
	if suffix == "" {
		return name, true
	}
	return name + " (" + suffix + ")", true
}

func readModelNameCache() modelNameCache {
	data, err := os.ReadFile(paths.ModelNamesFile())
	if err != nil {
		return modelNameCache{Names: map[string]string{}}
	}
	var cache modelNameCache
	if err := json.Unmarshal(data, &cache); err != nil || cache.Names == nil {
		return modelNameCache{Names: map[string]string{}}
	}
	return cache
}

func writeModelNameCache(cache modelNameCache) error {
	if err := os.MkdirAll(paths.RootDir(), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(paths.ModelNamesFile(), data, 0600)
}

func fetchModelName(token, id string) (string, error) {
	endpoint := "https://api.anthropic.com/v1/models/" + url.PathEscape(id)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("models API returned %s", resp.Status)
	}
	var model modelInfo
	if err := json.Unmarshal(body, &model); err != nil {
		return "", fmt.Errorf("parsing model response: %w", err)
	}
	if model.DisplayName == "" {
		return "", fmt.Errorf("model %s has no display name", id)
	}
	return model.DisplayName, nil
}

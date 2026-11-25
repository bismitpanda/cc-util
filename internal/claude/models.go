package claude

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bismitpanda/cc-util/internal/store"
)

type modelInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

func LoadModelNames(token string, ids []string) (map[string]string, error) {
	names := readModelNames()
	seen := map[string]bool{}
	var failed []string
	for _, id := range ids {
		if _, ok := DisplayModel(id, names); ok {
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
		names[fetchID] = name
		saveModelName(fetchID, name)
	}
	if len(failed) > 0 {
		return names, fmt.Errorf("model name lookup failed for %s", strings.Join(failed, ", "))
	}
	return names, nil
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

func readModelNames() map[string]string {
	s, err := store.Default()
	if err != nil {
		return map[string]string{}
	}
	names, err := s.ModelNames()
	if err != nil {
		return map[string]string{}
	}
	return names
}

func saveModelName(id, name string) {
	s, err := store.Default()
	if err != nil {
		return
	}
	_ = s.PutModelName(id, name)
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

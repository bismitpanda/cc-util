package claude

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

func ActiveOAuthAccount() (map[string]any, error) {
	path := GlobalFile()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	var doc struct {
		OAuthAccount map[string]any `json:"oauthAccount"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if doc.OAuthAccount == nil {
		return map[string]any{}, nil
	}
	return doc.OAuthAccount, nil
}

package main

import (
	"encoding/json/v2"
	"fmt"
)

// streamStatus distinguishes a completed response from a cleanly closed partial
// stream or an error carried inside HTTP 200. Text alone is not a successful run.
func streamStatus(data []byte, terminal string) (bool, error) {
	var event struct {
		Type string `json:"type"`
	}

	if json.Unmarshal(data, &event) != nil {
		return false, nil
	}

	switch event.Type {
	case "error", "response.failed", "response.incomplete":
		return false, fmt.Errorf("upstream stream failed: %s", event.Type)
	}

	return event.Type == terminal, nil
}

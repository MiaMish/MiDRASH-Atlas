package provenance

import (
	"fmt"
	"strings"
	"time"
)

// AIRun records how and when an AI-produced JSON value was created.
// It describes provenance, not scholarly approval.
type AIRun struct {
	Provider    string    `json:"provider"`
	Model       string    `json:"model"`
	GeneratedAt time.Time `json:"generated_at"`
	Purpose     string    `json:"purpose"`
	PromptHash  string    `json:"prompt_hash"`
}

func (run AIRun) Validate() error {
	required := map[string]string{
		"provider":    run.Provider,
		"model":       run.Model,
		"purpose":     run.Purpose,
		"prompt_hash": run.PromptHash,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("ai_provenance.%s is required", name)
		}
	}
	if run.GeneratedAt.IsZero() {
		return fmt.Errorf("ai_provenance.generated_at is required")
	}
	return nil
}

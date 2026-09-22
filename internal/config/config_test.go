package config

import (
	"testing"

	"github.com/opencode-ai/dhriti/internal/llm/models"
)

func TestEnsureRequiredAgentsFillsMissingEntries(t *testing.T) {
	cfg = &Config{}

	ensureRequiredAgents()

	for _, name := range []AgentName{AgentCoder, AgentSummarizer, AgentTask, AgentTitle} {
		if _, ok := cfg.Agents[name]; !ok {
			t.Fatalf("expected agent %q to be present", name)
		}
	}
	if got := cfg.Agents[AgentCoder].Model; got != models.GPT41 {
		t.Fatalf("coder model = %q, want %q", got, models.GPT41)
	}
	if got := cfg.Agents[AgentTitle].MaxTokens; got != 80 {
		t.Fatalf("title max tokens = %d, want 80", got)
	}
}

func TestEnsureRequiredAgentsKeepsConfiguredModels(t *testing.T) {
	cfg = &Config{
		Agents: map[AgentName]Agent{
			AgentCoder: {Model: models.Gemini25, MaxTokens: 1234},
		},
	}

	ensureRequiredAgents()

	if got := cfg.Agents[AgentCoder].Model; got != models.Gemini25 {
		t.Fatalf("coder model = %q, want %q", got, models.Gemini25)
	}
	if got := cfg.Agents[AgentCoder].MaxTokens; got != 1234 {
		t.Fatalf("coder max tokens = %d, want 1234", got)
	}
	if _, ok := cfg.Agents[AgentTitle]; !ok {
		t.Fatal("expected title agent to be filled in")
	}
}

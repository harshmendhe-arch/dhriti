// Package config manages application configuration from various sources.
package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencode-ai/dhriti/internal/llm/models"
	"github.com/opencode-ai/dhriti/internal/logging"
	"github.com/spf13/viper"
)

// MCPType defines the type of MCP (Model Control Protocol) server.
type MCPType string

// Supported MCP types
const (
	MCPStdio MCPType = "stdio"
	MCPSse   MCPType = "sse"
)

// MCPServer defines the configuration for a Model Control Protocol server.
type MCPServer struct {
	Command string            `json:"command"`
	Env     []string          `json:"env"`
	Args    []string          `json:"args"`
	Type    MCPType           `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type AgentName string

const (
	AgentCoder      AgentName = "coder"
	AgentSummarizer AgentName = "summarizer"
	AgentTask       AgentName = "task"
	AgentTitle      AgentName = "title"
)

// Agent defines configuration for different LLM models and their token limits.
type Agent struct {
	Model           models.ModelID `json:"model"`
	MaxTokens       int64          `json:"maxTokens"`
	ReasoningEffort string         `json:"reasoningEffort"` // For openai models low,medium,heigh
}

// Provider defines configuration for an LLM provider.
type Provider struct {
	APIKey       string `json:"apiKey"`
	Disabled     bool   `json:"disabled"`
	RefreshToken string `json:"refreshToken,omitempty"`
}

// Data defines storage configuration.
type Data struct {
	Directory string `json:"directory,omitempty"`
}

// LSPConfig defines configuration for Language Server Protocol integration.
type LSPConfig struct {
	Disabled bool     `json:"enabled"`
	Command  string   `json:"command"`
	Args     []string `json:"args"`
	Options  any      `json:"options"`
}

// TUIConfig defines the configuration for the Terminal User Interface.
type TUIConfig struct {
	Theme string `json:"theme,omitempty"`
}

// ShellConfig defines the configuration for the shell used by the bash tool.
type ShellConfig struct {
	Path string `json:"path,omitempty"`
	Args []string `json:"args,omitempty"`
}

// GatewayConfig configures a remote WebSocket gateway that speaks the
// OpenAI Realtime protocol. When URL is set, all LLM traffic routes
// through the gateway and local OpenAI/Gemini API keys are not required.
type GatewayConfig struct {
	URL    string `json:"url,omitempty"`
	APIKey string `json:"apiKey,omitempty"`
	Model  string `json:"model,omitempty"`
}

// Config is the main configuration structure for the application.
type Config struct {
	Data         Data                              `json:"data"`
	WorkingDir   string                            `json:"wd,omitempty"`
	MCPServers   map[string]MCPServer              `json:"mcpServers,omitempty"`
	Providers    map[models.ModelProvider]Provider `json:"providers,omitempty"`
	LSP          map[string]LSPConfig              `json:"lsp,omitempty"`
	Agents       map[AgentName]Agent               `json:"agents,omitempty"`
	Debug        bool                              `json:"debug,omitempty"`
	DebugLSP     bool                              `json:"debugLSP,omitempty"`
	ContextPaths []string                          `json:"contextPaths,omitempty"`
	TUI          TUIConfig                         `json:"tui"`
	Shell        ShellConfig                       `json:"shell,omitempty"`
	AutoCompact  bool                              `json:"autoCompact,omitempty"`
	Gateway      GatewayConfig                     `json:"gateway,omitempty"`
}

// GatewayEnabled reports whether a remote WebSocket gateway is configured.
func (c *Config) GatewayEnabled() bool {
	return c != nil && strings.TrimSpace(c.Gateway.URL) != ""
}

// Application constants
const (
	defaultDataDirectory = ".dhriti"
	defaultLogLevel      = "info"
	appName              = "dhriti"

	MaxTokensFallbackDefault = 4096
)

var defaultContextPaths = []string{
	".github/copilot-instructions.md",
	".cursorrules",
	".cursor/rules/",
	"CLAUDE.md",
	"CLAUDE.local.md",
	"dhriti.md",
	"dhriti.local.md",
	"Dhriti.md",
	"Dhriti.local.md",
	"DHRITI.md",
	"DHRITI.local.md",
}

// Global configuration instance
var cfg *Config

// Load initializes the configuration from environment variables and config files.
// If debug is true, debug mode is enabled and log level is set to debug.
// It returns an error if configuration loading fails.
func Load(workingDir string, debug bool) (*Config, error) {
	if cfg != nil {
		return cfg, nil
	}

	cfg = &Config{
		WorkingDir: workingDir,
		MCPServers: make(map[string]MCPServer),
		Providers:  make(map[models.ModelProvider]Provider),
		LSP:        make(map[string]LSPConfig),
	}

	configureViper()
	setDefaults(debug)

	// Read global config
	if err := readConfig(viper.ReadInConfig()); err != nil {
		return cfg, err
	}

	// Load and merge local config
	mergeLocalConfig(workingDir)

	setProviderDefaults()

	// Apply configuration to the struct
	if err := viper.Unmarshal(cfg); err != nil {
		return cfg, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	applyDefaultValues()
	defaultLevel := slog.LevelInfo
	if cfg.Debug {
		defaultLevel = slog.LevelDebug
	}
	if os.Getenv("DHRITI_DEV_DEBUG") == "true" {
		loggingFile := fmt.Sprintf("%s/%s", cfg.Data.Directory, "debug.log")
		messagesPath := fmt.Sprintf("%s/%s", cfg.Data.Directory, "messages")

		// if file does not exist create it
		if _, err := os.Stat(loggingFile); os.IsNotExist(err) {
			if err := os.MkdirAll(cfg.Data.Directory, 0o755); err != nil {
				return cfg, fmt.Errorf("failed to create directory: %w", err)
			}
			if _, err := os.Create(loggingFile); err != nil {
				return cfg, fmt.Errorf("failed to create log file: %w", err)
			}
		}

		if _, err := os.Stat(messagesPath); os.IsNotExist(err) {
			if err := os.MkdirAll(messagesPath, 0o756); err != nil {
				return cfg, fmt.Errorf("failed to create directory: %w", err)
			}
		}
		logging.MessageDir = messagesPath

		sloggingFileWriter, err := os.OpenFile(loggingFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
		if err != nil {
			return cfg, fmt.Errorf("failed to open log file: %w", err)
		}
		// Configure logger
		logger := slog.New(slog.NewTextHandler(sloggingFileWriter, &slog.HandlerOptions{
			Level: defaultLevel,
		}))
		slog.SetDefault(logger)
	} else {
		// Configure logger
		logger := slog.New(slog.NewTextHandler(logging.NewWriter(), &slog.HandlerOptions{
			Level: defaultLevel,
		}))
		slog.SetDefault(logger)
	}

	// Validate configuration
	if err := Validate(); err != nil {
		return cfg, fmt.Errorf("config validation failed: %w", err)
	}

	// Always materialize required agents so startup never fails with
	// "agent coder not found" when no keys/gateway are configured yet.
	// Credentials are checked later when the provider client is created.
	ensureRequiredAgents()
	return cfg, nil
}

// ensureRequiredAgents fills in the agents the app always needs.
// It runs after Validate so a missing provider yields a clear
// createAgentProvider error instead of an unknown-agent error, and so
// `dhriti login` can Load config before any keys exist.
func ensureRequiredAgents() {
	if cfg.Agents == nil {
		cfg.Agents = make(map[AgentName]Agent)
	}

	defaults := map[AgentName]Agent{
		AgentCoder:      {Model: models.GPT41},
		AgentSummarizer: {Model: models.GPT41},
		AgentTask:       {Model: models.GPT41Mini},
		AgentTitle:      {Model: models.GPT41Mini, MaxTokens: 80},
	}
	for name, def := range defaults {
		agent, ok := cfg.Agents[name]
		if !ok {
			cfg.Agents[name] = def
			continue
		}
		if agent.Model == "" {
			agent.Model = def.Model
		}
		if name == AgentTitle {
			agent.MaxTokens = 80
		}
		cfg.Agents[name] = agent
	}
}

// configureViper sets up viper's configuration paths and environment variables.
func configureViper() {
	viper.SetConfigName(fmt.Sprintf(".%s", appName))
	viper.SetConfigType("json")
	viper.AddConfigPath("$HOME")
	viper.AddConfigPath(fmt.Sprintf("$XDG_CONFIG_HOME/%s", appName))
	viper.AddConfigPath(fmt.Sprintf("$HOME/.config/%s", appName))
	viper.SetEnvPrefix(strings.ToUpper(appName))
	// DHRITI_GATEWAY_URL → gateway.url (dots are invalid in env names).
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
}

// setDefaults configures default values for configuration options.
func setDefaults(debug bool) {
	viper.SetDefault("data.directory", defaultDataDirectory)
	viper.SetDefault("contextPaths", defaultContextPaths)
	viper.SetDefault("tui.theme", "dhriti")
	viper.SetDefault("autoCompact", true)

	// Set default shell from environment or fallback to /bin/bash
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		shellPath = "/bin/bash"
	}
	viper.SetDefault("shell.path", shellPath)
	viper.SetDefault("shell.args", []string{"-l"})

	if debug {
		viper.SetDefault("debug", true)
		viper.Set("log.level", "debug")
	} else {
		viper.SetDefault("debug", false)
		viper.SetDefault("log.level", defaultLogLevel)
	}
}

// setProviderDefaults configures LLM provider defaults based on provider provided by
// environment variables and configuration file.
func setProviderDefaults() {
	// Remote WebSocket gateway: when set, all traffic routes through it and
	// local OpenAI/Gemini API keys are not required.
	if url := strings.TrimSpace(viper.GetString("gateway.url")); url != "" {
		viper.SetDefault("agents.coder.model", models.GPT41)
		viper.SetDefault("agents.summarizer.model", models.GPT41)
		viper.SetDefault("agents.task.model", models.GPT41Mini)
		viper.SetDefault("agents.title.model", models.GPT41Mini)
		return
	}

	// Set all API keys we can find in the environment
	// Note: Viper does not default if the json apiKey is ""
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		viper.SetDefault("providers.openai.apiKey", apiKey)
	}
	if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
		viper.SetDefault("providers.gemini.apiKey", apiKey)
	}

	// Use this order to set the default models
	// 1. OpenAI
	// 2. Google Gemini

	// OpenAI configuration
	if key := viper.GetString("providers.openai.apiKey"); strings.TrimSpace(key) != "" {
		viper.SetDefault("agents.coder.model", models.GPT41)
		viper.SetDefault("agents.summarizer.model", models.GPT41)
		viper.SetDefault("agents.task.model", models.GPT41Mini)
		viper.SetDefault("agents.title.model", models.GPT41Mini)
		return
	}

	// Google Gemini configuration
	if key := viper.GetString("providers.gemini.apiKey"); strings.TrimSpace(key) != "" {
		viper.SetDefault("agents.coder.model", models.Gemini25)
		viper.SetDefault("agents.summarizer.model", models.Gemini25)
		viper.SetDefault("agents.task.model", models.Gemini25Flash)
		viper.SetDefault("agents.title.model", models.Gemini25Flash)
		return
	}
}

// readConfig handles the result of reading a configuration file.
func readConfig(err error) error {
	if err == nil {
		return nil
	}

	// It's okay if the config file doesn't exist
	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		return nil
	}

	return fmt.Errorf("failed to read config: %w", err)
}

// mergeLocalConfig loads and merges configuration from the local directory.
func mergeLocalConfig(workingDir string) {
	local := viper.New()
	local.SetConfigName(fmt.Sprintf(".%s", appName))
	local.SetConfigType("json")
	local.AddConfigPath(workingDir)

	// Merge local config if it exists
	if err := local.ReadInConfig(); err == nil {
		settings := local.AllSettings()
		// An empty gateway.url in a local file must not disable a gateway
		// configured globally (common: committed .dhriti.json with "url": "").
		if g, ok := settings["gateway"].(map[string]any); ok {
			if u, ok := g["url"].(string); ok && strings.TrimSpace(u) == "" {
				delete(g, "url")
				if len(g) == 0 {
					delete(settings, "gateway")
				}
			}
		}
		viper.MergeConfigMap(settings)
	}
}

// applyDefaultValues sets default values for configuration fields that need processing.
func applyDefaultValues() {
	// Set default MCP type if not specified
	for k, v := range cfg.MCPServers {
		if v.Type == "" {
			v.Type = MCPStdio
			cfg.MCPServers[k] = v
		}
	}
}

// It validates model IDs and providers, ensuring they are supported.
func validateAgent(cfg *Config, name AgentName, agent Agent) error {
	// Check if model exists
	// TODO:	If a copilot model is specified, but model is not found,
	// 		 	it might be new model. The https://api.githubcopilot.com/models
	// 		 	endpoint should be queried to validate if the model is supported.
	model, modelExists := models.SupportedModels[agent.Model]
	if !modelExists {
		logging.Warn("unsupported model configured, reverting to default",
			"agent", name,
			"configured_model", agent.Model)

		// Set default model based on available providers
		if setDefaultModelForAgent(name) {
			logging.Info("set default model for agent", "agent", name, "model", cfg.Agents[name].Model)
		} else {
			return fmt.Errorf("no valid provider available for agent %s", name)
		}
		return nil
	}

	// Check if provider for the model is configured
	provider := model.Provider
	providerCfg, providerExists := cfg.Providers[provider]

	// Gateway mode: provider key/disabled checks are skipped — transport is the gateway.
	if !cfg.GatewayEnabled() {
		if !providerExists {
			// Provider not configured, check if we have environment variables
			apiKey := getProviderAPIKey(provider)
			if apiKey == "" {
				logging.Warn("provider not configured for model, reverting to default",
					"agent", name,
					"model", agent.Model,
					"provider", provider)

				// Set default model based on available providers
				if setDefaultModelForAgent(name) {
					logging.Info("set default model for agent", "agent", name, "model", cfg.Agents[name].Model)
				} else {
					return fmt.Errorf("no valid provider available for agent %s", name)
				}
			} else {
				// Add provider with API key from environment
				cfg.Providers[provider] = Provider{
					APIKey: apiKey,
				}
				logging.Info("added provider from environment", "provider", provider)
			}
		} else if providerCfg.Disabled || providerCfg.APIKey == "" {
			// Provider is disabled or has no API key
			logging.Warn("provider is disabled or has no API key, reverting to default",
				"agent", name,
				"model", agent.Model,
				"provider", provider)

			// Set default model based on available providers
			if setDefaultModelForAgent(name) {
				logging.Info("set default model for agent", "agent", name, "model", cfg.Agents[name].Model)
			} else {
				return fmt.Errorf("no valid provider available for agent %s", name)
			}
		}
	}

	// Validate max tokens
	if agent.MaxTokens <= 0 {
		logging.Warn("invalid max tokens, setting to default",
			"agent", name,
			"model", agent.Model,
			"max_tokens", agent.MaxTokens)

		// Update the agent with default max tokens
		updatedAgent := cfg.Agents[name]
		if model.DefaultMaxTokens > 0 {
			updatedAgent.MaxTokens = model.DefaultMaxTokens
		} else {
			updatedAgent.MaxTokens = MaxTokensFallbackDefault
		}
		cfg.Agents[name] = updatedAgent
	} else if model.ContextWindow > 0 && agent.MaxTokens > model.ContextWindow/2 {
		// Ensure max tokens doesn't exceed half the context window (reasonable limit)
		logging.Warn("max tokens exceeds half the context window, adjusting",
			"agent", name,
			"model", agent.Model,
			"max_tokens", agent.MaxTokens,
			"context_window", model.ContextWindow)

		// Update the agent with adjusted max tokens
		updatedAgent := cfg.Agents[name]
		updatedAgent.MaxTokens = model.ContextWindow / 2
		cfg.Agents[name] = updatedAgent
	}

	// Validate reasoning effort for models that support reasoning
	if model.CanReason && provider == models.ProviderOpenAI {
		if agent.ReasoningEffort == "" {
			// Set default reasoning effort for models that support it
			logging.Info("setting default reasoning effort for model that supports reasoning",
				"agent", name,
				"model", agent.Model)

			// Update the agent with default reasoning effort
			updatedAgent := cfg.Agents[name]
			updatedAgent.ReasoningEffort = "medium"
			cfg.Agents[name] = updatedAgent
		} else {
			// Check if reasoning effort is valid (low, medium, high)
			effort := strings.ToLower(agent.ReasoningEffort)
			if effort != "low" && effort != "medium" && effort != "high" {
				logging.Warn("invalid reasoning effort, setting to medium",
					"agent", name,
					"model", agent.Model,
					"reasoning_effort", agent.ReasoningEffort)

				// Update the agent with valid reasoning effort
				updatedAgent := cfg.Agents[name]
				updatedAgent.ReasoningEffort = "medium"
				cfg.Agents[name] = updatedAgent
			}
		}
	} else if !model.CanReason && agent.ReasoningEffort != "" {
		// Model doesn't support reasoning but reasoning effort is set
		logging.Warn("model doesn't support reasoning but reasoning effort is set, ignoring",
			"agent", name,
			"model", agent.Model,
			"reasoning_effort", agent.ReasoningEffort)

		// Update the agent to remove reasoning effort
		updatedAgent := cfg.Agents[name]
		updatedAgent.ReasoningEffort = ""
		cfg.Agents[name] = updatedAgent
	}

	return nil
}

// Validate checks if the configuration is valid and applies defaults where needed.
func Validate() error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	// Gateway mode: ensure provider entries exist so agent creation succeeds
	// without local API keys (keys are optional on the gateway path).
	if cfg.GatewayEnabled() {
		if cfg.Providers == nil {
			cfg.Providers = make(map[models.ModelProvider]Provider)
		}
		for _, p := range []models.ModelProvider{models.ProviderOpenAI, models.ProviderGemini} {
			if existing, ok := cfg.Providers[p]; !ok || existing.Disabled {
				cfg.Providers[p] = Provider{Disabled: false}
			}
		}
	}

	// Validate agent models
	for name, agent := range cfg.Agents {
		if err := validateAgent(cfg, name, agent); err != nil {
			return err
		}
	}

	// Validate providers (skipped in gateway mode — keys live on the gateway)
	if !cfg.GatewayEnabled() {
		for provider, providerCfg := range cfg.Providers {
			if providerCfg.APIKey == "" && !providerCfg.Disabled {
				fmt.Printf("provider has no API key, marking as disabled %s", provider)
				logging.Warn("provider has no API key, marking as disabled", "provider", provider)
				providerCfg.Disabled = true
				cfg.Providers[provider] = providerCfg
			}
		}
	}

	// Validate LSP configurations
	for language, lspConfig := range cfg.LSP {
		if lspConfig.Command == "" && !lspConfig.Disabled {
			logging.Warn("LSP configuration has no command, marking as disabled", "language", language)
			lspConfig.Disabled = true
			cfg.LSP[language] = lspConfig
		}
	}

	return nil
}

// getProviderAPIKey gets the API key for a provider from environment variables
func getProviderAPIKey(provider models.ModelProvider) string {
	switch provider {
	case models.ProviderOpenAI:
		return os.Getenv("OPENAI_API_KEY")
	case models.ProviderGemini:
		return os.Getenv("GEMINI_API_KEY")
	}
	return ""
}

// setDefaultModelForAgent sets a default model for an agent based on available providers
func setDefaultModelForAgent(agent AgentName) bool {
	// Gateway mode: default models without requiring local provider keys.
	if cfg.GatewayEnabled() {
		var model models.ModelID
		maxTokens := int64(5000)
		reasoningEffort := ""

		switch agent {
		case AgentTitle:
			model = models.GPT41Mini
			maxTokens = 80
		case AgentTask:
			model = models.GPT41Mini
		default:
			model = models.GPT41
		}

		if modelInfo, ok := models.SupportedModels[model]; ok && modelInfo.CanReason {
			reasoningEffort = "medium"
		}

		cfg.Agents[agent] = Agent{
			Model:           model,
			MaxTokens:       maxTokens,
			ReasoningEffort: reasoningEffort,
		}
		return true
	}

	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		var model models.ModelID
		maxTokens := int64(5000)
		reasoningEffort := ""

		switch agent {
		case AgentTitle:
			model = models.GPT41Mini
			maxTokens = 80
		case AgentTask:
			model = models.GPT41Mini
		default:
			model = models.GPT41
		}

		// Check if model supports reasoning
		if modelInfo, ok := models.SupportedModels[model]; ok && modelInfo.CanReason {
			reasoningEffort = "medium"
		}

		cfg.Agents[agent] = Agent{
			Model:           model,
			MaxTokens:       maxTokens,
			ReasoningEffort: reasoningEffort,
		}
		return true
	}

	if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
		var model models.ModelID
		maxTokens := int64(5000)

		if agent == AgentTitle {
			model = models.Gemini25Flash
			maxTokens = 80
		} else {
			model = models.Gemini25
		}

		cfg.Agents[agent] = Agent{
			Model:     model,
			MaxTokens: maxTokens,
		}
		return true
	}

	return false
}

func updateCfgFile(updateCfg func(config *Config)) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	// Get the config file path
	configFile := viper.ConfigFileUsed()
	var configData []byte
	if configFile == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		configFile = filepath.Join(homeDir, fmt.Sprintf(".%s.json", appName))
		logging.Info("config file not found, creating new one", "path", configFile)
		configData = []byte(`{}`)
	} else {
		// Read the existing config file
		data, err := os.ReadFile(configFile)
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		configData = data
	}

	// Parse the JSON
	var userCfg *Config
	if err := json.Unmarshal(configData, &userCfg); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}
	if userCfg == nil {
		// File content was literally `null`.
		userCfg = &Config{}
	}

	updateCfg(userCfg)

	// Write the updated config back to file
	updatedData, err := json.MarshalIndent(userCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configFile, updatedData, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Get returns the current configuration.
// It's safe to call this function multiple times.
func Get() *Config {
	return cfg
}

// WorkingDirectory returns the current working directory from the configuration.
func WorkingDirectory() string {
	if cfg == nil {
		panic("config not loaded")
	}
	return cfg.WorkingDir
}

func UpdateAgentModel(agentName AgentName, modelID models.ModelID) error {
	if cfg == nil {
		panic("config not loaded")
	}

	existingAgentCfg := cfg.Agents[agentName]

	model, ok := models.SupportedModels[modelID]
	if !ok {
		return fmt.Errorf("model %s not supported", modelID)
	}

	maxTokens := existingAgentCfg.MaxTokens
	if model.DefaultMaxTokens > 0 {
		maxTokens = model.DefaultMaxTokens
	}

	newAgentCfg := Agent{
		Model:           modelID,
		MaxTokens:       maxTokens,
		ReasoningEffort: existingAgentCfg.ReasoningEffort,
	}
	cfg.Agents[agentName] = newAgentCfg

	if err := validateAgent(cfg, agentName, newAgentCfg); err != nil {
		// revert config update on failure
		cfg.Agents[agentName] = existingAgentCfg
		return fmt.Errorf("failed to update agent model: %w", err)
	}

	return updateCfgFile(func(config *Config) {
		if config.Agents == nil {
			config.Agents = make(map[AgentName]Agent)
		}
		config.Agents[agentName] = newAgentCfg
	})
}

// UpdateTheme updates the theme in the configuration and writes it to the config file.
func UpdateTheme(themeName string) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}

	// Update the in-memory config
	cfg.TUI.Theme = themeName

	// Update the file config
	return updateCfgFile(func(config *Config) {
		config.TUI.Theme = themeName
	})
}

// SaveProviderKey stores a provider API key (or OAuth access token) in config.
// provider is "openai" or "gemini".
func SaveProviderKey(provider string, key string) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	p := models.ModelProvider(provider)

	// Mirror into the live config so the current process can use it immediately.
	if cfg.Providers == nil {
		cfg.Providers = make(map[models.ModelProvider]Provider)
	}
	existing := cfg.Providers[p]
	existing.APIKey = key
	existing.Disabled = false
	cfg.Providers[p] = existing

	return updateCfgFile(func(c *Config) {
		if c.Providers == nil {
			c.Providers = make(map[models.ModelProvider]Provider)
		}
		e := c.Providers[p]
		e.APIKey = key
		e.Disabled = false
		c.Providers[p] = e
	})
}

// SaveGeminiRefreshToken persists the Google OAuth refresh token (optional).
func SaveGeminiRefreshToken(token string) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	if cfg.Providers == nil {
		cfg.Providers = make(map[models.ModelProvider]Provider)
	}
	e := cfg.Providers[models.ProviderGemini]
	e.RefreshToken = token
	cfg.Providers[models.ProviderGemini] = e

	return updateCfgFile(func(c *Config) {
		if c.Providers == nil {
			c.Providers = make(map[models.ModelProvider]Provider)
		}
		pe := c.Providers[models.ProviderGemini]
		pe.RefreshToken = token
		c.Providers[models.ProviderGemini] = pe
	})
}

// ValidateGatewayURL trims and checks a gateway endpoint. Only ws:// and
// wss:// schemes are accepted so typos like https:// fail at login time
// instead of with an opaque dial error later.
func ValidateGatewayURL(raw string) (string, error) {
	url := strings.TrimSpace(raw)
	if url == "" {
		return "", fmt.Errorf("gateway URL is empty")
	}
	u, err := neturl.Parse(url)
	if err != nil {
		return "", fmt.Errorf("invalid gateway URL: %w", err)
	}
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return "", fmt.Errorf("gateway URL must use ws:// or wss:// (got %q)", u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("gateway URL is missing a host")
	}
	return u.String(), nil
}

// SaveGateway writes gateway.url / gateway.apiKey / gateway.model into config.
// Empty apiKey means "use the OpenAI provider key as Bearer".
func SaveGateway(url, apiKey, model string) error {
	if cfg == nil {
		return fmt.Errorf("config not loaded")
	}
	clean, err := ValidateGatewayURL(url)
	if err != nil {
		return err
	}
	cfg.Gateway.URL = clean
	cfg.Gateway.APIKey = strings.TrimSpace(apiKey)
	cfg.Gateway.Model = strings.TrimSpace(model)

	return updateCfgFile(func(c *Config) {
		c.Gateway.URL = clean
		c.Gateway.APIKey = strings.TrimSpace(apiKey)
		c.Gateway.Model = strings.TrimSpace(model)
	})
}

// GatewayBearerToken returns the token to send as Authorization: Bearer
// for gateway WebSocket dials: gateway.apiKey if set, else OpenAI provider key.
func GatewayBearerToken() string {
	if cfg == nil {
		return ""
	}
	if t := strings.TrimSpace(cfg.Gateway.APIKey); t != "" {
		return t
	}
	if cfg.Providers != nil {
		if p, ok := cfg.Providers[models.ProviderOpenAI]; ok {
			return strings.TrimSpace(p.APIKey)
		}
	}
	return ""
}

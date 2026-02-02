package providers

import (
	"os"
	"time"
)

// PresetProvider creates a provider from a preset configuration.
func PresetProvider(name string) *OpenAIAPIProvider {
	config, ok := Presets[name]
	if !ok {
		return nil
	}

	// Expand environment variables in API key
	if config.APIKey != "" && config.APIKey[0] == '$' {
		envKey := config.APIKey[1:]
		config.APIKey = os.Getenv(envKey)
	}

	return NewOpenAIAPIProvider(config)
}

// Presets contains preset configurations for common providers.
var Presets = map[string]OpenAIAPIConfig{
	"ollama": {
		Name:         "ollama",
		DisplayName:  "Ollama (local)",
		BaseURL:      "http://localhost:11434/v1",
		APIKey:       "", // Not required
		DefaultModel: "llama3:8b",
		Timeout:      60 * time.Second,
	},
	"lmstudio": {
		Name:         "lmstudio",
		DisplayName:  "LM Studio (local)",
		BaseURL:      "http://localhost:1234",
		APIKey:       "$LM_API_TOKEN", // Optional
		DefaultModel: "",              // Auto-select first loaded model
		Timeout:      60 * time.Second,
	},
	"openai": {
		Name:         "openai",
		DisplayName:  "OpenAI",
		BaseURL:      "https://api.openai.com/v1",
		APIKey:       "$OPENAI_API_KEY", // From environment
		DefaultModel: "gpt-4o",
		Timeout:      60 * time.Second,
	},
	"openrouter": {
		Name:         "openrouter",
		DisplayName:  "OpenRouter",
		BaseURL:      "https://openrouter.ai/api/v1",
		APIKey:       "$OPENROUTER_API_KEY", // From environment
		DefaultModel: "anthropic/claude-3.5-sonnet",
		Timeout:      60 * time.Second,
	},
	"together": {
		Name:         "together",
		DisplayName:  "Together.ai",
		BaseURL:      "https://api.together.xyz/v1",
		APIKey:       "$TOGETHER_API_KEY",
		DefaultModel: "meta-llama/Llama-3-70b-chat-hf",
		Timeout:      60 * time.Second,
	},
	"groq": {
		Name:         "groq",
		DisplayName:  "Groq",
		BaseURL:      "https://api.groq.com/openai/v1",
		APIKey:       "$GROQ_API_KEY",
		DefaultModel: "llama3-70b-8192",
		Timeout:      30 * time.Second,
	},
}

// AllPresetNames returns all available preset names.
func AllPresetNames() []string {
	names := make([]string, 0, len(Presets))
	for name := range Presets {
		names = append(names, name)
	}
	return names
}

// GetPresets returns all preset configurations with expanded environment variables.
// This is used to register all preset providers at startup.
func GetPresets() []OpenAIAPIConfig {
	configs := make([]OpenAIAPIConfig, 0, len(Presets))
	for _, config := range Presets {
		// Expand environment variables in API key
		if config.APIKey != "" && len(config.APIKey) > 0 && config.APIKey[0] == '$' {
			envKey := config.APIKey[1:]
			config.APIKey = os.Getenv(envKey)
		}
		configs = append(configs, config)
	}
	return configs
}

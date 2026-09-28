package model

import "testing"

// TestModelConfigResolvesProviderDefaults 验证默认地址和 Ollama API Key。
func TestModelConfigResolvesProviderDefaults(t *testing.T) {
	openAI := &ModelConfig{Provider: ProviderOpenAICompatible}
	if openAI.ResolvedBaseURL() != "https://api.openai.com/v1" {
		t.Fatalf("OpenAI default base URL = %q", openAI.ResolvedBaseURL())
	}
	ollama := &ModelConfig{Provider: ProviderOllama}
	if ollama.ResolvedAPIKey() != "ollama" || ollama.ResolvedBaseURL() != "http://localhost:11434/v1" {
		t.Fatalf("Ollama defaults = (%q, %q)", ollama.ResolvedAPIKey(), ollama.ResolvedBaseURL())
	}
}

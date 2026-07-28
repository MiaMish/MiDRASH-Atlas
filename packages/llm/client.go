package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	ProviderOpenAI = "openai"
	ProviderOllama = "ollama"
)

type Config struct {
	OpenAIAPIKey  string
	OllamaBaseURL string
	HTTPClient    *http.Client
}

type Client struct {
	openAIKey     string
	ollamaBaseURL string
	httpClient    *http.Client
}

func NewClient(cfg Config) *Client {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Minute}
	}
	return &Client{
		openAIKey:     strings.TrimSpace(cfg.OpenAIAPIKey),
		ollamaBaseURL: strings.TrimSpace(cfg.OllamaBaseURL),
		httpClient:    httpClient,
	}
}

func (c *Client) Exec(ctx context.Context, provider, model, prompt string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case ProviderOpenAI:
		return c.execOpenAI(ctx, model, prompt)
	case ProviderOllama:
		return c.execOllama(ctx, model, prompt)
	default:
		return "", fmt.Errorf("unsupported LLM provider %q", provider)
	}
}

func (c *Client) execOpenAI(ctx context.Context, model, prompt string) (string, error) {
	if c.openAIKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY is empty")
	}
	if strings.TrimSpace(model) == "" {
		return "", fmt.Errorf("OpenAI model is empty")
	}
	payload := map[string]any{
		"model": model,
		"input": prompt,
		"store": false,
		"text":  map[string]any{"verbosity": "low"},
	}
	var response struct {
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := c.postJSON(ctx, "https://api.openai.com/v1/responses", payload, map[string]string{
		"Authorization": "Bearer " + c.openAIKey,
	}, &response); err != nil {
		return "", err
	}
	if response.Error != nil {
		return "", fmt.Errorf("OpenAI response error: %s", response.Error.Message)
	}
	var parts []string
	for _, item := range response.Output {
		for _, content := range item.Content {
			if content.Type == "output_text" && strings.TrimSpace(content.Text) != "" {
				parts = append(parts, content.Text)
			}
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("OpenAI response contained no output text")
	}
	return strings.Join(parts, "\n"), nil
}

func (c *Client) execOllama(ctx context.Context, model, prompt string) (string, error) {
	if c.ollamaBaseURL == "" {
		return "", fmt.Errorf("OLLAMA_BASE_URL is empty")
	}
	if strings.TrimSpace(model) == "" {
		return "", fmt.Errorf("Ollama model is empty")
	}
	base, err := url.Parse(c.ollamaBaseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("OLLAMA_BASE_URL must contain a valid scheme and host")
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/api/generate"
	var response struct {
		Response string `json:"response"`
		Error    string `json:"error"`
	}
	if err := c.postJSON(ctx, base.String(), map[string]any{
		"model": model, "prompt": prompt, "stream": false, "format": "json", "think": false,
	}, nil, &response); err != nil {
		return "", err
	}
	if response.Error != "" {
		return "", fmt.Errorf("Ollama response error: %s", response.Error)
	}
	if strings.TrimSpace(response.Response) == "" {
		return "", fmt.Errorf("Ollama response contained no generated text")
	}
	return response.Response, nil
}

func (c *Client) postJSON(ctx context.Context, endpoint string, payload any, headers map[string]string, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
		} else {
			data, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
			resp.Body.Close()
			if readErr != nil {
				return readErr
			}
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				if err := json.Unmarshal(data, out); err != nil {
					return fmt.Errorf("decode LLM response: %w", err)
				}
				return nil
			}
			lastErr = fmt.Errorf("LLM API returned %d: %s", resp.StatusCode, truncate(string(data), 512))
			if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
				return lastErr
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(1<<attempt) * time.Second):
		}
	}
	return lastErr
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}

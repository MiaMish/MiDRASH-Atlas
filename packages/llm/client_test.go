package llm

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestOpenAIResponsesClient(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://api.openai.com/v1/responses" {
			t.Fatalf("unexpected URL %s", request.URL)
		}
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("missing authorization header")
		}
		body, _ := io.ReadAll(request.Body)
		if !strings.Contains(string(body), `"store":false`) || !strings.Contains(string(body), `"input":"prompt"`) {
			t.Fatalf("unexpected payload: %s", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
			  "output":[{"content":[{"type":"output_text","text":"draft-json"}]}]
			}`)),
			Header: make(http.Header),
		}, nil
	})}
	client := NewClient(Config{OpenAIAPIKey: "secret", HTTPClient: httpClient})
	got, err := client.Exec(context.Background(), ProviderOpenAI, "test-model", "prompt")
	if err != nil {
		t.Fatal(err)
	}
	if got != "draft-json" {
		t.Fatalf("unexpected output %q", got)
	}
}

func TestOllamaGenerateClient(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://ollama.example/api/generate" {
			t.Fatalf("unexpected URL %s", request.URL)
		}
		body, _ := io.ReadAll(request.Body)
		if !strings.Contains(string(body), `"format":"json"`) || !strings.Contains(string(body), `"think":false`) {
			t.Fatalf("JSON/non-thinking mode not requested: %s", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"response":"draft-json","done":true}`)),
			Header:     make(http.Header),
		}, nil
	})}
	client := NewClient(Config{OllamaBaseURL: "https://ollama.example", HTTPClient: httpClient})
	got, err := client.Exec(context.Background(), ProviderOllama, "test-model", "prompt")
	if err != nil {
		t.Fatal(err)
	}
	if got != "draft-json" {
		t.Fatalf("unexpected output %q", got)
	}
}

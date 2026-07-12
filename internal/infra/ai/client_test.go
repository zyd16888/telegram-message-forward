package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpenAICompatibleClientResponsesAPI(t *testing.T) {
	var path string
	var maxOutputTokens int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		var body struct {
			MaxOutputTokens int `json:"max_output_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		maxOutputTokens = body.MaxOutputTokens
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"gpt-4o-mini","status":"completed","output_text":"responses ok","usage":{"input_tokens":2,"output_tokens":3,"total_tokens":5}}`))
	}))
	defer srv.Close()

	client := NewOpenAICompatibleClient(OpenAICompatibleConfig{
		BaseURL:      srv.URL,
		APIKey:       "test-key",
		APIType:      "responses",
		DefaultModel: "gpt-4o-mini",
		Timeout:      time.Second,
	})
	res, err := client.Generate(context.Background(), GenerateRequest{
		System:    "system",
		User:      "user",
		MaxTokens: 80,
	})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/responses" {
		t.Fatalf("path = %s, want /responses", path)
	}
	if maxOutputTokens != 80 {
		t.Fatalf("max_output_tokens = %d, want 80", maxOutputTokens)
	}
	if res.Text != "responses ok" || res.Usage.TotalTokens != 5 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestOpenAICompatibleClientChatCompletionsMultimodal(t *testing.T) {
	var content []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Role    string          `json:"role"`
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body.Messages[1].Content, &content); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"vision","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{}}`))
	}))
	defer srv.Close()

	client := NewOpenAICompatibleClient(OpenAICompatibleConfig{
		BaseURL: srv.URL, APIKey: "test-key", DefaultModel: "vision", Timeout: time.Second,
	})
	_, err := client.Generate(context.Background(), GenerateRequest{
		User: "analyze", Content: []ContentPart{
			{Type: "image", ImageURL: "data:image/png;base64,AA==", Detail: "high"},
			{Type: "text", Text: "second"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(content) != 3 || content[0]["type"] != "text" || content[1]["type"] != "image_url" {
		t.Fatalf("unexpected content: %#v", content)
	}
	image, ok := content[1]["image_url"].(map[string]any)
	if !ok || image["url"] != "data:image/png;base64,AA==" || image["detail"] != "high" {
		t.Fatalf("unexpected image content: %#v", content[1])
	}
}

func TestOpenAICompatibleClientResponsesMultimodal(t *testing.T) {
	var input []struct {
		Content []map[string]any `json:"content"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Input []struct {
				Content []map[string]any `json:"content"`
			} `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		input = body.Input
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"vision","status":"completed","output_text":"ok","usage":{}}`))
	}))
	defer srv.Close()

	client := NewOpenAICompatibleClient(OpenAICompatibleConfig{
		BaseURL: srv.URL, APIKey: "test-key", APIType: "responses", DefaultModel: "vision", Timeout: time.Second,
	})
	_, err := client.Generate(context.Background(), GenerateRequest{
		User: "analyze", Content: []ContentPart{{Type: "image", ImageURL: "data:image/jpeg;base64,AA==", Detail: "low"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(input) != 1 || len(input[0].Content) != 2 || input[0].Content[1]["type"] != "input_image" {
		t.Fatalf("unexpected input: %#v", input)
	}
	if input[0].Content[1]["image_url"] != "data:image/jpeg;base64,AA==" || input[0].Content[1]["detail"] != "low" {
		t.Fatalf("unexpected image input: %#v", input[0].Content[1])
	}
}

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

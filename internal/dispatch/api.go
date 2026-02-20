package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	defaultModel     = "claude-3-haiku-20240307"
	defaultBaseURL   = "https://api.anthropic.com"
	anthropicVersion = "2023-06-01"
	intentMaxTokens  = 1024
	apiTimeout       = 30 * time.Second
	maxResponseSize  = 64 * 1024 // 64KB limit for API response body
)

// Reuse a single http.Client for all API calls.
var httpClient = &http.Client{Timeout: apiTimeout}

// apiMessage represents a message in the Anthropic API format.
type apiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// apiRequest represents the Anthropic Messages API request body.
type apiRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system,omitempty"`
	Messages  []apiMessage `json:"messages"`
}

// apiResponse represents the Anthropic Messages API response.
type apiResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// analyzeIntent calls the Anthropic API to parse user input into structured tasks.
func analyzeIntent(ctx context.Context, apiKey, baseURL, model, systemPrompt, userPrompt string) (*IntentResponse, error) {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if model == "" {
		model = defaultModel
	}

	reqBody := apiRequest{
		Model:     model,
		MaxTokens: intentMaxTokens,
		System:    systemPrompt,
		Messages: []apiMessage{
			{Role: "user", Content: userPrompt},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	endpoint, err := url.JoinPath(baseURL, "v1/messages")
	if err != nil {
		return nil, fmt.Errorf("build API URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", anthropicVersion)
	if apiKey != "" {
		req.Header.Set("x-api-key", apiKey)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %.200s", resp.StatusCode, string(respBody))
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal API response: %w", err)
	}

	if apiResp.Error != nil {
		return nil, fmt.Errorf("API error: %s: %s", apiResp.Error.Type, apiResp.Error.Message)
	}

	// Extract text from response content
	var text string
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			text = block.Text
			break
		}
	}
	if text == "" {
		return nil, fmt.Errorf("no text content in API response")
	}

	// Parse the JSON response from Claude
	return parseIntentJSON(text)
}

// parseIntentJSON extracts and parses the IntentResponse from Claude's text output.
func parseIntentJSON(text string) (*IntentResponse, error) {
	// Try direct parse first
	var intent IntentResponse
	if err := json.Unmarshal([]byte(text), &intent); err == nil {
		return &intent, nil
	}

	// Try to find the first valid JSON object using json.Decoder.
	// This handles cases where Claude wraps JSON in markdown or prose containing braces.
	for i := 0; i < len(text); i++ {
		if text[i] != '{' {
			continue
		}
		dec := json.NewDecoder(bytes.NewReader([]byte(text[i:])))
		if err := dec.Decode(&intent); err == nil {
			return &intent, nil
		}
	}

	return nil, fmt.Errorf("could not parse intent JSON from response: %.200s", text)
}

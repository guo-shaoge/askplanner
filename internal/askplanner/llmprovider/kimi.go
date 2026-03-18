package llmprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

var retryBackoffs = []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second}

// KimiProvider implements Provider using the Moonshot (Kimi) API.
type KimiProvider struct {
	httpClient *http.Client
	endpoint   string
	apiKey     string
	model      string
}

// NewKimiProvider creates a Kimi provider.
func NewKimiProvider(apiKey, model, baseURL string) *KimiProvider {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	endpoint := base + "/v1/chat/completions"

	return &KimiProvider{
		httpClient: &http.Client{Timeout: 120 * time.Second},
		endpoint:   endpoint,
		apiKey:     apiKey,
		model:      model,
	}
}

func (k *KimiProvider) Name() string { return "kimi" }

func (k *KimiProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = k.model
	}

	apiReq := kimiRequest{
		Model:       model,
		Messages:    toKimiMessages(req.Messages),
		Temperature: req.Temperature,
	}
	if len(req.Tools) > 0 {
		apiReq.Tools = req.Tools
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	respBody, statusCode, err := k.doRequestWithRetry(ctx, body)
	if err != nil {
		return nil, err
	}

	if statusCode/100 != 2 {
		return nil, fmt.Errorf("API error status=%d body=%s", statusCode, string(respBody))
	}

	var apiResp kimiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w, body=%s", err, string(respBody))
	}
	if apiResp.Error != nil {
		return nil, fmt.Errorf("API error: %s (type=%s)", apiResp.Error.Message, apiResp.Error.Type)
	}
	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("empty choices in response")
	}

	choice := apiResp.Choices[0]
	msg := Message{
		Role:    choice.Message.Role,
		Content: parseContent(choice.Message.Content),
	}
	if len(choice.Message.ToolCalls) > 0 {
		msg.ToolCalls = make([]ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			msg.ToolCalls[i] = ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: ToolCallFunction{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}

	return &CompletionResponse{
		Message:      msg,
		FinishReason: choice.FinishReason,
		Usage: Usage{
			PromptTokens:     apiResp.Usage.PromptTokens,
			CompletionTokens: apiResp.Usage.CompletionTokens,
			TotalTokens:      apiResp.Usage.TotalTokens,
		},
	}, nil
}

// doRequestWithRetry sends the HTTP request and retries on 429 with exponential backoff.
func (k *KimiProvider) doRequestWithRetry(ctx context.Context, body []byte) ([]byte, int, error) {
	for attempt := 0; ; attempt++ {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, k.endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, 0, fmt.Errorf("build request: %w", err)
		}
		httpReq.Header.Set("Authorization", "Bearer "+k.apiKey)
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "application/json")

		resp, err := k.httpClient.Do(httpReq)
		if err != nil {
			return nil, 0, fmt.Errorf("call API: %w", err)
		}

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if err != nil {
			return nil, 0, fmt.Errorf("read response: %w", err)
		}

		if resp.StatusCode != 429 || attempt >= len(retryBackoffs) {
			return respBody, resp.StatusCode, nil
		}

		wait := retryBackoffs[attempt]
		log.Printf("[kimi] rate limited (429), retrying in %s (attempt %d/%d)", wait, attempt+1, len(retryBackoffs))

		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-time.After(wait):
		}
	}
}

// parseContent handles both string and array content formats from the API.
func parseContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}

	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var sb strings.Builder
		for _, p := range parts {
			if p.Type == "text" && strings.TrimSpace(p.Text) != "" {
				if sb.Len() > 0 {
					sb.WriteByte('\n')
				}
				sb.WriteString(strings.TrimSpace(p.Text))
			}
		}
		return sb.String()
	}

	return ""
}

func toKimiMessages(msgs []Message) []kimiMessage {
	out := make([]kimiMessage, len(msgs))
	for i, m := range msgs {
		km := kimiMessage{
			Role:    m.Role,
			Content: m.Content,
		}
		if m.ToolCallID != "" {
			km.ToolCallID = m.ToolCallID
		}
		if m.Name != "" {
			km.Name = m.Name
		}
		if len(m.ToolCalls) > 0 {
			km.ToolCalls = make([]kimiToolCall, len(m.ToolCalls))
			for j, tc := range m.ToolCalls {
				km.ToolCalls[j] = kimiToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: kimiToolCallFunction{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}
		out[i] = km
	}
	return out
}

// --- Kimi API wire types ---

type kimiRequest struct {
	Model       string           `json:"model"`
	Messages    []kimiMessage    `json:"messages"`
	Temperature float64          `json:"temperature,omitempty"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
}

type kimiMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Name       string         `json:"name,omitempty"`
	ToolCalls  []kimiToolCall `json:"tool_calls,omitempty"`
}

type kimiToolCall struct {
	ID       string               `json:"id"`
	Type     string               `json:"type"`
	Function kimiToolCallFunction `json:"function"`
}

type kimiToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type kimiResponse struct {
	Choices []kimiChoice `json:"choices"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

type kimiChoice struct {
	Message      kimiChoiceMessage `json:"message"`
	FinishReason string            `json:"finish_reason"`
}

type kimiChoiceMessage struct {
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	ToolCalls []kimiToolCall  `json:"tool_calls,omitempty"`
}

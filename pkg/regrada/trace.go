package regrada

import (
	"encoding/json"
	"time"
)

// Trace represents a single LLM API call trace
type Trace struct {
	TraceID          string        `json:"trace_id"`
	Timestamp        time.Time     `json:"timestamp"`
	Provider         string        `json:"provider"`
	Model            string        `json:"model"`
	Environment      string        `json:"environment,omitempty"`
	GitSHA           string        `json:"git_sha,omitempty"`
	GitBranch        string        `json:"git_branch,omitempty"`
	Request          TraceRequest  `json:"request"`
	Response         TraceResponse `json:"response"`
	Metrics          TraceMetrics  `json:"metrics"`
	RedactionApplied []string      `json:"redaction_applied,omitempty"`
	Tags             []string      `json:"tags,omitempty"`
}

// TraceRequest represents the request portion of a trace
type TraceRequest struct {
	Messages []Message       `json:"messages,omitempty"`
	Params   *SamplingParams `json:"params,omitempty"`
}

// TraceResponse represents the response portion of a trace
type TraceResponse struct {
	AssistantText string          `json:"assistant_text,omitempty"`
	ToolCalls     []ToolCall      `json:"tool_calls,omitempty"`
	Raw           json.RawMessage `json:"raw,omitempty"`
}

// TraceMetrics contains metrics for a trace
type TraceMetrics struct {
	LatencyMS int `json:"latency_ms,omitempty"`
	TokensIn  int `json:"tokens_in,omitempty"`
	TokensOut int `json:"tokens_out,omitempty"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// SamplingParams represents LLM sampling parameters
type SamplingParams struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"top_p,omitempty"`
	MaxOutputTokens *int     `json:"max_output_tokens,omitempty"`
	Stop            []string `json:"stop,omitempty"`
}

// ToolCall represents a function/tool call
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
	Response  json.RawMessage `json:"response,omitempty"`
}

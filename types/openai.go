package types

import (
	"context"
	"encoding/json"
)

// Copy for github.com/sashabaranov/go-openai/chat.go
// Chat message role defined by the OpenAI API.
const (
	ChatMessageRoleSystem    = "system"
	ChatMessageRoleUser      = "user"
	ChatMessageRoleAssistant = "assistant"
	ChatMessageRoleFunction  = "function"
	ChatMessageRoleTool      = "tool"
	ChatMessageRoleDeveloper = "developer"
)

// OpenAIChatClient interface for dependency injection and testing
type OpenAIChatClient interface {
	CreateChatCompletion(ctx context.Context, msg ChatCompletionRequest) (ChatCompletionResponse, error)
}

// ChatCompletionRequest represents a request structure for chat completion API.
type ChatCompletionRequest struct {
	Model               string                        `json:"model"`
	Messages            []ChatCompletionMessage       `json:"messages"`
	MaxTokens           int                           `json:"max_tokens,omitempty"`
	MaxCompletionTokens int                           `json:"max_completion_tokens,omitempty"`
	Temperature         float32                       `json:"temperature,omitempty"`
	TopP                float32                       `json:"top_p,omitempty"`
	N                   int                           `json:"n,omitempty"`
	Stream              bool                          `json:"stream,omitempty"`
	Stop                []string                      `json:"stop,omitempty"`
	ResponseFormat      *ChatCompletionResponseFormat `json:"response_format,omitempty"`
	Seed                *int                          `json:"seed,omitempty"`
	LogitBias           map[string]int                `json:"logit_bias,omitempty"`
	LogProbs            bool                          `json:"logprobs,omitempty"`
	TopLogProbs         int                           `json:"top_logprobs,omitempty"`
	User                string                        `json:"user,omitempty"`
	Tools               []Tool                        `json:"tools,omitempty"`
	ToolChoice          any                           `json:"tool_choice,omitempty"`
	StreamOptions       *StreamOptions                `json:"stream_options,omitempty"`
	ParallelToolCalls   any                           `json:"parallel_tool_calls,omitempty"`
	Store               bool                          `json:"store,omitempty"`
	ReasoningEffort     string                        `json:"reasoning_effort,omitempty"`
	Metadata            map[string]string             `json:"metadata,omitempty"`
	Prediction          *Prediction                   `json:"prediction,omitempty"`
	ChatTemplateKwargs  map[string]any                `json:"chat_template_kwargs,omitempty"`
	ServiceTier         string                        `json:"service_tier,omitempty"`
	Verbosity           string                        `json:"verbosity,omitempty"`
	SafetyIdentifier    string                        `json:"safety_identifier,omitempty"`
	GuidedChoice        []string                      `json:"guided_choice,omitempty"`
}

type ChatCompletionMessage struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	Refusal          string `json:"refusal,omitempty"`
	MultiContent     []ChatMessagePart
	Name             string        `json:"name,omitempty"`
	ReasoningContent string        `json:"reasoning_content,omitempty"`
	FunctionCall     *FunctionCall `json:"function_call,omitempty"`
	ToolCalls        []ToolCall    `json:"tool_calls,omitempty"`
	ToolCallID       string        `json:"tool_call_id,omitempty"`
}

type ChatMessagePart struct {
	Type     string               `json:"type,omitempty"`
	Text     string               `json:"text,omitempty"`
	ImageURL *ChatMessageImageURL `json:"image_url,omitempty"`
}

type ChatMessageImageURL struct {
	URL    string `json:"url,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type ChatCompletionResponseFormat struct {
	Type       string                                  `json:"type,omitempty"`
	JSONSchema *ChatCompletionResponseFormatJSONSchema `json:"json_schema,omitempty"`
}

type ChatCompletionResponseFormatJSONSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Schema      json.Marshaler `json:"schema"`
	Strict      bool           `json:"strict"`
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

type Prediction struct {
	Content string `json:"content"`
	Type    string `json:"type"`
}

type FunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type ToolCall struct {
	Index    *int         `json:"index,omitempty"`
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

/*****/

// ChatCompletionResponse represents a response structure for chat completion API.
type ChatCompletionResponse struct {
	ID                  string                 `json:"id"`
	Object              string                 `json:"object"`
	Created             int64                  `json:"created"`
	Model               string                 `json:"model"`
	Choices             []ChatCompletionChoice `json:"choices"`
	Usage               Usage                  `json:"usage"`
	SystemFingerprint   string                 `json:"system_fingerprint"`
	PromptFilterResults []struct {
		Index                int                  `json:"index"`
		ContentFilterResults ContentFilterResults `json:"content_filter_results,omitempty"`
	} `json:"prompt_filter_results,omitempty"`
	ServiceTier string `json:"service_tier,omitempty"`
}

type ChatCompletionChoice struct {
	Index        int                   `json:"index"`
	Message      ChatCompletionMessage `json:"message"`
	FinishReason string                `json:"finish_reason"`
	LogProbs     struct {
		Content          []LogProbContent `json:"content"`
		ReasoningContent []LogProbContent `json:"reasoning_content"`
	} `json:"logprobs,omitempty"`
	ContentFilterResults ContentFilterResults `json:"content_filter_results,omitempty"`
}

type LogProbContent struct {
	Token       string  `json:"token"`
	LogProb     float64 `json:"logprob"`
	Bytes       []byte  `json:"bytes,omitempty"` // Omitting the field if it is null
	TopLogProbs []struct {
		Token   string  `json:"token"`
		LogProb float64 `json:"logprob"`
		Bytes   []byte  `json:"bytes,omitempty"`
	} `json:"top_logprobs"`
}

type ContentFilterResults struct {
	Hate      Profanity `json:"hate,omitempty"`
	SelfHarm  Profanity `json:"self_harm,omitempty"`
	Sexual    Profanity `json:"sexual,omitempty"`
	Violence  Profanity `json:"violence,omitempty"`
	JailBreak Profanity `json:"jailbreak,omitempty"`
	Profanity Profanity `json:"profanity,omitempty"`
}

type Profanity struct {
	Filtered bool `json:"filtered"`
	Detected bool `json:"detected"`
}

// common.go defines common types used throughout the OpenAI API.

// Usage Represents the total token usage per request to OpenAI.
type Usage struct {
	PromptTokens            int                      `json:"prompt_tokens"`
	CompletionTokens        int                      `json:"completion_tokens"`
	TotalTokens             int                      `json:"total_tokens"`
	PromptTokensDetails     *PromptTokensDetails     `json:"prompt_tokens_details"`
	CompletionTokensDetails *CompletionTokensDetails `json:"completion_tokens_details"`
}

// CompletionTokensDetails Breakdown of tokens used in a completion.
type CompletionTokensDetails struct {
	AudioTokens              int `json:"audio_tokens"`
	ReasoningTokens          int `json:"reasoning_tokens"`
	AcceptedPredictionTokens int `json:"accepted_prediction_tokens"`
	RejectedPredictionTokens int `json:"rejected_prediction_tokens"`
}

// PromptTokensDetails Breakdown of tokens used in the prompt.
type PromptTokensDetails struct {
	AudioTokens  int `json:"audio_tokens"`
	CachedTokens int `json:"cached_tokens"`
}

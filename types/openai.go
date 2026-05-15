package types

import (
	"context"
	"encoding/json"
)

// Copy for github.com/sashabaranov/go-openai
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
	Model    string                  `json:"model"`
	Messages []ChatCompletionMessage `json:"messages"`
	// MaxTokens The maximum number of tokens that can be generated in the chat completion.
	// This value can be used to control costs for text generated via API.
	// Deprecated: use MaxCompletionTokens. Not compatible with o1-series models.
	// refs: https://platform.openai.com/docs/api-reference/chat/create#chat-create-max_tokens
	MaxTokens int `json:"max_tokens,omitempty"`
	// MaxCompletionTokens An upper bound for the number of tokens that can be generated for a completion,
	// including visible output tokens and reasoning tokens https://platform.openai.com/docs/guides/reasoning
	MaxCompletionTokens int                           `json:"max_completion_tokens,omitempty"`
	Temperature         float32                       `json:"temperature,omitempty"`
	TopP                float32                       `json:"top_p,omitempty"`
	N                   int                           `json:"n,omitempty"`
	Stream              bool                          `json:"stream,omitempty"`
	Stop                []string                      `json:"stop,omitempty"`
	PresencePenalty     float32                       `json:"presence_penalty,omitempty"`
	ResponseFormat      *ChatCompletionResponseFormat `json:"response_format,omitempty"`
	Seed                *int                          `json:"seed,omitempty"`
	FrequencyPenalty    float32                       `json:"frequency_penalty,omitempty"`
	// LogitBias is must be a token id string (specified by their token ID in the tokenizer), not a word string.
	// incorrect: `"logit_bias":{"You": 6}`, correct: `"logit_bias":{"1639": 6}`
	// refs: https://platform.openai.com/docs/api-reference/chat/create#chat/create-logit_bias
	LogitBias map[string]int `json:"logit_bias,omitempty"`
	// LogProbs indicates whether to return log probabilities of the output tokens or not.
	// If true, returns the log probabilities of each output token returned in the content of message.
	// This option is currently not available on the gpt-4-vision-preview model.
	LogProbs bool `json:"logprobs,omitempty"`
	// TopLogProbs is an integer between 0 and 5 specifying the number of most likely tokens to return at each
	// token position, each with an associated log probability.
	// logprobs must be set to true if this parameter is used.
	TopLogProbs int    `json:"top_logprobs,omitempty"`
	User        string `json:"user,omitempty"`
	// Deprecated: use Tools instead.
	Functions []FunctionDefinition `json:"functions,omitempty"`
	// Deprecated: use ToolChoice instead.
	FunctionCall any    `json:"function_call,omitempty"`
	Tools        []Tool `json:"tools,omitempty"`
	// This can be either a string or an ToolChoice object.
	ToolChoice any `json:"tool_choice,omitempty"`
	// Options for streaming response. Only set this when you set stream: true.
	StreamOptions *StreamOptions `json:"stream_options,omitempty"`
	// Disable the default behavior of parallel tool calls by setting it: false.
	ParallelToolCalls any `json:"parallel_tool_calls,omitempty"`
	// Store can be set to true to store the output of this completion request for use in distillations and evals.
	// https://platform.openai.com/docs/api-reference/chat/create#chat-create-store
	Store bool `json:"store,omitempty"`
	// Controls effort on reasoning for reasoning models. It can be set to "low", "medium", or "high".
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	// Metadata to store with the completion.
	Metadata map[string]string `json:"metadata,omitempty"`
	// Configuration for a predicted output.
	Prediction *Prediction `json:"prediction,omitempty"`
	// ChatTemplateKwargs provides a way to add non-standard parameters to the request body.
	// Additional kwargs to pass to the template renderer. Will be accessible by the chat template.
	// Such as think mode for qwen3. "chat_template_kwargs": {"enable_thinking": false}
	// https://qwen.readthedocs.io/en/latest/deployment/vllm.html#thinking-non-thinking-modes
	ChatTemplateKwargs map[string]any `json:"chat_template_kwargs,omitempty"`
	// Specifies the latency tier to use for processing the request.
	ServiceTier string `json:"service_tier,omitempty"`
	// Verbosity determines how many output tokens are generated. Lowering the number of
	// tokens reduces overall latency. It can be set to "low", "medium", or "high".
	// Note: This field is only confirmed to work with gpt-5, gpt-5-mini and gpt-5-nano.
	// Also, it is not in the API reference of chat completion at the time of writing,
	// though it is supported by the API.
	Verbosity string `json:"verbosity,omitempty"`
	// A stable identifier used to help detect users of your application that may be violating OpenAI's usage policies.
	// The IDs should be a string that uniquely identifies each user.
	// We recommend hashing their username or email address, in order to avoid sending us any identifying information.
	// https://platform.openai.com/docs/api-reference/chat/create#chat_create-safety_identifier
	SafetyIdentifier string `json:"safety_identifier,omitempty"`
	// Embedded struct for non-OpenAI extensions
	ChatCompletionRequestExtensions
}

type ChatCompletionMessage struct {
	Role         string `json:"role"`
	Content      string `json:"content,omitempty"`
	Refusal      string `json:"refusal,omitempty"`
	MultiContent []ChatMessagePart

	// This property isn't in the official documentation, but it's in
	// the documentation for the official library for python:
	// - https://github.com/openai/openai-python/blob/main/chatml.md
	// - https://github.com/openai/openai-cookbook/blob/main/examples/How_to_count_tokens_with_tiktoken.ipynb
	Name string `json:"name,omitempty"`

	// This property is used for the "reasoning" feature supported by deepseek-reasoner
	// which is not in the official documentation.
	// the doc from deepseek:
	// - https://api-docs.deepseek.com/api/create-chat-completion#responses
	ReasoningContent string `json:"reasoning_content,omitempty"`

	FunctionCall *FunctionCall `json:"function_call,omitempty"`

	// For Role=assistant prompts this may be set to the tool calls generated by the model, such as function calls.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// For Role=tool prompts this should be set to the ID given in the assistant's prior request to call a tool.
	ToolCallID string `json:"tool_call_id,omitempty"`
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
	// If set, an additional chunk will be streamed before the data: [DONE] message.
	// The usage field on this chunk shows the token usage statistics for the entire request,
	// and the choices field will always be an empty array.
	// All other chunks will also include a usage field, but with a null value.
	IncludeUsage bool `json:"include_usage,omitempty"`
}

type Prediction struct {
	Content string `json:"content"`
	Type    string `json:"type"`
}

// ChatCompletionRequestExtensions contains third-party OpenAI API extensions
// (e.g., vendor-specific implementations like vLLM).
type ChatCompletionRequestExtensions struct {
	// GuidedChoice is a vLLM-specific extension that restricts the model's output
	// to one of the predefined string choices provided in this field. This feature
	// is used to constrain the model's responses to a controlled set of options,
	// ensuring predictable and consistent outputs in scenarios where specific
	// choices are required.
	GuidedChoice []string `json:"guided_choice,omitempty"`
}

type FunctionCall struct {
	Name string `json:"name,omitempty"`
	// call function with arguments in JSON format
	Arguments string `json:"arguments,omitempty"`
}

type ToolCall struct {
	// Index is not nil only in chat completion chunk object
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
	Index   int                   `json:"index"`
	Message ChatCompletionMessage `json:"message"`
	// FinishReason
	// stop: API returned complete message,
	// or a message terminated by one of the stop sequences provided via the stop parameter
	// length: Incomplete model output due to max_tokens parameter or token limit
	// function_call: The model decided to call a function
	// content_filter: Omitted content due to a flag from our content filters
	// null: API response still in progress or incomplete
	FinishReason string `json:"finish_reason"`
	LogProbs     *struct {
		// Content is a list of message content tokens with log probability information.
		Content []struct {
			Token   string  `json:"token"`
			LogProb float64 `json:"logprob"`
			Bytes   []byte  `json:"bytes,omitempty"` // Omitting the field if it is null
			// TopLogProbs is a list of the most likely tokens and their log probability, at this token position.
			// In rare cases, there may be fewer than the number of requested top_logprobs returned.
			TopLogProbs []struct {
				Token   string  `json:"token"`
				LogProb float64 `json:"logprob"`
				Bytes   []byte  `json:"bytes,omitempty"`
			} `json:"top_logprobs"`
		} `json:"content"`
	} `json:"logprobs,omitempty"`
	ContentFilterResults ContentFilterResults `json:"content_filter_results,omitempty"`
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

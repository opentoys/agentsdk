package aichat

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/opentoys/agentsdk/modules/stdlib/jsonx"
	"github.com/opentoys/agentsdk/types"
)

const chaturl = "/chat/completions"

type openAI struct {
	apikey string
	base   string
	model  string
}

type Option func(*openAI)

func WithOpenAIKey(apikey string) Option {
	return func(oa *openAI) {
		oa.apikey = apikey
	}
}

func WithOpenAIBase(base string) Option {
	return func(oa *openAI) {
		oa.base = base
	}
}

func WithOpenAIModel(model string) Option {
	return func(oa *openAI) {
		oa.model = model
	}
}

func NewOpenAI(opts ...Option) *openAI {
	sdk := &openAI{}
	for _, f := range opts {
		f(sdk)
	}
	return sdk
}

func (s *openAI) CreateChatCompletion(ctx context.Context, in types.ChatCompletionRequest) (out types.ChatCompletionResponse, e error) {
	var url = s.base + chaturl
	in.Model = s.model
	buf, e := jsonx.Marshal(in)
	if e != nil {
		return
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if e != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+s.apikey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "go-agentsdk/1.0.0")

	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		return
	}
	defer resp.Body.Close()
	if buf, e = io.ReadAll(resp.Body); e != nil {
		return
	}

	if e = jsonx.Unmarshal(buf, &out); e != nil {
		return
	}
	if out.ID == "" {
		e = errors.New(string(buf))
	}
	return
}

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

const anthropicurl = "/chat/completions"

type anthropic struct {
	*config
}

func NewAnthropic(opts ...Option) *anthropic {
	cfg := &config{}
	for _, f := range opts {
		f(cfg)
	}
	return &anthropic{config: cfg}
}

func (s *anthropic) CreateChatCompletion(ctx context.Context, in types.ChatCompletionRequest) (out types.ChatCompletionResponse, e error) {
	var url = s.base + anthropicurl
	in.Model = s.model
	buf, e := jsonx.Marshal(in)
	if e != nil {
		return
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if e != nil {
		return
	}
	req.Header.Set("Authorization", s.apikey)
	req.Header.Set("x-api-key", s.apikey)
	req.Header.Set("anthropic-version", "2023-06-01")
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

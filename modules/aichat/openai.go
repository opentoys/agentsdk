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
const base = "https://api.deepseek.com/v1"
const mdoel = "deepseek-v4-flash"

type config struct {
	apikey string
	base   string
	model  string
	uri    string
}

type Option func(*config)

func WithKey(apikey string) Option {
	return func(oa *config) {
		oa.apikey = apikey
	}
}

func WithBase(base string) Option {
	return func(oa *config) {
		oa.base = base
	}
}

func WithModel(model string) Option {
	return func(oa *config) {
		oa.model = model
	}
}

type openai struct {
	*config
}

func New(opts ...Option) *openai {
	cfg := &config{}
	for _, f := range opts {
		f(cfg)
	}
	if cfg.base == "" {
		cfg.base = base
	}
	if cfg.model == "" {
		cfg.model = mdoel
	}
	return &openai{config: cfg}
}

func (s *openai) CreateChatCompletion(ctx context.Context, in types.ChatCompletionRequest) (out types.ChatCompletionResponse, e error) {
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

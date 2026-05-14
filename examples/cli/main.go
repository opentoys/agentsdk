package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/opentoys/agentsdk"
	"github.com/opentoys/agentsdk/memory"
	"github.com/opentoys/agentsdk/tool"
	"github.com/opentoys/agentsdk/vfs"
	"github.com/sashabaranov/go-openai"
)

func main() {
	mem := vfs.NewMem()
	mem.WriteFile("xxx/SKILL.md", []byte("hello"))
	rcfg := &agentsdk.Config{
		SkillsDir: mem,
		APIKey:    os.Getenv("OPENAI_API_KEY"),
		APIBase:   os.Getenv("OPENAI_API_BASE"),
		Model:     os.Getenv("OPENAI_API_MODE"),
		Debug:     true,
		BaseTools: map[string]*tool.Tool{
			"http": tool.DefineHttpRequest(),
			"read": tool.DefineReadLocal(mem),
		},
	}

	var start = time.Now()
	fmt.Println("系统开始", start)

	agent, e := agentsdk.New(rcfg)
	if e != nil {
		panic(e)
	}

	var messages []openai.ChatCompletionMessage
	buf, _ := os.ReadFile("xxx.json")
	json.Unmarshal(buf, &messages)
	var prev = len(messages)

	agent = agent.NewChat(messages)
	resp, e := agent.Run(context.Background(), os.Getenv("INPUT"))
	if e != nil {
		panic(e)
	}

	buf, _ = json.Marshal(agent.Messages()[prev:])
	if len(buf) > 0 {
		summar, _ := memory.BuildSummarize(context.Background(), agent.Chat, string(buf), memory.SummarizeOptions{})
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: summar,
		})
	}

	buf, _ = json.Marshal(messages)
	os.WriteFile("xxx.json", buf, 0o644)

	fmt.Println("系统结束", time.Since(start))
	fmt.Println(resp)

}

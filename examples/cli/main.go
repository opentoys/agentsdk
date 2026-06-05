package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/opentoys/agentsdk"
	"github.com/opentoys/agentsdk/modules/aichat"
	"github.com/opentoys/agentsdk/modules/log"
	"github.com/opentoys/agentsdk/tool"
	"github.com/opentoys/agentsdk/types"
)

func main() {
	fs := os.DirFS(os.Getenv("SKILL_DIR"))
	rcfg := types.Config{
		SkillsFS: fs,
		Debug:    &log.DefaultLog{},
		ChatClient: aichat.New(
			aichat.WithKey(os.Getenv("OPENAI_API_KEY")),
			aichat.WithBase(os.Getenv("OPENAI_API_BASE")),
			aichat.WithModel(os.Getenv("OPENAI_API_MODE")),
		),
		Tools: []types.Tool{
			tool.DefineHTTPTool(),
			tool.DefineReadLocal(fs),
		},
	}

	var start = time.Now()
	fmt.Println("系统开始", start)

	agent := agentsdk.New(rcfg)

	resp, e := agent.Run(context.Background(), os.Getenv("INPUT"))
	if e != nil {
		panic(e)
	}
	fmt.Println(resp)
}

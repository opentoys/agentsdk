package main

import (
	"context"
	"fmt"
	"os"

	"github.com/opentoys/agentsdk"
)

func main() {
	rcfg := &agentsdk.Config{
		SkillsDir:  os.Getenv("SKILL_DIRS"),
		MCPServers: nil,
		APIKey:     os.Getenv("OPENAI_API_KEY"),
		APIBase:    os.Getenv("OPENAI_API_BASE"),
		Model:      os.Getenv("OPENAI_API_MODE"),
		Debug:      true,
	}

	agent, e := agentsdk.New(rcfg)
	if e != nil {
		panic(e)
	}

	resp, e := agent.Run(context.Background(), os.Getenv("INPUT"))
	if e != nil {
		panic(e)
	}
	fmt.Println(resp)

	agent.NewChat(nil).Run(context.Background(), os.Getenv("INPUT"))
}

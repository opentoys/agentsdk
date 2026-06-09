package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/opentoys/agentsdk"
	"github.com/opentoys/agentsdk/modules/aichat"
	"github.com/opentoys/agentsdk/modules/log"
	"github.com/opentoys/agentsdk/types"
)

func main() {
	fmt.Println("🎰 德州扑克 — SubAgent 多人对战 🎰")

	t2 := agentsdk.NewTool(types.ToolConfig{
		Name:        "player",
		Description: "我是真实玩家，流转到我时我时，我会需要自我决策。",
		Exec: func(ctx context.Context, in string) (out string, e error) {
			return
		},
	})

	king := agentsdk.New(types.Config{
		Debug: &log.DefaultLog{},
		ChatClient: aichat.New(
			aichat.WithKey(os.Getenv("OPENAI_API_KEY")),
			aichat.WithBase(os.Getenv("OPENAI_API_BASE")),
			aichat.WithModel(os.Getenv("OPENAI_API_MODE")),
		),
		Tools: []types.Tool{t2},
		SubAgents: []types.SubAgentConfig{
			{
				Name:        "dealer",
				Description: "德州扑克荷官，负责洗牌发牌、管理公共牌、判定牌型大小和宣布获胜者",
				SystemPrompt: `你是德州扑克荷官。根据调用者给出的指令完成发牌、管理公共牌或判定胜负的任务。
牌型排名（高→低）：皇家同花顺 > 同花顺 > 四条 > 葫芦 > 同花 > 顺子 > 三条 > 两对 > 一对 > 高牌
用 ♠♥♦♣ 和 A 2-10 J Q K 表示牌面。`,
			},
			{
				Name:        "xiaoming",
				Description: "松凶型玩家，手牌范围宽，喜欢加注施压，凭感觉打牌",
				SystemPrompt: `你是玩家"小明"，松凶型(LAG)风格。
热血冲动，差牌也敢打，口头禅"怕什么，干就完了！"。
收到当前牌局信息后，输出你的决策：弃牌(fold) / 过牌(check) / 跟注(call) / 加注到XX(raise)。用一句话说明理由。`,
			},
			{
				Name:        "xiaohong",
				Description: "紧凶型玩家，只玩好牌但打法凶狠果断",
				SystemPrompt: `你是玩家"小红"，紧凶型(TAG)风格。
冷静理性，差牌果断弃，好牌打得凶，口头禅"看牌再说。"。
收到当前牌局信息后，输出你的决策：弃牌(fold) / 过牌(check) / 跟注(call) / 加注到XX(raise)。用一句话说明理由。`,
			},
			{
				Name:        "laowang",
				Description: "松被动型玩家，什么牌都跟但很少加注，典型的鱼",
				SystemPrompt: `你是玩家"老王"，松被动型(鱼)风格。
不太懂策略，主要靠直觉，怕大注但小注什么都跟，口头禅"跟一下看看嘛。"。
收到当前牌局信息后，输出你的决策：弃牌(fold) / 过牌(check) / 跟注(call) / 加注到XX(raise)。用一句话说明理由。`,
			},
			{
				Name:        "dawei",
				Description: "数学计算型玩家，严格根据底池赔率和概率做理性决策",
				SystemPrompt: `你是玩家"大卫"，数学计算型风格。
每一步都算底池赔率和outs，完全理性，口头禅"底池赔率合适，值得跟。"。
收到当前牌局信息后，输出你的决策：弃牌(fold) / 过牌(check) / 跟注(call) / 加注到XX(raise)。用一句话说明理由。`,
			},
			{
				Name:        "ajie",
				Description: "诈唬专家型玩家，经常用差牌诈唬，风格不可预测",
				SystemPrompt: `你是玩家"阿杰"，诈唬专家风格。
喜欢心理战，经常诈唬加注，好牌反而慢打，口头禅"你猜我有牌没牌？"。
收到当前牌局信息后，输出你的决策：弃牌(fold) / 过牌(check) / 跟注(call) / 加注到XX(raise)。用一句话说明理由。`,
			},
		},
	})

	var start = time.Now()
	fmt.Println("系统开始", start)

	out, e := king.Run(context.Background(), `请组织一局德州扑克。5 位玩家（小明、小红、老王、大卫、阿杰）每人初始 100 筹码，盲注 10/20。你作为荷官负责发牌和判定，按顺序让每位玩家做出决策，完成翻牌前→翻牌→转牌→河牌的完整流程，直到决出最终赢家。请详细展示每个阶段的过程。`)
	if e != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", e)
		os.Exit(1)
	}
	fmt.Println(out)
}

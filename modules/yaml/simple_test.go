package yaml

import (
	"fmt"
	"testing"
)

const yamltext = `name: security
description: Advanced security validation for Clawdbot - pattern detection, command sanitization, and threat monitoring
homepage: https://github.com/gtrusler/clawdbot-security-suite
metadata:
  clawdbot:
    emoji: "🔒"
    requires:
      bins: ["jq"]`

type SkillMeta struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	AllowedTools []string `json:"allowed-tools"`
	Model        string   `json:"model,omitempty"`
	Author       string   `json:"author,omitempty"`
	Version      string   `json:"version,omitempty"`
	License      string   `json:"license,omitempty"`
	Tools        []struct {
		Name        string `json:"name"`
		Script      string `json:"script,omitempty"` // 可选，指定脚本路径
		Description string `json:"description,omitempty"`
		Parameters  map[string]struct {
			Type        string `json:"type"`
			Description string `json:"description,omitempty"`
			Required    bool   `json:"required,omitempty"`
		} `json:"parameters,omitempty"`
	} `json:"tools,omitempty"` // 工具定义列表
}

func TestXxx(t *testing.T) {
	var mate SkillMeta
	e := Unmarshal([]byte(yamltext), &mate)
	fmt.Println(e, mate)
}

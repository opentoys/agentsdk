package tool_test

import (
	"fmt"
	"testing"

	"github.com/opentoys/agentsdk/tool"
)

func TestHttp(t *testing.T) {
	fmt.Println(tool.HttpRequest("curl -X POST -d '{}' https://qyapi.weixin.qq.com/cgi-bin/message/send"))
}

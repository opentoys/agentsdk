package tool_test

import (
	"fmt"
	"testing"

	"git.myscrm.cn/xiaqb01/agentsdk/tool"
)

func TestHttp(t *testing.T) {
	fmt.Println(tool.HttpRequest("curl -X POST -d '{}' https://qyapi.weixin.qq.com/cgi-bin/message/send"))
}

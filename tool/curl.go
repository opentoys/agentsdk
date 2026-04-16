package tool

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// CurlResult curl 命令解析结果
type CurlResult struct {
	Method  string            // GET POST PUT DELETE 等
	URL     string            // 请求 URL
	Headers map[string]string // 请求头 key:value
	Body    string            // 请求体
	Timeout int               // 超时秒数
	RawArgs []string          // 原始参数列表
}

// 支持解析 curl 命令行参数
func CurlParse(cmd string) *CurlResult {
	result := &CurlResult{
		Method:  "GET",
		Headers: make(map[string]string),
	}
	if cmd == "" {
		return result
	}

	// 按空格分割，但保留引号内的内容
	args := splitCommandLine(cmd)
	result.RawArgs = args

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "curl":
			continue
		case "-X", "--request", "--method":
			if i+1 < len(args) {
				i++
				result.Method = strings.ToUpper(args[i])
			}
		case "-H", "--header":
			if i+1 < len(args) {
				i++
				parts := strings.SplitN(args[i], ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					val := strings.TrimSpace(parts[1])
					result.Headers[key] = val
				}
			}
		case "-d", "--data", "--data-raw", "--data-binary":
			if i+1 < len(args) {
				i++
				result.Body = unquote(args[i])
			}
		case "--connect-timeout", "-m", "--max-time":
			if i+1 < len(args) {
				i++
				var t int
				fmt.Sscanf(args[i], "%d", &t)
				result.Timeout = t
			}
		default:
			// 不是选项参数，视为 URL
			if !strings.HasPrefix(arg, "-") && result.URL == "" {
				result.URL = arg
			} else if arg == "-" || strings.HasPrefix(arg, "@") {
				// stdin 或文件输入，跳过
				continue
			}
		}
	}

	// 如果有 body 但没有指定方法，默认为 POST
	if result.Body != "" && result.Method == "GET" {
		result.Method = "POST"
	}

	return result
}

// String 格式化输出
func (r *CurlResult) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Method: %s\n", r.Method))
	sb.WriteString(fmt.Sprintf("URL: %s\n", r.URL))
	if len(r.Headers) > 0 {
		sb.WriteString("Headers:\n")
		for k, v := range r.Headers {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}
	if r.Body != "" {
		body := r.Body
		if len(body) > 200 {
			body = body[:200] + "..."
		}
		sb.WriteString(fmt.Sprintf("Body: %s\n", body))
	}
	if r.Timeout > 0 {
		sb.WriteString(fmt.Sprintf("Timeout: %ds\n", r.Timeout))
	}
	return sb.String()
}

// splitCommandLine 按空格分割命令行，保留引号内的内容
func splitCommandLine(cmd string) []string {
	var args []string
	var buf strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	r := strings.NewReader(cmd)

	for {
		ch, _, err := r.ReadRune()
		if err != nil {
			break
		}
		switch ch {
		case '\'':
			if !inDoubleQuote {
				inSingleQuote = !inSingleQuote
			} else {
				buf.WriteRune(ch)
			}
		case '"':
			if !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
			} else {
				buf.WriteRune(ch)
			}
		case '\\':
			if inDoubleQuote || inSingleQuote {
				// 在引号内处理转义
				nextCh, _, readErr := r.ReadRune()
				if readErr != nil {
					buf.WriteRune(ch)
					break
				}
				buf.WriteRune(nextCh)
			} else {
				// 引号外反斜杠转义下一个字符
				nextCh, _, readErr := r.ReadRune()
				if readErr != nil {
					buf.WriteRune(ch)
					break
				}
				buf.WriteRune(nextCh)
			}
		default:
			if unicode.IsSpace(ch) && !inSingleQuote && !inDoubleQuote {
				if buf.Len() > 0 {
					args = append(args, buf.String())
					buf.Reset()
				}
			} else {
				buf.WriteRune(ch)
			}
		}
	}
	if buf.Len() > 0 {
		args = append(args, buf.String())
	}
	return args
}

// unquote 移除首尾的引号
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			unquoted, err := strconv.Unquote(s)
			if err == nil {
				return unquoted
			}
			return s[1 : len(s)-1]
		}
	}
	return s
}

package helper

import (
	"bufio"
	"encoding/json"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// JSONStreamScanner 处理流式 JSON 响应，确保只在完整的 JSON 对象时返回
// 用于解决 SSE 流中 JSON 被分割导致的解析错误问题
type JSONStreamScanner struct {
	scanner    *bufio.Scanner
	buffer     strings.Builder
	braceDepth int
	lastData   string
}

// NewJSONStreamScanner 创建一个新的 JSON 流式扫描器
func NewJSONStreamScanner(scanner *bufio.Scanner) *JSONStreamScanner {
	return &JSONStreamScanner{
		scanner:    scanner,
		braceDepth: 0,
	}
}

// Scan 读取下一个完整的 JSON 对象
// 返回 true 表示成功读取到一个完整的 JSON，false 表示流结束或遇到 [DONE]
func (j *JSONStreamScanner) Scan() bool {
	for j.scanner.Scan() {
		line := j.scanner.Text()

		// 跳过空行
		trimmed := strings.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}

		// 处理 SSE 格式: "data: {...}"
		if strings.HasPrefix(line, "data:") {
			line = strings.TrimPrefix(line, "data:")
			line = strings.TrimLeft(line, " ")
		}

		line = strings.TrimSuffix(line, "\r")
		line = strings.TrimSpace(line)

		if len(line) == 0 {
			continue
		}

		// 检查是否是 [DONE] 标记
		if strings.HasPrefix(line, "[DONE]") {
			if common.DebugEnabled {
				println("JSON stream scanner: received [DONE]")
			}
			return false
		}

		// 如果 buffer 为空，先尝试直接验证当前行是否是完整的 JSON（快速路径）
		if j.buffer.Len() == 0 {
			if json.Valid([]byte(line)) {
				j.lastData = line
				if common.DebugEnabled {
					println("===========================================")
					println("JSON stream scanner: single-line valid JSON")
					println("Length:", len(line))
					println("Content preview:", line[:min(200, len(line))])
					println("===========================================")
				}
				return true
			}

			// 如果不是完整的 JSON，检查是否以 { 或 [ 开头
			// 只有以 { 或 [ 开头的才可能是被分割的 JSON，才需要累积
			if !strings.HasPrefix(line, "{") && !strings.HasPrefix(line, "[") {
				if common.DebugEnabled {
					println("JSON stream scanner: skipping invalid line (not starting with { or [):", line[:min(50, len(line))])
				}
				continue
			}
		}

		// 累积数据到缓冲区（处理跨行的 JSON）
		j.buffer.WriteString(line)

		// 计算花括号/方括号深度（需要考虑字符串内的括号）
		newDepth := calculateJSONDepth(j.buffer.String())

		if common.DebugEnabled && j.buffer.Len() > 0 {
			println("JSON scanner: accumulated", j.buffer.Len(), "bytes, depth:", newDepth)
		}

		// 当括号平衡时（depth == 0 且以 { 或 [ 开头），说明可能是一个完整的 JSON
		bufferStr := j.buffer.String()
		startsWithBrace := strings.HasPrefix(bufferStr, "{") || strings.HasPrefix(bufferStr, "[")

		if newDepth == 0 && startsWithBrace && j.buffer.Len() > 0 {
			jsonStr := j.buffer.String()

			// 验证是否是有效的 JSON
			if json.Valid([]byte(jsonStr)) {
				j.lastData = jsonStr
				j.buffer.Reset()
				j.braceDepth = 0
				if common.DebugEnabled {
					println("===========================================")
					println("JSON stream scanner: parsed complete JSON (accumulated)")
					println("Length:", len(jsonStr))
					println("Content preview:", jsonStr[:min(200, len(jsonStr))])
					println("===========================================")
				}
				return true
			} else {
				// JSON 无效但括号平衡，可能是格式错误，重置 buffer
				if common.DebugEnabled {
					println("JSON stream scanner: invalid JSON with balanced braces, resetting buffer")
					println("Buffer content:", j.buffer.String()[:min(100, j.buffer.Len())])
				}
				j.buffer.Reset()
				j.braceDepth = 0
			}
		} else {
			// 更新深度
			j.braceDepth = newDepth
		}

		// 防止缓冲区过大（可能是格式错误）
		if j.buffer.Len() > 10*1024*1024 { // 10MB
			if common.DebugEnabled {
				println("JSON stream scanner: buffer too large, resetting")
			}
			j.buffer.Reset()
			j.braceDepth = 0
		}
	}

	// 扫描结束，检查是否有剩余的有效 JSON
	if j.buffer.Len() > 0 {
		jsonStr := j.buffer.String()
		if json.Valid([]byte(jsonStr)) {
			j.lastData = jsonStr
			j.buffer.Reset()
			j.braceDepth = 0
			return true
		} else if common.DebugEnabled {
			println("JSON stream scanner: end of stream with invalid buffered data:", jsonStr[:min(100, len(jsonStr))])
		}
	}

	return false
}

// calculateJSONDepth 计算 JSON 字符串的括号深度，正确处理字符串内的括号
// 返回当前的括号嵌套深度（0 表示所有括号都已配对）
func calculateJSONDepth(jsonStr string) int {
	depth := 0
	inString := false
	escaped := false

	for i := 0; i < len(jsonStr); i++ {
		ch := jsonStr[i]

		if escaped {
			// 转义字符后的任何字符都跳过
			escaped = false
			continue
		}

		if inString {
			// 在字符串内，只关心引号和转义符
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
		} else {
			// 在字符串外，处理结构字符
			switch ch {
			case '"':
				inString = true
			case '{', '[':
				depth++
			case '}', ']':
				depth--
				// 防止 depth 变成负数（格式错误的 JSON）
				if depth < 0 {
					if common.DebugEnabled {
						println("JSON depth calculation: unmatched closing bracket at position", i)
					}
					return -1
				}
			}
		}
	}

	// 如果最后还在字符串内，说明字符串未闭合
	if inString && common.DebugEnabled {
		println("JSON depth calculation: unclosed string")
	}

	return depth
}

// Text 返回完整的 JSON 字符串
func (j *JSONStreamScanner) Text() string {
	return j.lastData
}

// Err 返回扫描过程中的错误
func (j *JSONStreamScanner) Err() error {
	return j.scanner.Err()
}

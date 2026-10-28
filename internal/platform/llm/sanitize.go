package llm

import (
	"regexp"
	"strings"
)

var (
	codeBlockRegex     = regexp.MustCompile("(?s)```(?:json)?\\s*\n?(.*?)\\s*```")
	trailingCommaRegex = regexp.MustCompile(`,(\s*[}\]])`)
)

func sanitizeJSONResponse(raw string) string {
	trimmed := strings.TrimSpace(raw)
	// Remove UTF-8 BOM if present
	trimmed = strings.TrimPrefix(trimmed, "\xef\xbb\xbf")

	if matches := codeBlockRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
		trimmed = strings.TrimSpace(matches[1])
	} else {
		if strings.HasPrefix(trimmed, "```json") {
			trimmed = strings.TrimPrefix(trimmed, "```json")
		} else if strings.HasPrefix(trimmed, "```") {
			trimmed = strings.TrimPrefix(trimmed, "```")
		}
		if strings.HasSuffix(trimmed, "```") {
			trimmed = strings.TrimSuffix(trimmed, "```")
		}
		trimmed = strings.TrimSpace(trimmed)
	}

	// Find the start of the JSON object or array
	startObj := strings.Index(trimmed, "{")
	startArr := strings.Index(trimmed, "[")
	start := -1
	if startObj != -1 && (startArr == -1 || startObj < startArr) {
		start = startObj
	} else if startArr != -1 {
		start = startArr
	}

	if start == -1 {
		return trimmed
	}
	trimmed = trimmed[start:]

	// Scan through JSON tracking depth, strings, escaped characters, and escape literal newlines in strings
	var stack []byte
	var out strings.Builder
	out.Grow(len(trimmed) + 32)
	inString := false
	escaped := false
	rootClosed := false

	for i := 0; i < len(trimmed); i++ {
		ch := trimmed[i]

		if rootClosed {
			break
		}

		if escaped {
			escaped = false
			out.WriteByte(ch)
			continue
		}

		if ch == '\\' {
			if inString {
				escaped = true
			}
			out.WriteByte(ch)
			continue
		}

		if ch == '"' {
			inString = !inString
			out.WriteByte(ch)
			continue
		}

		if inString {
			// Escape unescaped control characters inside JSON strings
			if ch == '\n' {
				out.WriteString(`\n`)
			} else if ch == '\r' {
				out.WriteString(`\r`)
			} else if ch == '\t' {
				out.WriteString(`\t`)
			} else {
				out.WriteByte(ch)
			}
			continue
		}

		out.WriteByte(ch)
		if ch == '{' || ch == '[' {
			stack = append(stack, ch)
		} else if ch == '}' || ch == ']' {
			if len(stack) > 0 {
				top := stack[len(stack)-1]
				if (top == '{' && ch == '}') || (top == '[' && ch == ']') {
					stack = stack[:len(stack)-1]
					if len(stack) == 0 {
						rootClosed = true
					}
				}
			}
		}
	}

	// Auto-repair if truncated before root was closed
	if !rootClosed {
		if inString {
			out.WriteByte('"')
		}
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i] == '{' {
				out.WriteByte('}')
			} else if stack[i] == '[' {
				out.WriteByte(']')
			}
		}
	}

	res := out.String()
	// Remove trailing commas before closing curly braces or brackets
	res = trailingCommaRegex.ReplaceAllString(res, "$1")

	return res
}

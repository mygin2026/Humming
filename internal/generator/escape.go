package generator

import (
	"strconv"
	"strings"
)

// EscapeSwiftString 将任意文本安全转义为 Swift 双引号字符串字面量内容。
func EscapeSwiftString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 8)
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\u0000':
			b.WriteString(`\0`)
		default:
			if r < 0x20 || r == 0x7f {
				b.WriteString(`\u{`)
				b.WriteString(strconv.FormatInt(int64(r), 16))
				b.WriteString(`}`)
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// QuoteSwift 返回带双引号的 Swift 字符串字面量。
func QuoteSwift(s string) string {
	return `"` + EscapeSwiftString(s) + `"`
}

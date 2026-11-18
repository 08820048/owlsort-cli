package tomledit

import (
	"strconv"
	"strings"
)

// The states a scanner can be in: outside any string, or inside one of the four
// kinds TOML allows.
const (
	scanNone = iota
	scanBasic
	scanLiteral
	scanMultiBasic
	scanMultiLiteral
)

// scanner follows the file left to right, remembering whether the next line
// starts at the file's own key level: not inside a string, not inside an array.
// Without it a "[" in a path or in a name would look like a table header.
type scanner struct {
	depth int
	str   int
}

// atKeyLevel reports whether a key may start here.
func (s scanner) atKeyLevel() bool { return s.depth == 0 && s.str == scanNone }

// advance walks one line and returns the state at the start of the next.
func (s scanner) advance(text string) scanner {
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch s.str {
		case scanBasic:
			if c == '\\' {
				i++
				continue
			}
			if c == '"' {
				s.str = scanNone
			}
			continue
		case scanLiteral:
			if c == '\'' {
				s.str = scanNone
			}
			continue
		case scanMultiBasic:
			if c == '\\' {
				i++
				continue
			}
			if strings.HasPrefix(text[i:], `"""`) {
				s.str = scanNone
				i += 2
			}
			continue
		case scanMultiLiteral:
			if strings.HasPrefix(text[i:], `'''`) {
				s.str = scanNone
				i += 2
			}
			continue
		}
		switch c {
		case '#':
			return s // a comment runs to the end of the line
		case '"':
			if strings.HasPrefix(text[i:], `"""`) {
				s.str = scanMultiBasic
				i += 2
				continue
			}
			s.str = scanBasic
		case '\'':
			if strings.HasPrefix(text[i:], `'''`) {
				s.str = scanMultiLiteral
				i += 2
				continue
			}
			s.str = scanLiteral
		case '[':
			s.depth++
		case ']':
			if s.depth > 0 {
				s.depth--
			}
		}
	}
	return s
}

// isHeader reports whether a line opens a table.
func isHeader(text string) bool {
	_, ok := headerOf(text)
	return ok
}

// headerOf reads a table header. kind is the name between the brackets, so
// `[[rule]]` gives "rule".
func headerOf(text string) (kind string, ok bool) {
	trimmed := strings.TrimSpace(stripComment(text))
	if !strings.HasPrefix(trimmed, "[") {
		return "", false
	}
	if strings.HasPrefix(trimmed, "[[") && strings.HasSuffix(trimmed, "]]") {
		return strings.TrimSpace(trimmed[2 : len(trimmed)-2]), true
	}
	if !strings.HasPrefix(trimmed, "[[") && strings.HasSuffix(trimmed, "]") {
		return strings.TrimSpace(trimmed[1 : len(trimmed)-1]), true
	}
	return "", false
}

// stripComment removes a trailing comment, respecting strings.
func stripComment(text string) string {
	var st scanner
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch st.str {
		case scanBasic:
			if c == '\\' {
				i++
				continue
			}
			if c == '"' {
				st.str = scanNone
			}
			continue
		case scanLiteral:
			if c == '\'' {
				st.str = scanNone
			}
			continue
		}
		if c == '#' {
			return text[:i]
		}
		if c == '"' {
			st.str = scanBasic
		}
		if c == '\'' {
			st.str = scanLiteral
		}
	}
	return text
}

// assignment reports whether a line assigns a key with this name, and where its
// value sits on that line.
func assignment(text string, name string) (start, end int, ok bool) {
	i := 0
	for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
		i++
	}
	key, next, ok := readKey(text, i)
	if !ok || key != name {
		return 0, 0, false
	}
	i = next
	for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
		i++
	}
	if i >= len(text) || text[i] != '=' {
		return 0, 0, false
	}
	i++
	for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
		i++
	}
	start = i
	end = valueEnd(text, i)
	// Trailing spaces belong to the line, not to the value: keeping them out of
	// the span is what lets a trailing comment stay where the user put it.
	for end > start && (text[end-1] == ' ' || text[end-1] == '\t') {
		end--
	}
	if end <= start {
		return 0, 0, false
	}
	return start, end, true
}

// readKey reads a bare or quoted key and returns the offset after it.
func readKey(text string, i int) (key string, next int, ok bool) {
	if i >= len(text) {
		return "", 0, false
	}
	if text[i] == '"' || text[i] == '\'' {
		quoteChar := text[i]
		for j := i + 1; j < len(text); j++ {
			if text[j] == '\\' && quoteChar == '"' {
				j++
				continue
			}
			if text[j] == quoteChar {
				return text[i+1 : j], j + 1, true
			}
		}
		return "", 0, false
	}
	j := i
	for j < len(text) {
		c := text[j]
		bare := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			c == '_' || c == '-' || c == '.'
		if !bare {
			break
		}
		j++
	}
	if j == i {
		return "", 0, false
	}
	return text[i:j], j, true
}

// valueEnd is where the value starting at i ends on this line: at a comment, or
// at the newline. A value never runs past the end of its line here — the one
// multi-line shape this package edits, an array, is read by scanArray instead.
func valueEnd(text string, i int) int {
	var st scanner
	for ; i < len(text); i++ {
		c := text[i]
		switch st.str {
		case scanBasic:
			if c == '\\' {
				i++
				continue
			}
			if c == '"' {
				st.str = scanNone
			}
			continue
		case scanLiteral:
			if c == '\'' {
				st.str = scanNone
			}
			continue
		}
		if c == '#' || c == '\n' {
			return i
		}
		if c == '"' {
			st.str = scanBasic
		}
		if c == '\'' {
			st.str = scanLiteral
		}
	}
	return len(text)
}

// isSpace reports whether b is not part of any value.
func isSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\r' || b == '\n' }

// leadingSpace is the run of spaces and tabs at the start of s.
func leadingSpace(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return s[:i]
}

// insertAt puts s into text at offset.
func insertAt(text string, offset int, s string) string {
	if offset > len(text) {
		offset = len(text)
	}
	return text[:offset] + s + text[offset:]
}

// quote writes a basic string, escaping what has to be escaped.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
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
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// decodeString reads a TOML string, basic or literal.
func decodeString(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 {
		return "", false
	}
	if raw[0] == '\'' {
		if raw[len(raw)-1] != '\'' {
			return "", false
		}
		return raw[1 : len(raw)-1], true
	}
	if raw[0] != '"' || raw[len(raw)-1] != '"' {
		return "", false
	}
	body := raw[1 : len(raw)-1]
	var b strings.Builder
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		if i+1 >= len(body) {
			return "", false
		}
		i++
		switch body[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '"':
			b.WriteByte('"')
		case '\\':
			b.WriteByte('\\')
		case 'u', 'U':
			width := 4
			if body[i] == 'U' {
				width = 8
			}
			if i+width >= len(body) {
				return "", false
			}
			code, err := strconv.ParseUint(body[i+1:i+1+width], 16, 32)
			if err != nil {
				return "", false
			}
			b.WriteRune(rune(code))
			i += width
		default:
			return "", false
		}
	}
	return b.String(), true
}

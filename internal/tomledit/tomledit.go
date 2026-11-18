// Package tomledit edits a few keys of a TOML file in place, as text.
//
// The alternative — decode into config.Config, change a field, encode again —
// would drop every comment, blank line and bit of spacing the user wrote. The
// configuration file is meant to be read and edited by hand and kept in
// dotfiles; a command that reformats it on every use is not one anybody would
// run twice.
//
// So this package works on the bytes: it finds the region that holds a key and
// changes only that. When it cannot tell where the region ends it returns an
// error rather than guessing, and the caller re-parses the result before writing
// it, so a mistake here can never produce a file that will not load.
//
// It understands the TOML that this project's own configuration uses: top-level
// keys, `[[rule]]` tables, single-line and multi-line arrays, and comments. It is
// not a general TOML editor and does not try to be.
package tomledit

import (
	"errors"
	"fmt"
	"strings"
)

// Doc is a TOML file held as text.
type Doc struct {
	text string
}

// Parse holds the text; nothing is interpreted until a key is asked for.
func Parse(src []byte) *Doc { return &Doc{text: string(src)} }

// String is the file as it would be written.
func (d *Doc) String() string { return d.text }

// ErrNotUnderstood is what a caller gets when the text is not laid out in a way
// this package can edit safely. The message says what it could not find.
var ErrNotUnderstood = errors.New("tomledit: cannot edit this file")

// Block is one table of the file: the top-level keys, or one `[[rule]]`.
type Block struct {
	Kind  string // "rule" for a [[rule]] table, "" for the top-level keys
	Start int    // line index of the header (0 for the top level)
	End   int    // one past the last line of the block
	index int    // which [[rule]] this is, counting from one
}

// Index is the rule's position in the file, counting from one, so a message can
// call it "rule 3" when it has no name.
func (b Block) Index() int { return b.index }

// line is one line of the file, with its newline.
type line struct {
	text string // including "\n" when the source had one
}

// lines splits the text, keeping each line's terminator: rebuilding the file from
// these pieces gives back exactly the bytes that were read.
func (d *Doc) lines() []line {
	if d.text == "" {
		return nil
	}
	var out []line
	s := d.text
	for len(s) > 0 {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			out = append(out, line{text: s})
			break
		}
		out = append(out, line{text: s[:i+1]})
		s = s[i+1:]
	}
	return out
}

// join rebuilds the text from lines.
func join(lines []line) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l.text)
	}
	return b.String()
}

// Blocks returns the file's top-level region and every `[[rule]]` table, in
// order. A key that appears after a table header belongs to that table, which is
// why the boundaries matter: adding a top-level key at the end of the file would
// silently put it inside the last rule.
func (d *Doc) Blocks() []Block {
	lines := d.lines()
	var (
		blocks      []Block
		st          scanner
		open        = -1 // the block being read
		rules       int
		firstHeader = -1
	)
	for i, l := range lines {
		if st.atKeyLevel() {
			if kind, header := headerOf(l.text); header {
				if firstHeader < 0 {
					firstHeader = i
				}
				if open >= 0 {
					blocks[open].End = i
					open = -1
				}
				if kind == "rule" {
					rules++
					blocks = append(blocks, Block{Kind: "rule", Start: i, index: rules})
					open = len(blocks) - 1
				}
			}
		}
		st = st.advance(l.text)
	}
	if open >= 0 {
		blocks[open].End = len(lines)
	}
	if firstHeader < 0 {
		// No tables at all: every key is top-level.
		return []Block{{Kind: "", Start: 0, End: len(lines)}}
	}
	if firstHeader > 0 {
		// The keys above the first table header are the file's own. They form a
		// block of their own, which is what keeps a new top-level key out of the
		// first rule.
		blocks = append([]Block{{Kind: "", Start: 0, End: firstHeader}}, blocks...)
	}
	return blocks
}

// RuleBlocks is every `[[rule]]`, in order.
func (d *Doc) RuleBlocks() []Block {
	var out []Block
	for _, b := range d.Blocks() {
		if b.Kind == "rule" {
			out = append(out, b)
		}
	}
	return out
}

// FindRule returns the block whose `name` is name. Two rules with the same name
// are an error: there is no way to know which one was meant.
func (d *Doc) FindRule(name string) (Block, error) {
	var found []Block
	for _, b := range d.RuleBlocks() {
		if got, ok := d.StringValue(b, "name"); ok && got == name {
			found = append(found, b)
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return Block{}, fmt.Errorf("%w: no rule called %q", ErrNotUnderstood, name)
	default:
		return Block{}, fmt.Errorf("%w: %d rules are called %q; give them different names first", ErrNotUnderstood, len(found), name)
	}
}

// key is where a key's value sits.
type key struct {
	line       int // line index
	valueStart int // byte offset in the line, just after "="
	valueEnd   int // byte offset in the line, one past the value
}

// findKey looks for key inside a block, at the block's own level.
func (d *Doc) findKey(b Block, name string) (key, bool) {
	lines := d.lines()
	var st scanner
	for i := b.Start; i < b.End && i < len(lines); i++ {
		text := lines[i].text
		if st.atKeyLevel() && !isHeader(text) {
			if start, end, ok := assignment(text, name); ok {
				return key{line: i, valueStart: start, valueEnd: end}, true
			}
		}
		st = st.advance(text)
	}
	return key{}, false
}

// StringValue is the string value of a key in a block.
func (d *Doc) StringValue(b Block, name string) (string, bool) {
	raw := d.rawValue(b, name)
	if raw == "" {
		return "", false
	}
	return decodeString(raw)
}

// BoolValue is the value of a key that holds true or false.
func (d *Doc) BoolValue(b Block, name string) (bool, bool) {
	raw := d.rawValue(b, name)
	if raw == "" {
		return false, false
	}
	switch raw {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	return false, false
}

// rawValue is a key's value as written, trimmed.
func (d *Doc) rawValue(b Block, name string) string {
	k, ok := d.findKey(b, name)
	if !ok {
		return ""
	}
	return strings.TrimSpace(d.lines()[k.line].text[k.valueStart:k.valueEnd])
}

// SetBool makes `name = value` true in a block, adding the key when it is not
// there yet. Everything else on the line, including a trailing comment, is left
// alone.
func (d *Doc) SetBool(b Block, name string, value bool) error {
	literal := "false"
	if value {
		literal = "true"
	}
	lines := d.lines()

	if k, ok := d.findKey(b, name); ok {
		text := lines[k.line].text
		// Replace only the value: the spacing, any trailing comment and the line's
		// own ending stay exactly as the user wrote them.
		lines[k.line].text = text[:k.valueStart] + literal + text[k.valueEnd:]
		d.text = join(lines)
		return nil
	}

	// Not there: put it on its own line, right after the key that names the rule
	// so it reads where a person would expect it, indented like its neighbours.
	at := b.Start + 1
	indent := ""
	if named, ok := d.findKey(b, "name"); ok {
		at = named.line + 1
		indent = leadingSpace(lines[named.line].text)
	}
	if !strings.HasSuffix(lines[at-1].text, "\n") {
		lines[at-1].text += "\n" // the file did not end with a newline
	}
	newLine := line{text: indent + name + " = " + literal + "\n"}
	lines = append(lines[:at], append([]line{newLine}, lines[at:]...)...)
	d.text = join(lines)
	return nil
}

// Elements returns the string elements of a top-level array such as `folders`,
// and whether the key is there at all.
func (d *Doc) Elements(name string) ([]string, bool, error) {
	arr, ok, err := d.findArray(name)
	if err != nil || !ok {
		return nil, ok, err
	}
	out := make([]string, 0, len(arr.elements))
	for _, e := range arr.elements {
		value, ok := decodeString(e.raw)
		if !ok {
			return nil, true, fmt.Errorf("%w: %s holds something other than a string: %s", ErrNotUnderstood, name, e.raw)
		}
		out = append(out, value)
	}
	return out, true, nil
}

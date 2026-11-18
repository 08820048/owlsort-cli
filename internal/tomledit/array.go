package tomledit

import (
	"fmt"
	"strings"
)

// array is where a top-level array sits, and the elements it holds.
type array struct {
	openLine, openByte   int
	closeLine, closeByte int
	elements             []element
}

// element is one value inside an array, trimmed. raw is the bytes as written.
type element struct {
	startLine, startByte int
	endLine, endByte     int
	raw                  string
}

// offsets are the start offset of each line, so a byte offset maps back to a line
// and a column without rescanning the file.
type offsets []int

func (d *Doc) offsets() offsets {
	lines := d.lines()
	out := make(offsets, len(lines))
	n := 0
	for i, l := range lines {
		out[i] = n
		n += len(l.text)
	}
	return out
}

// at maps a byte offset to a line index and an offset within that line.
func (o offsets) at(off int) (line, col int) {
	lo, hi := 0, len(o)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if o[mid] <= off {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo, off - o[lo]
}

// findArray locates a top-level key holding an array.
func (d *Doc) findArray(name string) (array, bool, error) {
	var st scanner
	for i, l := range d.lines() {
		if st.atKeyLevel() && isHeader(l.text) {
			return array{}, false, nil // top-level keys only
		}
		if st.atKeyLevel() {
			if start, _, ok := assignment(l.text, name); ok {
				return d.scanArray(i, start)
			}
		}
		st = st.advance(l.text)
	}
	return array{}, false, nil
}

// scanArray walks from a value to the matching closing bracket and splits what it
// holds at the commas that sit at the array's own level.
func (d *Doc) scanArray(startLine, startByte int) (array, bool, error) {
	text := d.String()
	off := d.offsets()
	if startLine >= len(off) {
		return array{}, false, nil
	}
	from := off[startLine] + startByte
	// The value itself has to open with "[": searching forward would find the "["
	// of some later line and read the wrong array.
	if from >= len(text) || text[from] != '[' {
		return array{}, false, nil
	}
	open := from

	var (
		arr          array
		st           scanner
		depth        int
		elementStart = open + 1
	)
	arr.openLine, arr.openByte = off.at(open)

	add := func(from, to int) {
		for from < to && isSpace(text[from]) {
			from++
		}
		for to > from && isSpace(text[to-1]) {
			to--
		}
		if from >= to {
			return
		}
		el := element{raw: text[from:to]}
		el.startLine, el.startByte = off.at(from)
		el.endLine, el.endByte = off.at(to)
		arr.elements = append(arr.elements, el)
	}

	for i := open; i < len(text); i++ {
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
		switch c {
		case '"':
			st.str = scanBasic
		case '\'':
			st.str = scanLiteral
		case '#':
			for i < len(text) && text[i] != '\n' {
				i++
			}
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				arr.closeLine, arr.closeByte = off.at(i)
				add(elementStart, i)
				return arr, true, nil
			}
		case ',':
			if depth == 1 {
				add(elementStart, i)
				elementStart = i + 1
			}
		}
	}
	return array{}, false, fmt.Errorf("%w: an array is not closed", ErrNotUnderstood)
}

// Append adds value to a top-level array, creating the key when the file does not
// have it yet. A one-per-line array gets a new line in the same style; a one-line
// array gets ", value".
func (d *Doc) Append(name, value string) error {
	quoted := quote(value)
	arr, ok, err := d.findArray(name)
	if err != nil {
		return err
	}
	if !ok {
		return d.appendKey(name, "[\n  "+quoted+",\n]")
	}

	lines := d.lines()
	text := d.String()
	off := d.offsets()
	closeStart := off[arr.closeLine] + arr.closeByte

	if len(arr.elements) == 0 {
		if strings.Contains(text[off[arr.openLine]:closeStart], "\n") {
			// An empty array written over several lines: keep that style.
			indent := leadingSpace(lines[arr.closeLine].text) + "  "
			lines[arr.closeLine].text = indent + quoted + ",\n" + lines[arr.closeLine].text
			d.text = join(lines)
			return nil
		}
		lines[arr.closeLine].text = insertAt(lines[arr.closeLine].text, arr.closeByte, quoted)
		d.text = join(lines)
		return nil
	}

	last := arr.elements[len(arr.elements)-1]
	lastEnd := off[last.endLine] + last.endByte
	between := text[lastEnd:closeStart]

	onePerLine := strings.Contains(between, "\n") &&
		strings.TrimSpace(text[off[arr.closeLine]:closeStart]) == ""
	if onePerLine {
		if !strings.Contains(between, ",") {
			lines[last.endLine].text = insertAt(lines[last.endLine].text, last.endByte, ",")
		}
		indent := leadingSpace(lines[last.startLine].text)
		lines[arr.closeLine].text = indent + quoted + ",\n" + lines[arr.closeLine].text
		d.text = join(lines)
		return nil
	}

	lines[last.endLine].text = insertAt(lines[last.endLine].text, last.endByte, ", "+quoted)
	d.text = join(lines)
	return nil
}

// Remove takes value out of a top-level array and reports whether it was there.
// Removing something that is not there is not an error: the caller says so.
func (d *Doc) Remove(name, value string) (bool, error) {
	arr, ok, err := d.findArray(name)
	if err != nil || !ok {
		return false, err
	}
	for _, e := range arr.elements {
		got, ok := decodeString(e.raw)
		if !ok || got != value {
			continue
		}
		lines := d.lines()

		// One element alone on its line: take the whole line, so no blank line is
		// left behind. This is the shape a person writes a list in.
		before := lines[e.startLine].text[:e.startByte]
		after := lines[e.endLine].text[e.endByte:]
		if e.startLine == e.endLine && strings.TrimSpace(before) == "" &&
			(strings.TrimSpace(after) == "" || strings.TrimSpace(after) == ",") {
			lines = append(lines[:e.startLine], lines[e.startLine+1:]...)
			d.text = join(lines)
			return true, nil
		}

		if e.startLine == e.endLine {
			text := lines[e.startLine].text
			start, end := e.startByte, e.endByte
			// Swallow the comma after the element when it is on this line,
			// otherwise the one before it.
			j := end
			for j < len(text) && isSpace(text[j]) && !(e.startLine == arr.closeLine && j >= arr.closeByte) {
				j++
			}
			if j < len(text) && text[j] == ',' && !(e.startLine == arr.closeLine && j >= arr.closeByte) {
				j++
				for j < len(text) && isSpace(text[j]) && !(e.startLine == arr.closeLine && j >= arr.closeByte) {
					j++
				}
				end = j
			} else {
				for start > 0 && isSpace(text[start-1]) {
					start--
				}
				if start > 0 && text[start-1] == ',' {
					start--
				}
			}
			lines[e.startLine].text = text[:start] + text[end:]
			d.text = join(lines)
			return true, nil
		}

		// The element spans lines: drop it, joining what is left around it.
		joined := lines[e.startLine].text[:e.startByte] + lines[e.endLine].text[e.endByte:]
		lines = append(lines[:e.startLine], append([]line{{text: joined}}, lines[e.endLine+1:]...)...)
		d.text = join(lines)
		return true, nil
	}
	return false, nil
}

// appendKey adds a top-level key at the end of the top-level region: before the
// first table header, never after it, because a key after `[[rule]]` belongs to
// that rule.
func (d *Doc) appendKey(name, value string) error {
	lines := d.lines()
	at := len(lines)
	for _, b := range d.Blocks() {
		if b.Kind != "" {
			at = b.Start
			break
		}
	}
	if at > 0 && !strings.HasSuffix(lines[at-1].text, "\n") {
		lines[at-1].text += "\n"
	}
	newLine := line{text: name + " = " + value + "\n"}
	lines = append(lines[:at], append([]line{newLine}, lines[at:]...)...)
	d.text = join(lines)
	return nil
}

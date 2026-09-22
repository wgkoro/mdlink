package markdown

import (
	"bytes"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
)

type referenceDestination struct{ target, subpath string }
type referenceIndex struct {
	brackets    map[int]int
	definitions map[string]referenceDestination
	lines       map[int]int
}

func referenceLabels(content []byte) map[int]int {
	var result map[int]int
	var openings []int
	for cursor, c := range content {
		if c == '\r' || c == '\n' {
			openings = openings[:0]
			continue
		}
		if (c != '[' && c != ']') || escaped(content, cursor) {
			continue
		}
		if c == '[' {
			openings = append(openings, cursor)
		} else if len(openings) > 0 {
			start := openings[len(openings)-1]
			openings = openings[:len(openings)-1]
			if result == nil {
				result = map[int]int{}
			}
			result[start] = cursor
		}
	}
	return result
}

func referenceLabel(raw []byte) (string, bool) {
	if len(raw) == 0 || len(raw) > 999*utf8.UTFMax || !utf8.Valid(raw) || utf8.RuneCount(raw) > 999 || bytes.ContainsAny(raw, "\r\n") {
		return "", false
	}
	for i, c := range raw {
		if (c == '[' || c == ']') && !escaped(raw, i) {
			return "", false
		}
	}
	var normalized strings.Builder
	pendingSpace := false
	for _, r := range string(raw) {
		if unicode.Is(unicode.Zs, r) || r == '\t' || r == '\r' || r == '\n' || r == '\f' {
			pendingSpace = normalized.Len() > 0
			continue
		}
		if pendingSpace {
			normalized.WriteByte(' ')
			pendingSpace = false
		}
		normalized.WriteRune(r)
	}
	if normalized.Len() == 0 {
		return "", false
	}
	return cases.Fold().String(normalized.String()), true
}

func (refs *referenceIndex) definition(content []byte, start int) int {
	opening := start
	for opening < len(content) && opening-start < 4 && content[opening] == ' ' {
		opening++
	}
	if opening-start > 3 || opening >= len(content) || content[opening] != '[' {
		return start
	}
	closing, ok := refs.brackets[opening]
	if !ok || closing+1 >= len(content) || content[closing+1] != ':' {
		return start
	}
	label, valid := referenceLabel(content[opening+1 : closing])
	if !valid {
		return start
	}
	end := len(content)
	if next := bytes.IndexByte(content[closing+2:], '\n'); next >= 0 {
		end = closing + 2 + next
	}
	next := end
	if next < len(content) {
		next++
	}
	if refs.lines == nil {
		refs.lines = map[int]int{}
	}
	refs.lines[start] = next
	body := bytes.Trim(content[closing+2:end], " \t\r")
	if destination, ok := referenceDefinitionDestination(body); ok {
		if refs.definitions == nil {
			refs.definitions = map[string]referenceDestination{}
		}
		if _, exists := refs.definitions[label]; !exists {
			refs.definitions[label] = destination
		}
	}
	return next
}

func referenceDefinitionDestination(body []byte) (referenceDestination, bool) {
	// Reuse the inline delimiter grammar on this one definition line only.
	wrapped := make([]byte, 0, len(body)+5)
	wrapped = append(wrapped, "[x]("...)
	wrapped = append(wrapped, body...)
	wrapped = append(wrapped, ')')
	bounds, ok := inlineDestinations(wrapped)[3]
	if !ok || bounds.invalid || bounds.next != len(wrapped) {
		return referenceDestination{}, false
	}
	destination := bytes.Trim(wrapped[bounds.begin:bounds.end], " \t")
	if len(destination) == 0 || (!bounds.angle && quotedTitle(destination)) {
		return referenceDestination{}, false
	}
	if !bounds.angle {
		for i, c := range destination {
			if (c == ' ' || c == '\t') && !escaped(destination, i) {
				return referenceDestination{}, false
			}
		}
	}
	target, subpath := destinationParts(destination)
	return referenceDestination{target, subpath}, true
}

func (refs *referenceIndex) resolve(content []byte, start, closing int, allowShortcut bool) (referenceDestination, int, bool) {
	labelBytes := content[start+1 : closing]
	next := closing + 1
	full := next < len(content) && content[next] == '['
	if full {
		suffixEnd, ok := refs.brackets[next]
		if !ok {
			return referenceDestination{}, next + 1, false
		}
		if suffixEnd > next+1 {
			labelBytes = content[next+1 : suffixEnd]
		}
		next = suffixEnd + 1
	}
	label, valid := referenceLabel(labelBytes)
	if !valid {
		if full {
			return referenceDestination{}, next, false
		}
		return referenceDestination{}, start, false
	}
	destination, found := refs.definitions[label]
	if !full && (!allowShortcut || !found || !shortcutOwnsLabel(labelBytes)) {
		return referenceDestination{}, start, false
	}
	return destination, next, found
}

// A shortcut cannot hide a comment/code delimiter extending beyond its label.
// referenceLabel has already bounded this input to 999 Unicode code points.
func shortcutOwnsLabel(label []byte) bool {
	for cursor := 0; cursor < len(label); {
		if bytes.HasPrefix(label[cursor:], []byte("<!--")) {
			end := bytes.Index(label[cursor+4:], []byte("-->"))
			if end < 0 {
				return false
			}
			cursor += 4 + end + 3
			continue
		}
		if bytes.HasPrefix(label[cursor:], []byte("%%")) {
			end := bytes.Index(label[cursor+2:], []byte("%%"))
			if end < 0 {
				return false
			}
			cursor += 2 + end + 2
			continue
		}
		if label[cursor] == '`' {
			end := inlineCodeEnd(label, cursor)
			if end == cursor+runLength(label, cursor) {
				return false
			}
			cursor = end
			continue
		}
		cursor++
	}
	return true
}

package markdown

import (
	"bytes"
	"strings"
)

type LinkKind string

const (
	Wikilink     LinkKind = "wikilink"
	MarkdownLink LinkKind = "markdown"
)

type RawLink struct {
	SourceLogicalPath string
	RawTarget         string
	Subpath           string
	Kind              LinkKind
	Embed             bool
	Offset            int
	End               int
	RawLink           string
}

func Scan(source string, content []byte) []RawLink { return scan(source, content, false, nil) }

func ScanWithRaw(source string, content []byte) []RawLink { return scan(source, content, true, nil) }

func scan(source string, content []byte, keepRaw bool, mask *fragmentMask) []RawLink {
	if mask == nil && !bytes.Contains(content, []byte("[")) {
		return nil
	}
	refs := referenceIndex{brackets: referenceLabels(content)}
	if bytes.Contains(content, []byte("]:")) {
		var collectMask *fragmentMask
		if mask != nil {
			copy := *mask
			copy.body = nil
			collectMask = &copy
		}
		scanPass(source, content, false, collectMask, &refs, true)
	}
	return scanPass(source, content, keepRaw, mask, &refs, false)
}

func scanPass(source string, content []byte, keepRaw bool, mask *fragmentMask, refs *referenceIndex, collect bool) []RawLink {
	var links []RawLink
	var labels map[int]int
	var destinations map[int]destinationBounds
	if bytes.Contains(content, []byte("](")) {
		labels = inlineLabels(content)
		destinations = inlineDestinations(content)
	}
	for cursor := 0; cursor < len(content); {
		if mask != nil {
			if end := mask.codeEnd(cursor); end > cursor {
				cursor = end
				continue
			}
		}
		if cursor == 0 || content[cursor-1] == '\n' {
			if end := fencedCodeEnd(content, cursor); end > cursor {
				cursor = end
				continue
			}
			if collect {
				if end := refs.definition(content, cursor); end > cursor {
					cursor = end
					continue
				}
			} else if end, ok := refs.lines[cursor]; ok {
				cursor = end
				continue
			}
		}
		if content[cursor] == '<' || content[cursor] == '%' {
			if end := commentEnd(content, cursor); end > cursor {
				if mask != nil && !mask.comment(cursor, end) {
					return nil
				}
				cursor = end
				continue
			}
		}
		if content[cursor] == '`' {
			cursor = inlineCodeEnd(content, cursor)
			continue
		}
		if cursor+1 >= len(content) || content[cursor] != '[' || escaped(content, cursor) {
			cursor++
			continue
		}
		embed := cursor > 0 && content[cursor-1] == '!' && !escaped(content, cursor-1)
		offset := cursor
		if embed {
			offset--
		}
		if content[cursor+1] != '[' {
			if labelEnd, exists := labels[cursor]; exists {
				bounds, exists := destinations[labelEnd+1]
				if !exists {
					cursor++
					continue
				}
				cursor = bounds.next
				if bounds.invalid || mask != nil || collect {
					continue
				}
				destination := content[bounds.begin:bounds.end]
				if !bounds.angle {
					destination = bytes.Trim(destination, " \t")
					if quotedTitle(destination) {
						continue
					}
				}
				target, subpath := destinationParts(destination)
				if _, valid := DecodeInternalTarget(target); valid && (target != "" || subpath != "") {
					raw := RawLink{SourceLogicalPath: source, RawTarget: target, Subpath: subpath, Kind: MarkdownLink, Embed: embed, Offset: offset}
					if keepRaw {
						raw.End = bounds.next
						raw.RawLink = string(content[offset:bounds.next])
					}
					links = append(links, raw)
				}
				continue
			}
			if closing, ok := refs.brackets[cursor]; ok {
				destination, next, found := refs.resolve(content, cursor, closing, !collect)
				if next > cursor {
					if found && mask == nil && !collect {
						if _, valid := DecodeInternalTarget(destination.target); valid && (destination.target != "" || destination.subpath != "") {
							raw := RawLink{SourceLogicalPath: source, RawTarget: destination.target, Subpath: destination.subpath, Kind: MarkdownLink, Embed: embed, Offset: offset}
							if keepRaw {
								raw.End = next
								raw.RawLink = string(content[offset:next])
							}
							links = append(links, raw)
						}
					}
					cursor = next
					continue
				}
			}
			cursor++
			continue
		}

		start := cursor
		end := start + 2
		closed := false
		for end < len(content) && content[end] != '\n' && content[end] != '\r' {
			if end+1 < len(content) {
				if content[end] == '[' && content[end+1] == '[' {
					break
				}
				if content[end] == ']' && content[end+1] == ']' {
					closed = true
					break
				}
			}
			end++
		}
		if !closed {
			cursor = start + 2
			continue
		}
		if mask != nil || collect {
			cursor = end + 2
			continue
		}
		target, _, _ := strings.Cut(string(content[start+2:end]), "|")
		subpath := ""
		if index := strings.IndexByte(target, '#'); index >= 0 {
			target, subpath = target[:index], target[index:]
		}
		if strings.TrimSpace(target) == "" && subpath == "" {
			cursor = end + 2
			continue
		}
		raw := RawLink{SourceLogicalPath: source, RawTarget: target, Subpath: subpath, Kind: Wikilink, Embed: embed, Offset: offset}
		if keepRaw {
			raw.End = end + 2
			raw.RawLink = string(content[offset : end+2])
		}
		links = append(links, raw)
		cursor = end + 2
	}
	return links
}

func inlineLabels(content []byte) map[int]int {
	var labels map[int]int
	var pending []struct{ closing, depth int }
	depth := 0
	for cursor := len(content) - 1; cursor >= 0; cursor-- {
		if (content[cursor] != '[' && content[cursor] != ']') || escaped(content, cursor) {
			continue
		}
		if content[cursor] == ']' {
			depth++
			if cursor+1 < len(content) && content[cursor+1] == '(' {
				pending = append(pending, struct{ closing, depth int }{cursor, depth})
			}
		} else if depth > 0 {
			if last := len(pending) - 1; last >= 0 && pending[last].depth == depth {
				if labels == nil {
					labels = make(map[int]int)
				}
				labels[cursor] = pending[last].closing
				pending = pending[:last]
			}
			depth--
		}
	}
	return labels
}

type destinationBounds struct {
	begin, end, next int
	angle            bool
	invalid          bool
}

func inlineDestinations(content []byte) map[int]destinationBounds {
	var destinations map[int]destinationBounds
	var pending []struct{ opening, depth int }
	depth := 0
	for cursor, char := range content {
		if char == '\n' || char == '\r' {
			depth = 0
			pending = pending[:0]
			continue
		}
		if (char != '(' && char != ')') || escaped(content, cursor) {
			continue
		}
		if char == '(' {
			depth++
			if cursor > 0 && content[cursor-1] == ']' {
				pending = append(pending, struct{ opening, depth int }{cursor, depth})
			}
		} else if depth > 0 {
			if last := len(pending) - 1; last >= 0 && pending[last].depth == depth {
				opening := pending[last].opening
				if destinations == nil {
					destinations = make(map[int]destinationBounds)
				}
				destinations[opening] = destinationBounds{begin: opening + 1, end: cursor, next: cursor + 1}
				pending = pending[:last]
			}
			depth--
		}
	}
	titles, unclosed, pairs := titleBoundaries(content)
	destinations = applyTitles(content, destinations, titles, unclosed, pairs)
	angleEnd, angleClose, angleTail, nonSpace := -1, -1, -1, len(content)
	for cursor := len(content) - 1; cursor >= 0; cursor-- {
		char := content[cursor]
		if char == '\n' || char == '\r' {
			angleEnd, angleClose, angleTail, nonSpace = -1, -1, -1, len(content)
			continue
		}
		if char == '>' && !escaped(content, cursor) {
			angleEnd, angleClose, angleTail = cursor, -1, nonSpace
			if nonSpace < len(content) && content[nonSpace] == ')' {
				angleClose = nonSpace
			} else if closing, ok := titles[nonSpace]; ok {
				angleClose = closing
			}
		}
		if char == '(' && cursor > 0 && content[cursor-1] == ']' && nonSpace < len(content) && content[nonSpace] == '<' {
			fallback, hasFallback := destinations[cursor]
			delete(destinations, cursor)
			if angleClose < 0 && hasFallback {
				fallback.invalid = true
				destinations[cursor] = fallback
			} else if angleClose < 0 {
				if end, ok := unclosed[angleTail]; ok {
					if destinations == nil {
						destinations = map[int]destinationBounds{}
					}
					destinations[cursor] = destinationBounds{next: end, invalid: true}
				}
			}
			if angleClose >= 0 {
				if destinations == nil {
					destinations = make(map[int]destinationBounds)
				}
				destinations[cursor] = destinationBounds{begin: nonSpace + 1, end: angleEnd, next: angleClose + 1, angle: true}
			}
		}
		if char != ' ' && char != '\t' {
			nonSpace = cursor
		}
	}
	return destinations
}

func inlineCodeEnd(content []byte, start int) int {
	width := runLength(content, start)
	for cursor := start + width; cursor < len(content); {
		if content[cursor] != '`' {
			cursor++
			continue
		}
		closing := runLength(content, cursor)
		if closing == width {
			return cursor + closing
		}
		cursor += closing
	}
	return start + width
}

func runLength(content []byte, start int) int {
	end := start + 1
	for end < len(content) && content[end] == content[start] {
		end++
	}
	return end - start
}

func fencedCodeEnd(content []byte, start int) int {
	lineEnd := len(content)
	if index := bytes.IndexByte(content[start:], '\n'); index >= 0 {
		lineEnd = start + index
	}
	line := bytes.TrimSuffix(content[start:lineEnd], []byte{'\r'})
	marker, width, tail := fenceLine(line)
	if width < 3 || (marker == '`' && bytes.IndexByte(tail, '`') >= 0) {
		return start
	}
	for cursor := lineEnd + 1; cursor < len(content); {
		end := len(content)
		if index := bytes.IndexByte(content[cursor:], '\n'); index >= 0 {
			end = cursor + index
		}
		line := bytes.TrimSuffix(content[cursor:end], []byte{'\r'})
		closingMarker, closingWidth, tail := fenceLine(line)
		if closingMarker == marker && closingWidth >= width && len(bytes.Trim(tail, " \t")) == 0 {
			if end < len(content) {
				return end + 1
			}
			return end
		}
		cursor = end + 1
	}
	return len(content)
}

func fenceLine(line []byte) (byte, int, []byte) {
	indent := 0
	for indent < len(line) && line[indent] == ' ' {
		indent++
	}
	if indent > 3 || indent == len(line) || (line[indent] != '`' && line[indent] != '~') {
		return 0, 0, nil
	}
	width := runLength(line, indent)
	return line[indent], width, line[indent+width:]
}

func escaped(content []byte, index int) bool {
	count := 0
	for index > 0 && content[index-1] == '\\' {
		count++
		index--
	}
	return count%2 == 1
}

func commentEnd(content []byte, start int) int {
	opening, closing := "", ""
	if bytes.HasPrefix(content[start:], []byte("<!--")) {
		opening, closing = "<!--", "-->"
	} else if bytes.HasPrefix(content[start:], []byte("%%")) {
		opening, closing = "%%", "%%"
	} else {
		return start
	}
	end := bytes.Index(content[start+len(opening):], []byte(closing))
	if end < 0 {
		return len(content)
	}
	return start + len(opening) + end + len(closing)
}

// Precompute quote boundaries once so many malformed candidates cannot rescan a line.
func titleBoundaries(content []byte) (map[int]int, map[int]int, map[int]int) {
	if !bytes.ContainsAny(content, "\"'") {
		return nil, nil, nil
	}
	var titles, unclosed, pairs map[int]int
	type closingQuote struct{ position, paren int }
	quotes := [2]closingQuote{{-1, -1}, {-1, -1}}
	nonSpace, lineEnd := len(content), len(content)
	for cursor := len(content) - 1; cursor >= 0; cursor-- {
		c := content[cursor]
		if c == '\n' || c == '\r' {
			quotes = [2]closingQuote{{-1, -1}, {-1, -1}}
			nonSpace = cursor
			lineEnd = cursor
			continue
		}
		if (c == '\'' || c == '"') && !escaped(content, cursor) {
			which := 0
			if c == '"' {
				which = 1
			}
			next := quotes[which]
			if cursor > 0 && (content[cursor-1] == ' ' || content[cursor-1] == '\t') && !escaped(content, cursor-1) {
				if next.position < 0 {
					if unclosed == nil {
						unclosed = map[int]int{}
					}
					unclosed[cursor] = lineEnd
				} else {
					if pairs == nil {
						pairs = map[int]int{}
					}
					pairs[cursor] = next.position
					if next.paren >= 0 {
						if titles == nil {
							titles = map[int]int{}
						}
						titles[cursor] = next.paren
					}
				}
			}
			closing := -1
			if nonSpace < len(content) && content[nonSpace] == ')' {
				closing = nonSpace
			}
			quotes[which] = closingQuote{cursor, closing}
		}
		if c != ' ' && c != '\t' {
			nonSpace = cursor
		}
	}
	return titles, unclosed, pairs
}

func applyTitles(content []byte, destinations map[int]destinationBounds, titles, unclosed, pairs map[int]int) map[int]destinationBounds {
	if len(titles) == 0 && len(unclosed) == 0 {
		return destinations
	}
	type pendingDestination struct {
		opening, depth int
		quoteEnd       int
		angle          bool
	}
	var pending []pendingDestination
	depth := 0
	for cursor := 0; cursor < len(content); cursor++ {
		c := content[cursor]
		if c == '\n' || c == '\r' {
			pending = pending[:0]
			depth = 0
			continue
		}
		if c != '\'' && c != '"' && c != '(' && c != ')' {
			continue
		}
		if escaped(content, cursor) {
			continue
		}
		if (c == '\'' || c == '"') && len(pending) > 0 {
			last := pending[len(pending)-1]
			if last.depth == depth && !last.angle && cursor > last.quoteEnd {
				if closing, ok := titles[cursor]; ok {
					if destinations == nil {
						destinations = map[int]destinationBounds{}
					}
					destinations[last.opening] = destinationBounds{begin: last.opening + 1, end: cursor, next: closing + 1}
					cursor = closing
					depth--
					pending = pending[:len(pending)-1]
					continue
				}
				if end, ok := pairs[cursor]; ok {
					pending[len(pending)-1].quoteEnd = end
				}
				if end, ok := unclosed[cursor]; ok {
					if _, exists := destinations[last.opening]; !exists {
						if destinations == nil {
							destinations = map[int]destinationBounds{}
						}
						destinations[last.opening] = destinationBounds{next: end, invalid: true}
					}
				}
			}
		}
		if c == '(' {
			depth++
			if cursor > 0 && content[cursor-1] == ']' {
				start := cursor + 1
				for start < len(content) && (content[start] == ' ' || content[start] == '\t') {
					start++
				}
				pending = append(pending, pendingDestination{opening: cursor, depth: depth, quoteEnd: -1, angle: start < len(content) && content[start] == '<'})
			}
		} else if c == ')' && depth > 0 {
			if len(pending) > 0 && pending[len(pending)-1].depth == depth {
				pending = pending[:len(pending)-1]
			}
			depth--
		}
	}
	return destinations
}

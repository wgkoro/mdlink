package markdown

import (
	"bytes"
	"cmp"
	"slices"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"golang.org/x/text/unicode/norm"
)

// FragmentIndex owns only keys, not the parsed document or source body.
type FragmentIndex struct {
	headings, blocks                 map[string]int
	unsupported, unsupportedHeadings bool
}

func BuildFragments(content []byte) FragmentIndex {
	index := FragmentIndex{headings: map[string]int{}, blocks: map[string]int{}}
	body := bytes.Clone(content)
	if bytes.HasPrefix(body, []byte{0xef, 0xbb, 0xbf}) {
		body = body[3:]
	}
	if !maskFrontmatter(body) {
		index.unsupported = true
		return index
	}
	code, ambiguous := fragmentStructureRanges(body)
	mask := fragmentMask{body: body, code: code, ambiguous: ambiguous}
	scan("", body, false, &mask)
	if mask.unsupported {
		index.unsupported = true
		return index
	}
	document := goldmark.DefaultParser().Parse(text.NewReader(body))
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node.(type) {
		case *ast.Heading:
			key, valid := headingKey(node, body)
			if !valid {
				index.unsupportedHeadings = true
			} else if key != "" {
				index.headings[key]++
			}
			return ast.WalkSkipChildren, nil
		case *ast.Paragraph, *ast.TextBlock:
			if id := blockID(node, body); id != "" {
				index.blocks[id]++
			}
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return index
}

func ValidFragment(fragment string) bool {
	if len(fragment) < 2 || fragment[0] != '#' || strings.Contains(fragment[1:], "#") {
		return false
	}
	if fragment[1] == '^' {
		return validBlockID(fragment[2:])
	}
	return true
}

func (index FragmentIndex) Match(fragment string) string {
	if !ValidFragment(fragment) {
		return "unsupported-fragment"
	}
	count := 0
	if index.unsupported {
		return "unsupported-fragment"
	}
	if fragment[1] == '^' {
		count = index.blocks[fragment[2:]]
	} else {
		if index.unsupportedHeadings {
			return "unsupported-fragment"
		}
		count = index.headings[norm.NFC.String(fragment[1:])]
	}
	if count == 0 {
		return "missing-fragment"
	}
	if count > 1 {
		return "ambiguous-fragment"
	}
	return ""
}

func headingKey(heading ast.Node, body []byte) (string, bool) {
	var key strings.Builder
	valid := true
	_ = ast.Walk(heading, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := node.(type) {
		case *ast.Heading, *ast.Emphasis, *ast.Link, *ast.Image:
		case *ast.CodeSpan:
			for child := node.FirstChild(); child != nil; child = child.NextSibling() {
				value := child.(*ast.Text).Value(body)
				key.WriteString(strings.ReplaceAll(strings.ReplaceAll(string(value), "\r\n", "\n"), "\n", " "))
			}
			return ast.WalkSkipChildren, nil
		case *ast.Text:
			value := node.Value(body)
			if bytes.ContainsAny(value, "\\&") || bytes.Contains(value, []byte("[[")) {
				valid = false
			}
			key.Write(value)
			if node.SoftLineBreak() || node.HardLineBreak() {
				key.WriteByte(' ')
			}
		case *ast.String:
			if bytes.ContainsAny(node.Value, "\\&") || bytes.Contains(node.Value, []byte("[[")) {
				valid = false
			}
			key.Write(node.Value)
		default:
			valid = false
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return norm.NFC.String(strings.Trim(key.String(), " \t")), valid && !strings.Contains(key.String(), "[[")
}

func blockID(node ast.Node, body []byte) string {
	if node.Lines().Len() == 0 {
		return ""
	}
	last := node.Lines().At(node.Lines().Len() - 1)
	end := last.Stop
	for end > last.Start && strings.ContainsRune(" \t\r\n", rune(body[end-1])) {
		end--
	}
	start := end
	for start > last.Start && body[start-1] != '^' {
		start--
	}
	if start == last.Start {
		return ""
	}
	start--
	id := string(body[start+1 : end])
	if !validBlockID(id) || (start > last.Start && body[start-1] != ' ' && body[start-1] != '\t') {
		return ""
	}
	// The suffix must be ordinary text directly in the paragraph, not a label or code.
	leaf, ok := node.LastChild().(*ast.Text)
	if !ok || leaf.Segment.Start > start || leaf.Segment.Stop < end || escaped(body, start) {
		return ""
	}
	return id
}

func validBlockID(id string) bool {
	if id == "" {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !asciiLetter(c) && !(c >= '0' && c <= '9') && c != '-' {
			return false
		}
	}
	return true
}

func maskBytes(body []byte, start, end int) {
	for i := start; i < end; i++ {
		if body[i] != '\r' && body[i] != '\n' {
			body[i] = ' '
		}
	}
}

func maskFrontmatter(body []byte) bool {
	start := 0

	end := bytes.IndexByte(body[start:], '\n')
	if end < 0 {
		end = len(body) - start
	}
	end += start
	if !bytes.Equal(bytes.TrimSuffix(body[start:end], []byte{'\r'}), []byte("---")) {
		return true
	}
	for cursor := end + 1; cursor < len(body); {
		stop := len(body)
		if n := bytes.IndexByte(body[cursor:], '\n'); n >= 0 {
			stop = cursor + n
		}
		line := bytes.TrimSuffix(body[cursor:stop], []byte{'\r'})
		if bytes.Equal(line, []byte("---")) || bytes.Equal(line, []byte("...")) {
			maskBytes(body, 0, stop)
			return true
		}
		cursor = stop + 1
	}
	return false
}

// These ranges are used only while producing the comment-masked parser view.
type fragmentRange struct{ start, end int }
type fragmentMask struct {
	body          []byte
	code          []fragmentRange
	ambiguous     []fragmentRange
	ambiguousNext int
	next          int
	unsupported   bool
}

func (mask *fragmentMask) codeEnd(cursor int) int {
	for mask.next < len(mask.code) && mask.code[mask.next].end <= cursor {
		mask.next++
	}
	if mask.next < len(mask.code) && mask.code[mask.next].start <= cursor {
		return mask.code[mask.next].end
	}
	return cursor
}

func (mask *fragmentMask) comment(start, end int) bool {
	for mask.ambiguousNext < len(mask.ambiguous) && mask.ambiguous[mask.ambiguousNext].start < start {
		mask.ambiguousNext++
	}
	for i := mask.ambiguousNext; i < len(mask.ambiguous) && mask.ambiguous[i].start < end; i++ {
		if mask.ambiguous[i].end > end {
			mask.unsupported = true
			return false
		}
	}
	if mask.body != nil {
		maskBytes(mask.body, start, end)
	}
	return true
}

func fragmentStructureRanges(body []byte) ([]fragmentRange, []fragmentRange) {
	var ranges, ambiguous []fragmentRange
	document := goldmark.DefaultParser().Parse(text.NewReader(body))
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		if node.Type() == ast.TypeBlock && (node.FirstChild() == nil || node.FirstChild().Type() == ast.TypeInline) {
			start, end := -1, -1
			if node.Lines().Len() > 0 {
				first, last := node.Lines().At(0), node.Lines().At(node.Lines().Len()-1)
				start, end = first.Start, last.Stop
			}
			if html, ok := node.(*ast.HTMLBlock); ok && html.HasClosure() {
				if start < 0 {
					start = html.ClosureLine.Start
				}
				end = html.ClosureLine.Stop
			}
			if start >= 0 {
				ambiguous = append(ambiguous, fragmentRange{start, end})
			}
		}
		switch node := node.(type) {
		case *ast.CodeSpan:
			if first, ok := node.FirstChild().(*ast.Text); ok {
				last := node.LastChild().(*ast.Text)
				ranges = append(ranges, fragmentRange{first.Segment.Start, last.Segment.Stop})
			}
			return ast.WalkSkipChildren, nil
		case *ast.CodeBlock:
			if node.Lines().Len() > 0 {
				first, last := node.Lines().At(0), node.Lines().At(node.Lines().Len()-1)
				ranges = append(ranges, fragmentRange{first.Start, last.Stop})
			}
			return ast.WalkSkipChildren, nil
		case *ast.FencedCodeBlock:
			if node.Lines().Len() > 0 {
				first, last := node.Lines().At(0), node.Lines().At(node.Lines().Len()-1)
				// Include the opening line, whose info string can itself contain comment markers.
				lineStart := bytes.LastIndexByte(body[:first.Start], '\n') + 1
				start := 0
				if lineStart > 0 {
					start = bytes.LastIndexByte(body[:lineStart-1], '\n') + 1
				}
				ranges = append(ranges, fragmentRange{start, last.Stop})
			} else if node.Info != nil {
				ranges = append(ranges, fragmentRange{node.Info.Segment.Start, node.Info.Segment.Stop})
			}
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	ambiguous = append(ambiguous, ranges...)
	for i := range ambiguous {
		for ambiguous[i].end > ambiguous[i].start && bytes.ContainsRune([]byte(" \t\r\n"), rune(body[ambiguous[i].end-1])) {
			ambiguous[i].end--
		}
	}
	slices.SortFunc(ranges, func(a, b fragmentRange) int { return cmp.Compare(a.start, b.start) })
	slices.SortFunc(ambiguous, func(a, b fragmentRange) int { return cmp.Compare(a.start, b.start) })
	return ranges, ambiguous
}

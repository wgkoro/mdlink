package markdown

import (
	"net/url"
	"strings"
	"unicode/utf8"
)

func destinationParts(content []byte) (string, string) {
	query, fragment := -1, -1
	for i, c := range content {
		if (c != '?' && c != '#') || escaped(content, i) {
			continue
		}
		if c == '#' {
			fragment = i
			break
		}
		if query < 0 {
			query = i
		}
	}
	end, subpath := len(content), ""
	if fragment >= 0 {
		end, subpath = fragment, string(content[fragment:])
	}
	if query >= 0 {
		end = query
	}
	return string(content[:end]), subpath
}

func quotedTitle(content []byte) bool {
	for i := 1; i < len(content); i++ {
		quote := content[i]
		if (quote != '\'' && quote != '"') || (content[i-1] != ' ' && content[i-1] != '\t') || escaped(content, i) || escaped(content, i-1) {
			continue
		}
		end := i + 1
		for end < len(content) && (content[end] != quote || escaped(content, end)) {
			end++
		}
		if end >= len(content)-1 {
			return true
		}
		i = end
	}
	return false
}

func DecodeInternalTarget(raw string) (string, bool) {
	if externalTarget(raw) {
		return "", false
	}
	value := raw
	if strings.ContainsRune(raw, '\\') {
		var unescaped strings.Builder
		for i := 0; i < len(raw); i++ {
			if raw[i] == '\\' && i+1 < len(raw) && asciiPunctuation(raw[i+1]) {
				i++
			}
			unescaped.WriteByte(raw[i])
		}
		value = unescaped.String()
	}
	decoded, err := url.PathUnescape(value)
	if err != nil || !utf8.ValidString(decoded) || externalTarget(decoded) {
		return "", false
	}
	return decoded, true
}

func asciiPunctuation(value byte) bool {
	return value >= '!' && value <= '/' || value >= ':' && value <= '@' || value >= '[' && value <= '`' || value >= '{' && value <= '~'
}

func externalTarget(value string) bool {
	if strings.HasPrefix(value, "//") {
		return true
	}
	if len(value) == 0 || !asciiLetter(value[0]) {
		return false
	}
	for i := 1; i < len(value); i++ {
		c := value[i]
		if c == ':' {
			return true
		}
		if !asciiLetter(c) && !(c >= '0' && c <= '9') && c != '+' && c != '-' && c != '.' {
			return false
		}
	}
	return false
}

func asciiLetter(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

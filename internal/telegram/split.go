package telegram

import (
	"strings"
	"unicode/utf8"
)

// DefaultSplitLimit — a reserve from the Telegram limit (4096 UTF-16 units).
// 3800 runes is safe, even if the response is filled with emoji.
const DefaultSplitLimit = 3800

// SplitMessage cuts long text into pieces, trying to preserve
// the boundaries of paragraphs, lines, and sentences.
func SplitMessage(text string, limit int) []string {
	if limit <= 0 {
		limit = DefaultSplitLimit
	}

	if utf8.RuneCountInString(text) <= limit {
		return []string{text}
	}

	paragraphs := strings.Split(text, "\n\n")

	var chunks []string
	var current strings.Builder

	flush := func() {
		s := strings.TrimRight(current.String(), "\n")
		if s != "" {
			chunks = append(chunks, s)
		}
		current.Reset()
	}

	for _, p := range paragraphs {
		pLen := utf8.RuneCountInString(p)

		// If a paragraph itself is larger than the limit, we cut it separately.
		if pLen > limit {
			flush()
			for _, sub := range splitLongParagraph(p, limit) {
				chunks = append(chunks, sub)
			}
			continue
		}

		curLen := utf8.RuneCountInString(current.String())
		if curLen > 0 && curLen+2+pLen > limit {
			flush()
		}

		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(p)
	}
	flush()

	return chunks
}

func splitLongParagraph(s string, limit int) []string {
	lines := strings.Split(s, "\n")

	var chunks []string
	var current strings.Builder

	flush := func() {
		if current.Len() > 0 {
			chunks = append(chunks, current.String())
			current.Reset()
		}
	}

	for _, line := range lines {
		lineLen := utf8.RuneCountInString(line)

		if lineLen <= limit {
			curLen := utf8.RuneCountInString(current.String())
			if curLen > 0 && curLen+1+lineLen > limit {
				flush()
			}
			if current.Len() > 0 {
				current.WriteString("\n")
			}
			current.WriteString(line)
			continue
		}

		flush()
		chunks = append(chunks, splitByWords(line, limit)...)
	}

	flush()
	return chunks
}

func splitByWords(s string, limit int) []string {
	words := strings.Fields(s)

	var chunks []string
	var current strings.Builder

	flush := func() {
		if current.Len() > 0 {
			chunks = append(chunks, current.String())
			current.Reset()
		}
	}

	for _, w := range words {
		wLen := utf8.RuneCountInString(w)

		if wLen > limit {
			flush()
			chunks = append(chunks, splitRunes(w, limit)...)
			continue
		}

		curLen := utf8.RuneCountInString(current.String())
		if curLen > 0 && curLen+1+wLen > limit {
			flush()
		}
		if current.Len() > 0 {
			current.WriteString(" ")
		}
		current.WriteString(w)
	}
	flush()

	return chunks
}

func splitRunes(s string, limit int) []string {
	runes := []rune(s)
	var out []string
	for i := 0; i < len(runes); i += limit {
		end := i + limit
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, string(runes[i:end]))
	}
	return out
}
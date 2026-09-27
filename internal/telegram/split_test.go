package telegram

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSplitMessage_Short(t *testing.T) {
	got := SplitMessage("hello world", 100)
	if len(got) != 1 || got[0] != "hello world" {
		t.Fatalf("got %#v", got)
	}
}

func TestSplitMessage_Empty(t *testing.T) {
	got := SplitMessage("", 100)
	if len(got) != 1 || got[0] != "" {
		t.Fatalf("got %#v", got)
	}
}

func TestSplitMessage_ExactlyAtLimit(t *testing.T) {
	text := strings.Repeat("a", 100)
	got := SplitMessage(text, 100)
	if len(got) != 1 || got[0] != text {
		t.Fatalf("got %d chunks, want 1", len(got))
	}
}

func TestSplitMessage_Cyrillic(t *testing.T) {
	// 100 Cyrillic runes = 200 bytes. The limit is counted in runes.
	text := strings.Repeat("п", 100)
	got := SplitMessage(text, 100)
	if len(got) != 1 {
		t.Fatalf("expected 1 chunk for 100 cyrillic runes at limit=100, got %d", len(got))
	}
}

func TestSplitMessage_Emoji(t *testing.T) {
	// Emoji — 4 bytes in UTF-8, but 1 rune. The limit is in runes.
	text := strings.Repeat("😀", 50)
	got := SplitMessage(text, 100)
	if len(got) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(got))
	}
}

func TestSplitMessage_AllChunksRespectLimit(t *testing.T) {
	text := strings.Repeat("word ", 2000)
	const limit = 100
	chunks := SplitMessage(text, limit)
	for i, ch := range chunks {
		if n := utf8.RuneCountInString(ch); n > limit {
			t.Errorf("chunk %d has %d runes > %d", i, n, limit)
		}
	}
}

func TestSplitMessage_PreservesContent(t *testing.T) {
	text := "para one\n\npara two with more tokens here\n\n" +
		strings.Repeat("word ", 500)
	chunks := SplitMessage(text, 80)
	joined := strings.Join(chunks, " ")

	if !strings.Contains(joined, "para one") {
		t.Error("first paragraph lost")
	}
	if !strings.Contains(joined, "para two") {
		t.Error("second paragraph lost")
	}
	if n := strings.Count(joined, "word"); n != 500 {
		t.Errorf("word count = %d, want 500", n)
	}
}

func TestSplitMessage_SingleLongWordHardSplit(t *testing.T) {
	text := strings.Repeat("x", 5000)
	const limit = 1000
	chunks := SplitMessage(text, limit)

	total := 0
	for i, ch := range chunks {
		n := utf8.RuneCountInString(ch)
		if n > limit {
			t.Errorf("chunk %d has %d runes > %d", i, n, limit)
		}
		total += n
	}
	if total != 5000 {
		t.Errorf("total runes = %d, want 5000", total)
	}
}

func TestSplitMessage_DefaultLimit(t *testing.T) {
	text := strings.Repeat("a", DefaultSplitLimit+1)
	chunks := SplitMessage(text, 0)
	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks, got %d", len(chunks))
	}
}
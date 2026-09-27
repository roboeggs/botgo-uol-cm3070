package i18n

import (
	"testing"
	"strings"
)

func TestT_Translation(t *testing.T) {
	Init()
	userID := int64(123)
	SetLanguageCacheOnly(userID, "ru")

	msg := T(userID, "greeting", map[string]interface{}{"Name": "Test"})
	if !strings.Contains(msg, "Привет") {
		t.Errorf("RU greeting should contain 'Привет', got %q", msg)
	}
	if !strings.Contains(msg, "Test") {
		t.Errorf("RU greeting should contain the user name, got %q", msg)
	}
	if strings.Contains(msg, "<no value>") {
		t.Errorf("RU greeting has unsubstituted placeholder: %q", msg)
	}

	msg = T(userID, "help_message")
	if msg == "" {
		t.Error("Help message should not be empty")
	}

	SetLanguageCacheOnly(userID, "en")
	msg = T(userID, "greeting", map[string]interface{}{"Name": "Test"})
	if !strings.Contains(msg, "Hi") {
		t.Errorf("EN greeting should contain 'Hi', got %q", msg)
	}
	if !strings.Contains(msg, "Test") {
		t.Errorf("EN greeting should contain the user name, got %q", msg)
	}
	if strings.Contains(msg, "<no value>") {
		t.Errorf("EN greeting has unsubstituted placeholder: %q", msg)
	}
}

func TestT_WithTemplateData(t *testing.T) {
	Init()
	userID := int64(456)
	SetLanguageCacheOnly(userID, "ru")

	msg := T(userID, "echo", map[string]interface{}{"Text": "Hello"})
	expected := "Ты написал: Hello"
	if msg != expected {
		t.Errorf("Expected %q, got %q", expected, msg)
	}

	SetLanguageCacheOnly(userID, "en")
	msg = T(userID, "echo", map[string]interface{}{"Text": "Hello"})
	expected = "You wrote: Hello"
	if msg != expected {
		t.Errorf("Expected %q, got %q", expected, msg)
	}
}

func TestT_MissingKey(t *testing.T) {
	Init()
	userID := int64(789)
	SetLanguageCacheOnly(userID, "ru")

	msg := T(userID, "nonexistent")
	if msg != "nonexistent" {
		t.Errorf("Expected %q, got %q", "nonexistent", msg)
	}
}

// Test SetLanguageCacheOnly (without DB)
func TestSetLanguageCacheOnly(t *testing.T) {
	Init()
	userID := int64(999)
	SetLanguageCacheOnly(userID, "en")
	msg := T(userID, "greeting", map[string]interface{}{"Name": "Test"})
	if !strings.Contains(msg, "Hi") {
		t.Errorf("EN greeting should contain 'Hi', got %q", msg)
	}
}
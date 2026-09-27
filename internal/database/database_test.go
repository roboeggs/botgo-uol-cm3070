package database

import (
	"testing"
)

func TestInitDB(t *testing.T) {
	err := InitDB("invalid-dsn")
	if err == nil {
		t.Error("Expected error for invalid DSN, got nil")
	}
}
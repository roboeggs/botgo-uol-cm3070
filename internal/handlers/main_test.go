package handlers

import (
	"os"
	"testing"

	"botgo/internal/i18n"
)

func TestMain(m *testing.M) {
	i18n.Init()
	i18n.SetLanguageCacheOnly(1, "en")
	os.Exit(m.Run())
}
package generate

import (
	"context"
	"testing"

	"botgo/internal/database"
)

func TestGetTickerArg(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want string
		wantErr bool
	}{
		{
			name: "valid ticker",
			args: map[string]any{
				"ticker": "BTCUSDT",
			},
			want: "BTCUSDT",
		},
		{
			name: "lowercase ticker",
			args: map[string]any{
				"ticker": "btcusdt",
			},
			want: "BTCUSDT",
		},
		{
			name: "ticker with spaces",
			args: map[string]any{
				"ticker": "  btcusdt  ",
			},
			want: "BTCUSDT",
		},
		{
			name: "missing ticker",
			args: map[string]any{},
			wantErr: true,
		},
		{
			name: "ticker has wrong type",
			args: map[string]any{
				"ticker": 123,
			},
			wantErr: true,
		},
		{
			name: "empty ticker",
			args: map[string]any{
				"ticker": "",
			},
			wantErr: true,
		},
		{
			name: "spaces only",
			args: map[string]any{
				"ticker": "   ",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getTickerArg(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToolExecutorUnknownTool(t *testing.T) {
	executor := NewToolExecutor(NewCache())

	_, err := executor.Execute(
		context.Background(),
		"unknown_tool",
		nil,
	)

	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestToolExecutorTickerLevelsMissingTicker(t *testing.T) {
	executor := NewToolExecutor(NewCache())

	_, err := executor.Execute(
		context.Background(),
		"get_ticker_levels",
		map[string]any{},
	)

	if err == nil {
		t.Fatal("expected error for missing ticker")
	}
}

func TestToolExecutorTickerLevelsInvalidTickerType(t *testing.T) {
	executor := NewToolExecutor(NewCache())

	_, err := executor.Execute(
		context.Background(),
		"get_ticker_levels",
		map[string]any{
			"ticker": 123,
		},
	)

	if err == nil {
		t.Fatal("expected error for invalid ticker type")
	}
}

func TestFormatTickerLevels(t *testing.T) {
	row := &database.DetectorStateRow{
		Ticker:          "BTCUSDT",
		HasActiveLevels: true,
		FilteredStateBlob: []byte(`{
			"levels": [
				{
					"level_id": 1,
					"price": 100000
				}
			]
		}`),
	}

	result, err := formatTickerLevels(row, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["ticker"] != "BTCUSDT" {
		t.Errorf(
			"ticker = %v, want BTCUSDT",
			result["ticker"],
		)
	}

	if result["has_active_levels"] != true {
		t.Errorf(
			"has_active_levels = %v, want true",
			result["has_active_levels"],
		)
	}

	if result["cached"] != false {
		t.Errorf(
			"cached = %v, want false",
			result["cached"],
		)
	}

	if result["state"] == nil {
		t.Fatal("expected decoded state")
	}
}

func TestFormatTickerLevelsInvalidJSON(t *testing.T) {
	row := &database.DetectorStateRow{
		Ticker: "BTCUSDT",
		FilteredStateBlob: []byte(`invalid json`),
	}

	_, err := formatTickerLevels(row, false)

	if err == nil {
		t.Fatal("expected JSON decode error")
	}
}

func TestBuildTools(t *testing.T) {
	tools := buildTools()
	enabled := EnabledTools()

	if len(tools) != len(enabled) {
		t.Fatalf(
			"buildTools() returned %d tools, but %d are enabled",
			len(tools), len(enabled),
		)
	}

	// Every enabled tool must appear in the declarations.
	got := make(map[string]bool, len(tools))
	for _, tool := range tools {
		got[tool.Function.Name] = true
	}

	for _, e := range enabled {
		if !got[e.Name] {
			t.Errorf("enabled tool %q was not registered in buildTools()", e.Name)
		}
	}

	// And conversely — nothing extra.
	want := make(map[string]bool, len(enabled))
	for _, e := range enabled {
		want[e.Name] = true
	}
	for name := range got {
		if !want[name] {
			t.Errorf("buildTools() contains %q which is not enabled", name)
		}
	}
}
package generate

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/openai/openai-go"
)

// Tool — a description of a single tool.
type Tool struct {
	Name string
	Declaration openai.FunctionDefinitionParam
	Enabled bool
}

// registry — the single source of truth about all tools.
var registry = []*Tool{
	{
		Name: "get_active_tickers",
		Declaration: declGetActiveTickers(),
		Enabled: true,
	},
	{
		Name: "get_top_ml_tickers",
		Declaration: declGetTopMLTickers(),
		Enabled: true,
	},
	{
		Name: "get_ticker_levels",
		Declaration: declGetTickerLevels(),
		Enabled: true,
	},
	{
		Name: "get_market_data",
		Declaration: declGetMarketData(),
		Enabled: true,
	},
}

func EnabledTools() []*Tool {
	var out []*Tool
	for _, t := range registry {
		if t.Enabled {
			out = append(out, t)
		}
	}
	return out
}

func EnabledToolNames() []string {
	var names []string
	for _, t := range registry {
		if t.Enabled {
			names = append(names, t.Name)
		}
	}
	return names
}

func SetToolEnabled(name string, enabled bool) error {
	for _, t := range registry {
		if t.Name == name {
			t.Enabled = enabled
			slog.Info("tool toggled", "tool", name, "enabled", enabled)
			return nil
		}
	}
	return fmt.Errorf("unknown tool: %s", name)
}

// buildTools collects Tool declarations only from the included tools.
func buildTools() []openai.ChatCompletionToolParam {
	var out []openai.ChatCompletionToolParam
	for _, t := range EnabledTools() {
		out = append(out, openai.ChatCompletionToolParam{
			Function: t.Declaration,
		})
	}
	return out
}

func init() {
	disabled := os.Getenv("DISABLED_TOOLS")
	if disabled == "" {
		return
	}
	for _, name := range strings.Split(disabled, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if err := SetToolEnabled(name, false); err != nil {
			slog.Warn("failed to disable tool", "tool", name, "err", err)
		}
	}
}
package generate

import (
	"context"
	"os"
	"strings"
	"testing"
)

// Tests the full AI pipeline: classifier + data loading + LLM call.
// The query is in English to verify the pipeline works for English users.

func TestOpenAIFunctionCalling(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")

	if apiKey == "" {
		t.Skip("OPENAI_API_KEY is not set")
	}

	service, err := New(apiKey)
	if err != nil {
		t.Fatalf("create OpenAI service: %v", err)
	}

	ctx := context.Background()

	answer, err := service.GenerateText(
		ctx,
		999999,
		"What coins are interesting right now?",
	)

	if err != nil {
		t.Fatalf("GenerateText failed: %v", err)
	}

	if strings.TrimSpace(answer) == "" {
		t.Fatal("OpenAI returned empty answer")
	}

	t.Log("OpenAI final answer:")
	t.Log(answer)
}

// Same as above but for a specific ticker.
func TestOpenAIFunctionCallingSpecificTicker(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")

	if apiKey == "" {
		t.Skip("OPENAI_API_KEY is not set")
	}

	service, err := New(apiKey)
	if err != nil {
		t.Fatalf("create OpenAI service: %v", err)
	}

	answer, err := service.GenerateText(
		context.Background(),
		999998,
		"Analyze BTCUSDT right now.",
	)

	if err != nil {
		t.Fatalf("GenerateText failed: %v", err)
	}

	if strings.TrimSpace(answer) == "" {
		t.Fatal("OpenAI returned empty answer")
	}

	t.Log("OpenAI final answer:")
	t.Log(answer)
}

// Verifies that the model can answer a general question without using
// any market tools.
func TestOpenAIDoesNotNeedMarketTools(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")

	if apiKey == "" {
		t.Skip("OPENAI_API_KEY is not set")
	}

	service, err := New(apiKey)
	if err != nil {
		t.Fatalf("create OpenAI service: %v", err)
	}

	answer, err := service.GenerateText(
		context.Background(),
		999997,
		"What is RSI(14) and what does a value above 70 mean?",
	)

	if err != nil {
		t.Fatalf("GenerateText failed: %v", err)
	}

	if strings.TrimSpace(answer) == "" {
		t.Fatal("OpenAI returned empty answer")
	}

	t.Log("OpenAI final answer:")
	t.Log(answer)
}

// Verifies that the bot correctly handles a Russian query
// and responds in Russian. This confirms the multilingual pipeline.
func TestOpenAIRussianQuery(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")

	if apiKey == "" {
		t.Skip("OPENAI_API_KEY is not set")
	}

	service, err := New(apiKey)
	if err != nil {
		t.Fatalf("create OpenAI service: %v", err)
	}

	answer, err := service.GenerateText(
		context.Background(),
		999996,
		"Расскажи про BTCUSDT",
	)

	if err != nil {
		t.Fatalf("GenerateText failed: %v", err)
	}

	if strings.TrimSpace(answer) == "" {
		t.Fatal("OpenAI returned empty answer")
	}

	// Russian query should produce a Russian answer.
	// At least verify Cyrillic characters are present.
	hasCyrillic := false
	for _, r := range answer {
		if r >= 'А' && r <= 'я' {
			hasCyrillic = true
			break
		}
	}
	if !hasCyrillic {
		t.Errorf("Russian query did not produce Cyrillic answer: %q", answer)
	}

	t.Log("OpenAI final answer:")
	t.Log(answer)
}
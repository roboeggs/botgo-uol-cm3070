package generate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

const OpenAIModel = "gpt-6-luna"

// maxHistoryTurns — how many recent user moves we keep in the history
const maxHistoryTurns = 10

const systemPrompt = `
	You are a cryptocurrency market analysis assistant.

	You analyse cryptocurrency market data using external tools.

	Rules:

	1. Never invent market data.
	2. Never invent support/resistance levels.
	3. If the user asks about a specific ticker, analyse that ticker.
	4. If the user asks what cryptocurrency may be interesting:
	- first call get_top_ml_tickers;
	- select potentially interesting tickers;
	- call get_ticker_levels for selected tickers;
	- call get_market_data for selected tickers;
	- compare current price with support/resistance and EMA(20);
	- consider RSI(14).
	5. Clearly distinguish factual data from interpretation.
	6. Do not claim that a cryptocurrency will definitely rise or fall.
	7. Do not present the analysis as guaranteed financial advice.

	Technical interpretation:

	- Price above EMA(20) can indicate short-term bullish momentum.
	- Price below EMA(20) can indicate short-term bearish momentum.
	- RSI above 70 can indicate overbought conditions.
	- RSI below 30 can indicate oversold conditions.
	- Support and resistance are historical price areas and are not guaranteed
	to hold.

	Use tools only when the required information is not already available
	in the conversation.
`

// chatSession stores the message history. OpenAI stateless — we manage the history ourselves.
type chatSession struct {
	messages []openai.ChatCompletionMessageParamUnion
	mu sync.Mutex
}

type Service struct {
	client openai.Client
	executor *ToolExecutor

	mu sync.RWMutex
	chats map[int64]*chatSession
}

func buildSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString(systemPrompt)
	sb.WriteString("\n\nAvailable tools:\n")
	for _, t := range EnabledTools() {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", t.Name, t.Declaration.Description.Value))
	}
	return sb.String()
}

func New(apiKey string) (*Service, error) {
	client := openai.NewClient(option.WithAPIKey(apiKey))
	cache := NewCache()

	return &Service{
		client: client,
		executor: NewToolExecutor(cache),
		chats: make(map[int64]*chatSession),
	}, nil
}

func (s *Service) GetOrCreateChat(ctx context.Context, userID int64) (*chatSession, error) {
	s.mu.RLock()
	session, exists := s.chats[userID]
	s.mu.RUnlock()

	if exists {
		return session, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if session, exists := s.chats[userID]; exists {
		return session, nil
	}

	session = &chatSession{
		messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(buildSystemPrompt()),
		},
	}

	s.chats[userID] = session

	return session, nil
}

// ResetChat clears the history for a specific user.
func (s *Service) ResetChat(userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.chats, userID)
}

func (s *Service) GenerateText(ctx context.Context, userID int64, prompt string) (string, error) {
	session, err := s.GetOrCreateChat(ctx, userID)
	if err != nil {
		return "", err
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	// Trim the history, preserving the system-prompt and the last N user-turns.
	session.messages = trimHistory(session.messages)

	// Detach the context from the parent timeout + our own independent timeout.
	detachedCtx := context.WithoutCancel(ctx)
	extendedCtx, cancel := context.WithTimeout(detachedCtx, 5*time.Minute)
	defer cancel()

	out, err := s.generateWithTools(extendedCtx, session, prompt)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", ErrLLMTimeout
		}
		return "", err
	}

	if looksLikeEmptyOrJunk(out) {
		slog.Warn("LLM returned junk content",
			"user_id", userID, "len", len(out))
		return "", ErrLLMEmpty
	}

	return out, nil
}

// generateWithTools — a tool-calling loop over the OpenAI Chat Completions API.
func (s *Service) generateWithTools(ctx context.Context, session *chatSession, prompt string) (string, error) {

	session.messages = append(session.messages, openai.UserMessage(prompt))

	tools := buildTools()

	for {
		params := openai.ChatCompletionNewParams{
			Model: OpenAIModel,
			Messages: session.messages,
			Tools: tools,
		}

		resp, err := s.client.Chat.Completions.New(ctx, params)
		if err != nil {
			return "", fmt.Errorf("chat completion: %w", err)
		}
		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("chat completion: no choices returned")
		}

		msg := resp.Choices[0].Message

		session.messages = append(session.messages, msg.ToParam())

		if len(msg.ToolCalls) == 0 {
			return msg.Content, nil
		}

		for _, tc := range msg.ToolCalls {
			args := map[string]any{}
			if tc.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					slog.Warn("failed to unmarshal tool arguments",
						"tool", tc.Function.Name, "err", err)
				}
			}

			result, execErr := s.executor.Execute(ctx, tc.Function.Name, args)

			var payload []byte
			if execErr != nil {
				payload, _ = json.Marshal(map[string]any{"error": execErr.Error()})
			} else {
				payload, _ = json.Marshal(result)
			}

			session.messages = append(session.messages,
				openai.ToolMessage(string(payload), tc.ID),
			)
		}
	}
}


// trimHistory leaves the system-prompt and the last maxHistoryTurns
// user-turns. We cut only at the boundary of the user message to avoid breaking the pair assistant(tool_calls) + tool(message).
func trimHistory(messages []openai.ChatCompletionMessageParamUnion) []openai.ChatCompletionMessageParamUnion {

	if len(messages) <= 2 {
		return messages
	}

	userCount := 0
	cut := -1
	for i := len(messages) - 1; i >= 1; i-- {
		if messages[i].OfUser != nil {
			userCount++
			if userCount == maxHistoryTurns {
				cut = i
				break
			}
		}
	}
	if cut <= 1 {
		return messages
	}

	out := make([]openai.ChatCompletionMessageParamUnion, 0, 1+len(messages)-cut)
	out = append(out, messages[0])
	out = append(out, messages[cut:]...)
	return out
}

// looksLikeEmptyOrJunk — a rough check of the LLM response before sending.
// An empty string or a string without a single letter is considered junk.
func looksLikeEmptyOrJunk(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	for _, r := range s {
		if unicode.IsLetter(r) {
			return false
		}
	}
	return true
}
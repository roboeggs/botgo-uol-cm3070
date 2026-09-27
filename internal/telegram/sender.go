package telegram

import (
	"context"
	"log/slog"
	"time"
)

type SendFunc func(ctx context.Context, chatID int64, text string) error
type RetryAfterFunc func(err error) (seconds int, ok bool)

type OutboundMessage struct {
	ChatID int64
	Text string
}

const (
	MaxChunksPerReply = 5
	MinSendInterval = 350 * time.Millisecond
	MinChunkDelay = 300 * time.Millisecond
	QueueSize = 1024
)

type Sender struct {
	send SendFunc
	retryAfter RetryAfterFunc
	queue chan OutboundMessage
	interval time.Duration
	chunkDelay time.Duration
}

func NewSender(send SendFunc, retryAfter RetryAfterFunc) *Sender {
	return newSender(send, retryAfter, MinSendInterval, MinChunkDelay, QueueSize)
}

// newSender — an internal constructor used by tests
// to reset intervals and reduce the queue.
func newSender(send SendFunc, retryAfter RetryAfterFunc, interval, chunkDelay time.Duration, queueSize int) *Sender {
	s := &Sender{
		send: send,
		retryAfter: retryAfter,
		queue: make(chan OutboundMessage, queueSize),
		interval: interval,
		chunkDelay: chunkDelay,
	}
	go s.run()
	return s
}

func (s *Sender) Enqueue(chatID int64, text string) {
	select {
	case s.queue <- OutboundMessage{ChatID: chatID, Text: text}:
	default:
		slog.Warn("outbound queue full, dropping message",
			"chat_id", chatID, "len", len(text))
	}
}

func (s *Sender) run() {
	for msg := range s.queue {
		s.sendLong(context.Background(), msg.ChatID, msg.Text)
		if s.interval > 0 {
			time.Sleep(s.interval)
		}
	}
}

func (s *Sender) sendLong(ctx context.Context, chatID int64, text string) {
	chunks := SplitMessage(text, DefaultSplitLimit)

	if len(chunks) > MaxChunksPerReply {
		slog.Warn("reply too long, truncating",
			"chat_id", chatID, "chunks", len(chunks))
		chunks = chunks[:MaxChunksPerReply]
		chunks[MaxChunksPerReply-1] += "\n\n…(the answer is cut off)"
	}

	for i, chunk := range chunks {
		if i > 0 && s.chunkDelay > 0 {
			time.Sleep(s.chunkDelay)
		}
		if err := s.sendWithRetry(ctx, chatID, chunk); err != nil {
			slog.Error("send chunk failed",
				"chat_id", chatID, "chunk", i, "err", err)
			return
		}
	}
}

func (s *Sender) sendWithRetry(ctx context.Context, chatID int64, text string) error {
	err := s.send(ctx, chatID, text)
	if err == nil {
		return nil
	}

	if seconds, ok := s.retryAfter(err); ok {
		if seconds > 0 {
			slog.Warn("telegram rate limit, waiting",
				"chat_id", chatID, "retry_after_sec", seconds)
			time.Sleep(time.Duration(seconds) * time.Second)
		}
		return s.send(ctx, chatID, text)
	}

	return err
}
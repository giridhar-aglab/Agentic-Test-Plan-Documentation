package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"

	"github.com/giri-ms19/testplan-agent/internal/llm"
)

// SpillStore holds tool payloads too large to keep in context. The model sees
// a handle and a short digest and re-fetches by handle only where it is
// actually reading. This is what makes a large repository tractable: a raw file
// listing would swamp the window, while a digest plus handle costs almost
// nothing.
type SpillStore struct {
	mutex            sync.RWMutex
	payloadsByHandle map[string]string
	handleSequence   int
}

// NewSpillStore builds an empty store.
func NewSpillStore() *SpillStore {
	return &SpillStore{payloadsByHandle: map[string]string{}}
}

// Put stores a payload and returns its handle.
func (spillStore *SpillStore) Put(payload string) string {
	spillStore.mutex.Lock()
	defer spillStore.mutex.Unlock()
	spillStore.handleSequence++
	contentHash := sha256.Sum256([]byte(payload))
	handle := fmt.Sprintf("spill-%03d-%s", spillStore.handleSequence, hex.EncodeToString(contentHash[:4]))
	spillStore.payloadsByHandle[handle] = payload
	return handle
}

// Get retrieves a spilled payload.
func (spillStore *SpillStore) Get(handle string) (string, bool) {
	spillStore.mutex.RLock()
	defer spillStore.mutex.RUnlock()
	payload, found := spillStore.payloadsByHandle[handle]
	return payload, found
}

// Digest returns a short summary of a payload, used in place of the payload
// itself once it has been spilled.
func Digest(payload string, maxCharacters int) string {
	trimmedPayload := strings.TrimSpace(payload)
	if len(trimmedPayload) <= maxCharacters {
		return trimmedPayload
	}
	lineCount := strings.Count(trimmedPayload, "\n") + 1
	return fmt.Sprintf("%s\n… (%d lines, %d bytes total; fetch the rest by handle)",
		trimmedPayload[:maxCharacters], lineCount, len(trimmedPayload))
}

// WorkingSet is one agent's short-term context: the message history for a
// single invocation, bounded by a token budget. When it fills, the oldest turns
// are summarised and dropped rather than the run failing.
type WorkingSet struct {
	SystemPrompt string

	messages          []llm.Message
	compactionSummary string
	compactionCount   int

	maxContextTokens  int
	compactAtFraction float64
	// Summarise turns dropped messages into a paragraph. Injectable so tests do
	// not need a model, and so a cheap tier can do the summarising in
	// production.
	Summarise func(droppedMessages []llm.Message) string
}

// NewWorkingSet builds a working set bounded by the given context window.
func NewWorkingSet(systemPrompt string, maxContextTokens int) *WorkingSet {
	return &WorkingSet{
		SystemPrompt:      systemPrompt,
		maxContextTokens:  maxContextTokens,
		compactAtFraction: 0.7,
		Summarise:         defaultSummarise,
	}
}

// Append adds a message, compacting first if the window is filling.
func (workingSet *WorkingSet) Append(message llm.Message) {
	workingSet.messages = append(workingSet.messages, message)
	workingSet.compactIfNeeded()
}

// Messages returns the current history, with any compaction summary prepended
// so dropped context is represented rather than silently lost.
func (workingSet *WorkingSet) Messages() []llm.Message {
	if workingSet.compactionSummary == "" {
		messagesCopy := make([]llm.Message, len(workingSet.messages))
		copy(messagesCopy, workingSet.messages)
		return messagesCopy
	}
	assembled := make([]llm.Message, 0, len(workingSet.messages)+1)
	assembled = append(assembled, llm.Message{
		Role: llm.RoleUser,
		Text: "Summary of earlier work in this phase:\n" + workingSet.compactionSummary,
	})
	return append(assembled, workingSet.messages...)
}

// ApproxTokens estimates the current context size.
func (workingSet *WorkingSet) ApproxTokens() int {
	return llm.EstimateTokens(llm.Request{
		System:   workingSet.SystemPrompt,
		Messages: workingSet.Messages(),
	})
}

// CompactionCount reports how many times the set has been compacted, which
// tests assert on and the report notes as a confidence signal.
func (workingSet *WorkingSet) CompactionCount() int { return workingSet.compactionCount }

func (workingSet *WorkingSet) compactIfNeeded() {
	compactionThreshold := int(float64(workingSet.maxContextTokens) * workingSet.compactAtFraction)
	if workingSet.ApproxTokens() <= compactionThreshold {
		return
	}
	// Keep the most recent turns: they carry the current line of reasoning.
	// Anything older becomes a summary.
	keepCount := len(workingSet.messages) / 3
	if keepCount < 2 {
		keepCount = 2
	}
	if keepCount >= len(workingSet.messages) {
		return
	}
	dropCount := len(workingSet.messages) - keepCount
	droppedMessages := workingSet.messages[:dropCount]

	newSummary := workingSet.Summarise(droppedMessages)
	if workingSet.compactionSummary == "" {
		workingSet.compactionSummary = newSummary
	} else {
		workingSet.compactionSummary = workingSet.compactionSummary + "\n" + newSummary
	}
	workingSet.messages = append([]llm.Message{}, workingSet.messages[dropCount:]...)
	workingSet.compactionCount++
}

func defaultSummarise(droppedMessages []llm.Message) string {
	var summaryBuilder strings.Builder
	toolCallCount := 0
	for _, droppedMessage := range droppedMessages {
		toolCallCount += len(droppedMessage.ToolCalls)
	}
	fmt.Fprintf(&summaryBuilder, "- %d earlier turns dropped, including %d tool calls.",
		len(droppedMessages), toolCallCount)
	for _, droppedMessage := range droppedMessages {
		for _, toolCall := range droppedMessage.ToolCalls {
			fmt.Fprintf(&summaryBuilder, "\n  - called %s", toolCall.ToolName)
		}
	}
	return summaryBuilder.String()
}

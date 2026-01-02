package llm

// EstimateTokens estimates token count using ~4 chars per token approximation.
func EstimateTokens(text string) int {
	if len(text) == 0 {
		return 0
	}
	return (len(text) + 3) / 4
}

// EstimateMessagesTokens estimates total tokens for a slice of messages.
func EstimateMessagesTokens(messages []ChatMessage) int {
	total := 0
	for _, msg := range messages {
		total += EstimateTokens(msg.Content) + 4 // +4 for role/formatting overhead
	}
	return total
}

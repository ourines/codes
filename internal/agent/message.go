package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SendMessage sends a typed message from one agent to another (or broadcast if to is empty).
func SendMessage(teamName, from, to, content string) (*Message, error) {
	return sendTypedMessage(teamName, MsgChat, from, to, content, 0)
}

// SendTaskReport sends a task completion/failure report message.
func SendTaskReport(teamName, from, to string, msgType MessageType, taskID int, content string) (*Message, error) {
	return sendTypedMessage(teamName, msgType, from, to, content, taskID)
}

// BroadcastMessage sends a message to all agents.
func BroadcastMessage(teamName, from, content string) (*Message, error) {
	return sendTypedMessage(teamName, MsgChat, from, "", content, 0)
}

// sendTypedMessage is the internal implementation for all message sends.
func sendTypedMessage(teamName string, msgType MessageType, from, to, content string, taskID int) (*Message, error) {
	dir := messagesDir(teamName)
	if err := ensureDir(dir); err != nil {
		return nil, err
	}

	now := time.Now()
	// Use nanosecond precision + random suffix to avoid ID collisions
	nanoStr := now.Format("20060102T150405.000000000")
	target := to
	if target == "" {
		target = "broadcast"
	}
	id := fmt.Sprintf("%s-%s-%s-%s", nanoStr, from, target, generateID()[:8])

	msg := &Message{
		ID:        id,
		Type:      msgType,
		From:      from,
		To:        to,
		Content:   content,
		TaskID:    taskID,
		Read:      false,
		CreatedAt: now,
	}

	path := filepath.Join(dir, id+".json")
	if err := writeJSON(path, msg); err != nil {
		return nil, fmt.Errorf("write message: %w", err)
	}

	return msg, nil
}

// GetMessages returns messages for a specific agent, optionally only unread.
func GetMessages(teamName, agentName string, unreadOnly bool) ([]*Message, error) {
	dir := messagesDir(teamName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var messages []*Message
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		var msg Message
		path := filepath.Join(dir, e.Name())
		if err := readJSON(path, &msg); err != nil {
			continue
		}

		// Include messages addressed to this agent or broadcast
		if msg.To != agentName && msg.To != "" {
			continue
		}

		if unreadOnly && msg.Read {
			continue
		}

		messages = append(messages, &msg)
	}

	// Sort by creation time
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].CreatedAt.Before(messages[j].CreatedAt)
	})

	return messages, nil
}

// GetMessagesByType returns messages filtered by type.
func GetMessagesByType(teamName, agentName string, msgType MessageType, unreadOnly bool) ([]*Message, error) {
	msgs, err := GetMessages(teamName, agentName, unreadOnly)
	if err != nil {
		return nil, err
	}

	var filtered []*Message
	for _, m := range msgs {
		if m.Type == msgType {
			filtered = append(filtered, m)
		}
	}
	return filtered, nil
}

// MarkRead marks a message as read.
func MarkRead(teamName, messageID string) error {
	dir := messagesDir(teamName)
	path := filepath.Join(dir, messageID+".json")

	var msg Message
	if err := readJSON(path, &msg); err != nil {
		return err
	}

	msg.Read = true
	return writeJSON(path, &msg)
}

// SendTypedMessage sends a message with a specific type and optional task ID.
func SendTypedMessage(teamName string, msgType MessageType, from, to, content string, taskID int) (*Message, error) {
	return sendTypedMessage(teamName, msgType, from, to, content, taskID)
}

// GetAllTeamMessages reads all messages for a team, sorted by time descending, limited to n.
func GetAllTeamMessages(teamName string, limit int) ([]*Message, error) {
	dir := messagesDir(teamName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var messages []*Message
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		var msg Message
		path := filepath.Join(dir, e.Name())
		if err := readJSON(path, &msg); err != nil {
			continue
		}

		messages = append(messages, &msg)
	}

	// Sort by creation time descending (newest first)
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].CreatedAt.After(messages[j].CreatedAt)
	})

	if limit > 0 && len(messages) > limit {
		messages = messages[:limit]
	}

	return messages, nil
}

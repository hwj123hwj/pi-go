package feishu

import (
	"context"
	"fmt"
	"strings"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
)

const (
	worktreeCardActionKey     = "pi_go_action"
	worktreeCardChatKey       = "chat_key"
	worktreeCardCommitMessage = "commit_message"

	worktreeActionStatus  = "worktree_status"
	worktreeActionCommit  = "worktree_commit"
	worktreeActionDiscard = "worktree_discard"
)

// BuildWorktreeCard builds an interactive worktree control card.
func BuildWorktreeCard(chatKey string, route *ChatRoute, content string) map[string]any {
	content = strings.TrimSpace(content)
	if content == "" {
		content = "Worktree 状态不可用"
	}

	elements := []any{
		map[string]any{
			"tag":        "markdown",
			"element_id": "worktree_status",
			"content":    truncateCardText(content),
			"text_align": "left",
			"text_size":  "normal_v2",
		},
	}

	if route != nil && route.WorktreeRoot != "" {
		elements = append(elements,
			map[string]any{
				"tag":         "input",
				"name":        worktreeCardCommitMessage,
				"placeholder": plainText("提交信息，例如 finish task"),
			},
			map[string]any{
				"tag":    "action",
				"layout": "flow",
				"actions": []any{
					worktreeButton("刷新状态", "default", worktreeActionStatus, chatKey, nil),
					worktreeButton("提交并清理", "primary", worktreeActionCommit, chatKey, map[string]any{
						"title": plainText("提交 worktree"),
						"text":  plainText("将当前 worktree 改动提交到隔离分支，并移除 worktree 目录。"),
					}),
					worktreeButton("丢弃", "danger", worktreeActionDiscard, chatKey, map[string]any{
						"title": plainText("丢弃 worktree"),
						"text":  plainText("将删除 worktree 目录和对应分支，未提交改动无法恢复。"),
					}),
				},
			},
		)
	}

	summary := "Worktree"
	if route != nil && route.WorktreeBranch != "" {
		summary = route.WorktreeBranch
	}

	return map[string]any{
		"schema": "2.0",
		"config": map[string]any{
			"streaming_mode": false,
			"summary":        map[string]any{"content": summary},
		},
		"body": map[string]any{
			"elements": elements,
		},
	}
}

func worktreeButton(text, buttonType, action, chatKey string, confirm map[string]any) map[string]any {
	button := map[string]any{
		"tag":  "button",
		"text": plainText(text),
		"type": buttonType,
		"value": map[string]any{
			worktreeCardActionKey: action,
			worktreeCardChatKey:   chatKey,
		},
	}
	if confirm != nil {
		button["confirm"] = confirm
	}
	return button
}

func plainText(content string) map[string]any {
	return map[string]any{
		"tag":     "plain_text",
		"content": content,
	}
}

// SendWorktreeCard sends a CardKit worktree control card.
func (c *Client) SendWorktreeCard(ctx context.Context, chatID, replyTo string, card map[string]any) (string, error) {
	cardID, err := c.createCardKitCard(ctx, card)
	if err != nil {
		return "", fmt.Errorf("create worktree card: %w", err)
	}
	return c.sendCardKitMessage(ctx, chatID, cardID, replyTo)
}

func worktreeCardResponse(content string, route *ChatRoute, chatKey string, toastType string) *callback.CardActionTriggerResponse {
	if toastType == "" {
		toastType = "success"
	}
	return &callback.CardActionTriggerResponse{
		Toast: &callback.Toast{
			Type:    toastType,
			Content: firstLine(content),
		},
		Card: &callback.Card{
			Type: "card_json",
			Data: BuildWorktreeCard(chatKey, route, content),
		},
	}
}

func firstLine(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "Done"
	}
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		return text[:idx]
	}
	return text
}

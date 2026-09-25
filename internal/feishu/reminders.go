package feishu

import (
	"context"
	"fmt"
	"log/slog"
)

func (h *Handler) sendProjectGroupPermissionReminder(ctx context.Context, recipientOpenID, groupName string) {
	if h == nil || h.client == nil || recipientOpenID == "" || h.appID == "" {
		return
	}

	go func() {
		probeCtx := context.Background()
		granted, ok, err := h.client.ProbeGrantedScopes(probeCtx)
		if err != nil {
			slog.Warn("probe feishu scopes failed", "error", err)
			return
		}

		missing := missingScopes(granted, requiredAppScopes)
		hasGroupMsg := ok && hasScope(granted, sensitiveGroupMessageScope)
		if ok && len(missing) == 0 && hasGroupMsg {
			return
		}

		msg := buildProjectGroupPermissionReminder(h.appID, groupName, missing, !hasGroupMsg, !ok)
		if _, err := h.client.SendMessage(probeCtx, recipientOpenID, msg, ""); err != nil {
			slog.Warn("send feishu permission reminder failed", "recipient", recipientOpenID, "error", err)
		}
	}()
}

func buildProjectGroupPermissionReminder(appID, groupName string, missing []string, missingGroupMsg bool, scopeUnknown bool) string {
	msg := fmt.Sprintf("💬 **【重要体验提示 — 飞书项目群权限】**\n\n您刚才成功创建了项目群「%s」。\n\n", groupName)

	if scopeUnknown {
		msg += "ℹ️ 当前应用还不能读取自身已开通 scope 列表（通常缺少 `application:application:self_manage`），无法自动确认权限完整性。\n\n"
	}

	if len(missing) > 0 || scopeUnknown {
		scopes := missing
		if scopeUnknown {
			scopes = requiredAppScopes
		}
		msg += fmt.Sprintf("⚠️ **基础权限可能未完整开通**，缺失时会影响收发消息、卡片更新、文件/图片等功能。\n👉 一键申请基础权限：%s\n\n", buildScopeApplyURL(appID, scopes))
	}

	if missingGroupMsg || scopeUnknown {
		msg += fmt.Sprintf("⚠️ **免 @ 权限未确认**：如果不开通 `%s`，群里普通消息可能不会触发机器人；请在群里 @ 机器人，或申请免 @ 权限。\n👉 一键申请免 @ 权限：%s\n\n", sensitiveGroupMessageScope, buildScopeApplyURL(appID, []string{sensitiveGroupMessageScope}))
	}

	msg += "还需要确认事件订阅：\n"
	msg += fmt.Sprintf("1️⃣ 事件订阅页确认订阅 `im.message.receive_v1`：%s\n", buildEventSubURL(appID))
	msg += fmt.Sprintf("2️⃣ 权限管理页申请发布版本使权限生效：%s\n", buildPermissionPageURL(appID))
	msg += "\n权限生效前，群里请先使用 `@机器人 你的问题`。"
	return msg
}

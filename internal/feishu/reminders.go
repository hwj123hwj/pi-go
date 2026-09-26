package feishu

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// BuildStartupWelcome creates the private message sent to the setup owner when
// the Feishu WebSocket connection becomes ready.
func BuildStartupWelcome(appID, workspace string, grantedScopes []string, scopesKnown bool) string {
	lines := []string{
		"👋 **Pi-go 飞书 Bot 已连接，随时待命。**",
		"",
		"**💡 快速开始**",
		"- 私聊直接发送任务即可。",
		"- 在私聊中发送 `/project create <项目路径> <群名称>` 创建项目协作群。",
		"- 发送 `/help` 查看可用命令。",
		"",
		"**📂 默认工作目录**",
	}
	if workspace != "" {
		lines = append(lines, "`"+workspace+"`")
	} else {
		lines = append(lines, "跟随 pi-agent 服务端工作目录；可设置 `PI_GO_WORKSPACE` 指定目录。")
	}

	if scopesKnown {
		missing := missingScopes(grantedScopes, requiredAppScopes)
		missingGroupMsg := !hasScope(grantedScopes, sensitiveGroupMessageScope)
		if len(missing) == 0 && !missingGroupMsg {
			lines = append(lines, "", "✅ **应用权限完整**，私聊、群聊、卡片、文件和群管理能力均已授权。")
		} else {
			if len(missing) > 0 {
				lines = append(lines,
					"",
					fmt.Sprintf("⚠️ **缺少 %d 项基础权限**，部分接收、回复、卡片或文件能力会受限：", len(missing)),
				)
				for _, scope := range missing {
					lines = append(lines, "- `"+scope+"`")
				}
				lines = append(lines, "👉 一键申请："+buildScopeApplyURL(appID, missing))
			}
			if missingGroupMsg {
				lines = append(lines,
					"",
					"💬 群消息默认需要 @机器人。若希望群内普通消息也能触发，可申请敏感权限 `"+sensitiveGroupMessageScope+"`：",
					"👉 "+buildScopeApplyURL(appID, []string{sensitiveGroupMessageScope}),
				)
			}
			lines = append(lines, "🔄 权限申请后需发布应用版本使其生效：", "👉 "+buildPermissionPageURL(appID))
		}
	} else {
		lines = append(lines,
			"",
			"ℹ️ 暂时无法读取应用权限清单。请确认已开通 `application:application:self_manage`，并检查权限和事件订阅配置。",
			"👉 基础权限申请："+buildScopeApplyURL(appID, requiredAppScopes),
			"👉 权限管理："+buildPermissionPageURL(appID),
		)
	}

	lines = append(lines,
		"",
		"**🔌 事件接收检查**",
		"确认已启用长连接并订阅 `im.message.receive_v1`：",
		"👉 "+buildEventSubURL(appID),
	)
	return strings.Join(lines, "\n")
}

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

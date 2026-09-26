package feishu

import (
	"context"
	"log/slog"
	"time"
)

// ConfigureStartupWelcome sends the setup guide and a permission check to the
// configured owner after the WebSocket connection is ready.
func ConfigureStartupWelcome(gateway *Gateway, appID, ownerOpenID, workspace string, client *Client) {
	gateway.SetOnReady(func() {
		if ownerOpenID == "" {
			slog.Info("skipping feishu startup welcome: owner open_id is not configured")
			return
		}

		go func() {
			probeCtx, cancelProbe := context.WithTimeout(context.Background(), 8*time.Second)
			scopes, scopesKnown, err := client.ProbeGrantedScopes(probeCtx)
			cancelProbe()
			if err != nil {
				slog.Warn("feishu startup permission check failed", "error", err)
				scopesKnown = false
			}

			welcome := BuildStartupWelcome(appID, workspace, scopes, scopesKnown)
			sendCtx, cancelSend := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancelSend()
			if _, err := client.SendMarkdown(sendCtx, ownerOpenID, welcome, ""); err != nil {
				slog.Warn("failed to send feishu startup welcome", "error", err)
			}
		}()
	})
}

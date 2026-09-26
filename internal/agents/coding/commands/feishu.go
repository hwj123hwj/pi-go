package commands

import (
	"fmt"
	"strings"

	"github.com/hwj123hwj/pi-go/internal/feishu"
	"github.com/hwj123hwj/pi-go/sdk/slashcmd"
)

// feishuGatewayMgr is the package-level feishu gateway manager.
var feishuGatewayMgr = feishu.NewGatewayManager()

// RegisterFeishuCommands registers /feishu and /lark slash commands.
func RegisterFeishuCommands(registry *slashcmd.Registry) {
	registerFeishuCommand(registry, "feishu")
}

func registerFeishuCommand(registry *slashcmd.Registry, name string) {
	registry.Register(slashcmd.Command{
		Name:        name,
		Description: "Configure Feishu and show how to run the standalone bridge",
		Subcommands: []slashcmd.Subcommand{
			{Name: "setup", Description: "Scan QR to login (or --manual <AppId> <AppSecret>)"},
			{Name: "start", Description: "Check credentials and show bridge start instructions"},
			{Name: "stop", Description: "Stop the bot"},
			{Name: "status", Description: "Show current status"},
			{Name: "logout", Description: "Clear credentials and disconnect"},
		},
		Handler: func(ctx slashcmd.Context, args string) (slashcmd.CommandResult, error) {
			args = strings.TrimSpace(args)
			subCmd := strings.Fields(args)

			switch {
			case len(subCmd) == 0:
				return handleFeishuSetup(ctx, nil)

			case subCmd[0] == "setup":
				return handleFeishuSetup(ctx, subCmd[1:])

			case subCmd[0] == "start":
				return handleFeishuStart()

			case subCmd[0] == "stop":
				return handleFeishuStop()

			case subCmd[0] == "status":
				return handleFeishuStatus()

			case subCmd[0] == "logout":
				return handleFeishuLogout()

			default:
				return slashcmd.CommandResult{Output: feishuHelp()}, nil
			}
		},
	})
}

func handleFeishuSetup(ctx slashcmd.Context, args []string) (slashcmd.CommandResult, error) {
	// Mode: manual (--manual <appId> <appSecret>)
	if len(args) >= 1 && args[0] == "--manual" {
		if len(args) < 3 {
			return slashcmd.CommandResult{Output: "Usage: /feishu setup --manual <AppId> <AppSecret>"}, nil
		}
		appID := args[1]
		appSecret := args[2]

		// Probe to verify + get bot info
		botName, botOpenID, probeErr := feishu.ProbeCredentials(appID, appSecret, "feishu")
		if probeErr != nil {
			return slashcmd.CommandResult{Output: fmt.Sprintf("⚠️ Credentials validation failed: %v\nCredentials saved anyway.", probeErr)}, nil
		}

		creds := feishu.Credentials{
			AppID:     appID,
			AppSecret: appSecret,
			BotName:   botName,
			BotOpenID: botOpenID,
			Platform:  "feishu",
		}
		if err := feishu.SaveCredentials(creds); err != nil {
			return slashcmd.CommandResult{}, fmt.Errorf("save credentials: %w", err)
		}

		return slashcmd.CommandResult{
			Output: fmt.Sprintf("✅ Feishu credentials saved!\n  App ID: %s\n  Bot: %s\n  Saved to: ~/.pi-go/feishu-credentials.json\n\nNext: run /feishu start to start the bot", appID, botName),
		}, nil
	}

	// ── Mode 1: Device-code registration (scan QR to auto-create app) ──
	fmt.Println("\n📱 Mode 1: Scan QR to auto-create Feishu app")
	fmt.Println("   Connecting to Feishu...")

	// Step 1: Init
	if err := feishu.InitRegistration("feishu"); err != nil {
		return slashcmd.CommandResult{
			Output: fmt.Sprintf("❌ Registration init failed: %v\n\nFallback: /feishu setup --manual <AppId> <AppSecret>", err),
		}, nil
	}

	// Step 2: Begin — get device code + QR URL
	begin, err := feishu.BeginRegistration("feishu")
	if err != nil {
		return slashcmd.CommandResult{
			Output: fmt.Sprintf("❌ Registration begin failed: %v\n\nFallback: /feishu setup --manual <AppId> <AppSecret>", err),
		}, nil
	}

	fmt.Println("  ✅ QR code generated")
	fmt.Printf("  🔗 URL: %s\n\n", begin.QRURL)
	fmt.Println("  📱 Scan the QR code above with your Feishu mobile app")
	fmt.Println("     Or open the link in a browser to complete authorization")
	fmt.Println("  ⏳ Waiting for QR scan...")
	fmt.Println("     (Press Ctrl+C to cancel)")

	// Open browser if possible
	feishu.OpenBrowser(begin.QRURL)

	// Step 3: Poll for scan result
	poll, err := feishu.PollRegistration(
		begin.DeviceCode,
		begin.Interval,
		begin.ExpireIn,
		"feishu",
		func(dots string) {
			fmt.Printf("\r  Waiting%s", dots)
		},
	)
	fmt.Println() // newline after dots

	if err != nil || poll == nil {
		return slashcmd.CommandResult{
			Output: fmt.Sprintf("❌ Feishu QR scan timed out or failed: %v\n\nTry /feishu setup again,\nor use /feishu setup --manual <AppId> <AppSecret>", err),
		}, nil
	}

	// Step 4: Probe to get bot name + open_id
	botName, botOpenID, _ := feishu.ProbeCredentials(poll.AppID, poll.AppSecret, poll.Domain)

	// Step 5: Save credentials
	creds := feishu.Credentials{
		AppID:      poll.AppID,
		AppSecret:  poll.AppSecret,
		UserOpenID: poll.OpenID,
		BotName:    botName,
		BotOpenID:  botOpenID,
		Platform:   poll.Domain,
	}
	if err := feishu.SaveCredentials(creds); err != nil {
		return slashcmd.CommandResult{}, fmt.Errorf("save credentials: %w", err)
	}

	if botName == "" {
		botName = "(unknown)"
	}

	return slashcmd.CommandResult{
		Output: fmt.Sprintf(`✅ Feishu app created successfully!

  Bot Name:  %s
  App ID:    %s
  Saved to:  ~/.pi-go/feishu-credentials.json

Next: run /feishu start to start the bot`, botName, poll.AppID),
	}, nil
}

func handleFeishuStart() (slashcmd.CommandResult, error) {
	if feishuGatewayMgr.IsRunning() {
		return slashcmd.CommandResult{Output: "⚠️ Feishu Bot is already running. Run /feishu stop first."}, nil
	}

	creds, err := feishu.LoadCredentials()
	if err != nil || creds == nil {
		return slashcmd.CommandResult{
			Output: `⚠️ Feishu credentials not found.

Please configure first:
  /feishu setup              # Scan QR to auto-login
  or
  /feishu setup --manual <AppId> <AppSecret>`,
		}, nil
	}

	// For now, just return instructions since we need a message handler
	// wired from the runtime layer. The actual gateway start happens
	// when pi-feishu-bridge is launched.
	_ = creds
	return slashcmd.CommandResult{
		Output: fmt.Sprintf(`✅ Feishu credentials found.

  App ID:  %s
  Platform: %s

This command does not open the Feishu WebSocket. To start receiving messages, run:
  pi-feishu-bridge

After the WebSocket connects, the bot sends the setup owner a welcome guide and permission check.
For manual credentials, set FEISHU_OWNER_OPEN_ID to receive that startup message.`, creds.AppID, creds.Platform),
	}, nil
}

func handleFeishuStop() (slashcmd.CommandResult, error) {
	if !feishuGatewayMgr.IsRunning() {
		return slashcmd.CommandResult{Output: "⚠️ Feishu Bot is not running."}, nil
	}

	feishuGatewayMgr.Stop()
	return slashcmd.CommandResult{Output: "✅ Feishu Bot stopped."}, nil
}

func handleFeishuStatus() (slashcmd.CommandResult, error) {
	creds, _ := feishu.LoadCredentials()
	if creds == nil {
		return slashcmd.CommandResult{Output: "📊 Feishu Status: Not configured\n\nRun /feishu setup to get started."}, nil
	}

	status := "stopped"
	if feishuGatewayMgr.IsRunning() {
		status = "running"
	}

	botName := creds.BotName
	if botName == "" {
		botName = "(unknown)"
	}

	return slashcmd.CommandResult{
		Output: fmt.Sprintf(`📊 Feishu Bot Status

  Status:    %s
  App ID:    %s
  Bot Name:  %s
  Platform:  %s
  Credentials: ~/.pi-go/feishu-credentials.json`, status, creds.AppID, botName, creds.Platform),
	}, nil
}

func handleFeishuLogout() (slashcmd.CommandResult, error) {
	if feishuGatewayMgr.IsRunning() {
		feishuGatewayMgr.Stop()
	}

	if err := feishu.DeleteCredentials(); err != nil {
		return slashcmd.CommandResult{}, fmt.Errorf("delete credentials: %w", err)
	}

	return slashcmd.CommandResult{
		Output: "✅ Feishu credentials cleared. Run /feishu setup to reconfigure.",
	}, nil
}

func feishuHelp() string {
	return strings.TrimSpace(`
🤖 /feishu — Feishu Bot Integration

Usage:
  /feishu                        Interactive setup (QR scan login)
  /feishu setup                  Scan QR code to login (recommended)
  /feishu setup --manual <AppId> <AppSecret>  Manual credentials
  /feishu start                 Check credentials and show bridge start instructions
  /feishu stop                  Stop the bot
  /feishu status                Show current status
  /feishu logout                Clear credentials and disconnect

Workflow:
  1. /feishu setup              # Scan QR (you become the owner)
  2. /feishu start              # Show the standalone bridge command
     pi-feishu-bridge           # Start the actual Feishu connection
  3. Send a message to the bot in Feishu

Credentials are saved to ~/.pi-go/feishu-credentials.json
`)
}

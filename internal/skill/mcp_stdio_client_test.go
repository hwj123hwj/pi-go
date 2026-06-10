package skill

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStdioMCPSkillClient_LoadsResourceListAndContents(t *testing.T) {
	client := NewStdioMCPSkillClient(os.Args[0], "-test.run=TestMCPStdioHelperProcess")
	client.Env = []string{"PI_GO_MCP_STDIO_HELPER=1"}

	resources, err := client.ListSkillResources(context.Background())
	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, "mcp://deck-server/skills/deck/SKILL.md", resources[0].URI)
	assert.Equal(t, "deck", resources[0].Name)

	data, err := client.ReadSkillResource(context.Background(), resources[0].URI)
	require.NoError(t, err)
	assert.Contains(t, string(data), "name: deck")
	assert.Contains(t, string(data), "Build decks from MCP")
}

func TestStdioMCPSkillClient_ReportsRPCError(t *testing.T) {
	client := NewStdioMCPSkillClient(os.Args[0], "-test.run=TestMCPStdioHelperProcess")
	client.Env = []string{"PI_GO_MCP_STDIO_HELPER=1"}

	_, err := client.ReadSkillResource(context.Background(), "mcp://deck-server/missing/SKILL.md")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStdioMCPSkillSourceProvider_LoadsSkills(t *testing.T) {
	client := NewStdioMCPSkillClient(os.Args[0], "-test.run=TestMCPStdioHelperProcess")
	client.Env = []string{"PI_GO_MCP_STDIO_HELPER=1"}

	result := NewMCPSkillSourceProvider("deck-server", client).LoadSkills(context.Background())
	require.Empty(t, result.Diagnostics)
	require.Len(t, result.Skills, 1)
	assert.Equal(t, "deck", result.Skills[0].Name)
	assert.Equal(t, "mcp://deck-server/skills/deck/SKILL.md", result.Skills[0].FilePath)
	assert.Equal(t, "mcp://deck-server/skills/deck", result.Skills[0].BaseDir)
	assert.Equal(t, SourcePlugin, result.Skills[0].Source)
}

func TestMCPStdioHelperProcess(t *testing.T) {
	if os.Getenv("PI_GO_MCP_STDIO_HELPER") != "1" {
		return
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		data, err := readMCPMessage(reader, defaultMCPMaxMessageBytes)
		if err != nil {
			os.Exit(0)
		}
		var req struct {
			ID     json.RawMessage `json:"id,omitempty"`
			Method string          `json:"method"`
			Params struct {
				URI string `json:"uri"`
			} `json:"params"`
		}
		if err := json.Unmarshal(data, &req); err != nil {
			os.Exit(2)
		}
		if len(req.ID) == 0 {
			continue
		}
		switch req.Method {
		case "initialize":
			writeHelperResponse(req.ID, map[string]any{
				"protocolVersion": defaultMCPProtocolVersion,
				"capabilities":    map[string]any{"resources": map[string]any{}},
				"serverInfo":      map[string]string{"name": "test-mcp", "version": "0"},
			})
		case "resources/list":
			writeHelperResponse(req.ID, map[string]any{
				"resources": []map[string]string{{
					"uri":  "mcp://deck-server/skills/deck/SKILL.md",
					"name": "deck",
				}},
			})
		case "resources/read":
			if req.Params.URI != "mcp://deck-server/skills/deck/SKILL.md" {
				writeHelperError(req.ID, -32004, "not found")
				continue
			}
			writeHelperResponse(req.ID, map[string]any{
				"contents": []map[string]string{{
					"uri":      req.Params.URI,
					"mimeType": "text/markdown",
					"text":     "---\nname: deck\ndescription: Build decks from MCP\n---\nUse this deck workflow.",
				}},
			})
		default:
			writeHelperError(req.ID, -32601, "method not found")
		}
	}
}

func writeHelperResponse(id json.RawMessage, result any) {
	_ = writeMCPMessage(os.Stdout, map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(id),
		"result":  result,
	})
}

func writeHelperError(id json.RawMessage, code int, message string) {
	_ = writeMCPMessage(os.Stdout, map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(id),
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}

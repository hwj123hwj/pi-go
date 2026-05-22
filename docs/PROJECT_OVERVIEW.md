# Pi-Go Desktop - Comprehensive Project Overview

## 📁 Project Structure

```
pi-go/
├── cmd/pi-agent/          # Go backend CLI entry point
├── internal/              # Go backend internal packages (74 Go files, ~10,784 lines)
│   ├── agent/            # Agent loop and orchestration
│   ├── ai/               # AI provider interfaces (Anthropic, OpenAI, Mock, DeepV)
│   ├── app/              # Application assembly layer
│   ├── config/           # Configuration management
│   ├── extensions/       # Extension system
│   ├── mode/             # Operation modes (run, chat, serve)
│   ├── prompt/           # System prompt construction
│   ├── runtime/          # Session registry and agent sessions
│   ├── server/           # HTTP + WebSocket server
│   ├── session/          # Session storage (JSONL)
│   ├── sessionmgr/       # Session manager
│   ├── skill/            # Skill system
│   ├── slashcmd/         # Slash command registry
│   ├── tools/            # 9 built-in tools (bash, read, write, edit, grep, find, ls, truncate, prompt_info)
│   ├── compaction/       # Context compaction
│   └── util/             # Utilities (git, shell)
├── desktop/              # Electron + React desktop client
│   ├── electron/         # Electron main process
│   ├── src/              # React frontend
│   │   ├── components/   # UI components
│   │   ├── stores/       # Zustand state management
│   │   ├── services/     # API and WebSocket clients
│   │   └── styles/       # CSS modules and variables
│   ├── package.json      # Node.js dependencies
│   └── vite.config.ts    # Vite configuration
├── docs/                 # Documentation
├── learning/             # Learning materials
└── go.mod                # Go module dependencies
```

## 📦 Package.json (Desktop)

### Dependencies
- **react**: ^19.0.0
- **react-dom**: ^19.0.0  
- **zustand**: ^5.0.0 (State management)
- **react-markdown**: ^9.0.0 (Markdown rendering)
- **rehype-highlight**: ^7.0.0 (Syntax highlighting)
- **highlight.js**: ^11.11.0 (Code highlighting)

### Scripts
- `dev` - Start Vite dev server
- `build` - Build for production
- `electron:dev` - Start Electron with dev tools
- `electron:build` - Build Electron app
- `electron:build:arm64` - Build for Apple Silicon
- `electron:build:x64` - Build for Intel

### Dev Dependencies
- electron ^33.0.0
- electron-builder ^25.1.0
- vite ^6.0.0
- typescript ^5.6.0
- concurrently ^9.1.0
- wait-on ^8.0.0

## 🏗️ React Component Hierarchy

```
App (Root)
└── AppLayout (Main layout)
    ├── Sidebar (Left panel)
    │   ├── Header (Logo + Status)
    │   ├── NewSessionButton
    │   ├── SessionList
    │   └── ModelSelector
    └── ChatPanel (Main content)
        ├── MessageList
        │   ├── UserMessage
        │   └── AssistantMessage
        │       ├── StreamingContent
        │       ├── MarkdownRenderer
        │       └── ToolCallBlock
        └── InputArea (Send/Stop controls)
```

## 🎨 CSS Structure

### CSS Variables (variables.css)
**Theme: 青夜**
- Backgrounds: `--bg-primary`, `--bg-secondary`, `--bg-tertiary`, `--bg-surface`, `--bg-hover`
- Text: `--text-primary`, `--text-secondary`, `--text-tertiary`, `--text-inverse`
- Accents: `--accent-primary`, `--accent-secondary`, `--accent-danger`
- Borders: `--border-primary`, `--border-secondary`
- Links: `--link`, `--link-hover`
- Spacing: `--space-xs` to `--space-xl`
- Border Radius: `--radius-sm`, `--radius-md`, `--radius-lg`, `--radius-full`
- Animations: `--transition-fast`, `--transition-normal`, `--transition-slow`

### CSS Modules
- `global.css` - Global styles, scrollbar, selection
- `layout.module.css` - Main layout
- `sidebar.module.css` - Sidebar components
- `chat.module.css` - Chat messages and tool calls
- `input.module.css` - Input area
- `common.module.css` - Shared components

## 🗄️ State Management (Zustand Stores)

### connectionStore.ts
- `connected` - WebSocket connection status
- `serverUrl` - Server URL
- `connect(url)` - Connect to server
- `disconnect()` - Disconnect

### sessionStore.ts
- `sessions[]` - Array of sessions
- `currentSessionId` - Active session
- `loading` - Loading state
- `loadSessions()` - Load from backend
- `createSession()` - Create new session
- `switchSession(id)` - Switch active session
- `deleteSession(id)` - Delete session

### modelStore.ts
- `models[]` - Available models
- `currentModel` - Currently selected model
- `loading` - Loading state
- `loadModels()` - Load models from backend
- `switchModel(sessionId, modelId)` - Switch model for session

### chatStore.ts
- `messagesBySession` - Map of session → messages
- `streamingSessionId` - Currently streaming session
- `error` - Error state
- `loadHistory(sessionId)` - Load message history
- `sendPrompt(sessionId, prompt)` - Send message via WebSocket
- `cancelGeneration()` - Cancel current generation
- `setupWSListeners()` - Setup WebSocket event handlers

## ⚡ Electron Main Process

### main.ts
- BrowserWindow configuration (1200x800)
- IPC handlers for server management
- Lifecycle management (ready, activate, quit)

### preload.ts
- Secure IPC bridge using contextBridge
- Exposes `getServerUrl()` and `startServer()` to renderer

### pi-go-manager.ts
- Manages pi-go backend process lifecycle
- Auto-finds free port
- Health check polling
- Environment setup (.env management)
- Binary location handling (dev vs packaged)

## 🌐 Go Backend Structure

### Main Entry (cmd/pi-agent/main.go)
- Mode flags: `run`, `chat`, `serve`
- Session management
- Skill directory support

### Server (internal/server/)
- HTTP REST API + WebSocket
- Routes: `/health`, `/chat`, `/sessions`, `/tools`, `/models`
- WebSocket event streaming (text_delta, tool_start, tool_end, done, error)

### Agent (internal/agent/)
- Double-loop architecture (follow-up + tool calls)
- Sequential/parallel tool execution
- Event streaming

### AI Providers (internal/ai/providers/)
- Anthropic (Claude)
- OpenAI (GPT)
- DeepV (custom)
- Mock (testing)

### Tools (internal/tools/)
9 built-in tools:
- **bash** - Execute shell commands
- **read** - Read files
- **write** - Write files
- **edit** - Edit files with sed
- **grep** - Search text
- **find** - Find files
- **ls** - List directory contents
- **truncate** - Truncate files
- **prompt_info** - Show prompt information

## ✅ Existing Features

### Session Management
- Create, switch, delete sessions
- Session history persistence
- Auto-load on startup
- Session metadata (message count, last active)

### WebSocket Streaming
- Real-time message streaming
- Auto-reconnect with exponential backoff
- Event types: text_delta, tool_start, tool_end, turn_end, done, error
- Cancel generation support

### Model Switching
- Model dropdown selector
- Per-session model configuration
- Provider-level model management
- Session info sync on switch

### Tool Call Visualization
- Tool call blocks with status (running/done/error)
- Tool icons (bash⌨, read📖, write✏️, edit📝, grep🔍, find📂, ls📋)
- Result truncation (2000 chars max)
- Spinner animation for running tools
- Status indicators (✓ done, ✗ error)

### Connection Status
- Online/offline indicator
- WebSocket health monitoring
- Reconnection attempts with visual feedback

### Dark Theme (青夜)
- Deep teal-blue (#3B4A54) base
- Warm rose accents (#C99AAF)
- Smooth transitions
- Custom scrollbar styling
- Selection highlighting

## 🎯 UI Components Built

### Buttons
- **NewSessionButton** - Create new session
- **SendButton** - Send message (↑)
- **StopButton** - Cancel generation (■)
- **DeleteButton** - Delete session (×)

### Inputs
- **InputArea** - Multi-line textarea with auto-height
- **ModelSelector** - Model dropdown

### Message Components
- **UserMessage** - User message bubble
- **AssistantMessage** - AI message with markdown and tools
- **MessageList** - Scrollable message container
- **StreamingContent** - Streaming text with cursor
- **MarkdownRenderer** - Markdown with code highlighting
- **ToolCallBlock** - Tool call visualization

### Common Components
- **LoadingIndicator** - Loading spinner
- **ErrorBanner** - Error display

### Layout Components
- **AppLayout** - Main application layout
- **Sidebar** - Left panel with logo, sessions, model selector
- **ChatPanel** - Main chat area

## 🚀 Configuration

### Environment Variables
- `PI_GO_PROVIDER` - LLM provider (deepv, anthropic, openai)
- `ANTHROPIC_API_KEY`, `OPENAI_API_KEY` - API keys
- `PI_GO_HOST`, `PI_GO_PORT` - Server address
- `PI_GO_ENABLE_BASH` - Enable bash tool
- `PI_GO_SESSION_FILE` - Session storage path

### TypeScript Configuration
- Target: ES2020
- JSX: react-jsx
- Strict mode enabled
- Module resolution: bundler

### Vite Configuration
- React plugin
- Port: 5173
- Base: './'
- Output: dist/renderer

## 📚 Documentation

### Main README
- Architecture diagram
- Quick start guide
- HTTP API documentation
- Environment variables reference
- Testing instructions

### Code Statistics
- 49 Go files
- ~7,400 lines of Go code
- Minimal dependencies (only testify for testing)
- Full TypeScript coverage for frontend

## 🔌 WebSocket Protocol

### Client → Server
```json
{
  "type": "prompt|cancel|switch_model|ping",
  "session_id": "string",
  "prompt": "string",  // for type:prompt
  "model": "string",   // for type:switch_model
  "provider": "string" // for type:switch_model
}
```

### Server → Client
```json
{
  "type": "event|session_id|status|model_info|error|pong",
  "session_id": "string",
  "event": {
    "type": "text_delta|tool_start|tool_end|turn_end|done|error",
    ...
  },
  "streaming": true,
  "provider": "string",
  "model": "string",
  "message": "string"
}
```

## 🎨 Design System

### Colors
- Primary background: `#3B4A54` (deep teal)
- Secondary background: `#34424B`
- Accent primary: `#C99AAF` (warm rose)
- Accent secondary: `#8CC790` (green)
- Accent danger: `#C77070` (red)

### Typography
- System fonts: SF Pro, Segoe UI, Roboto
- Monospace: SF Mono, Fira Code
- Base size: 14px
- Line height: 1.5-1.7

### Spacing
- xs: 4px, sm: 8px, md: 16px, lg: 24px, xl: 32px

### Border Radius
- sm: 6px, md: 10px, lg: 16px, full: 9999px

---

**Last Updated**: 2026-05-22
**Project Version**: 0.1.0

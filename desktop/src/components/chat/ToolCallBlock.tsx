// ToolCallBlock.tsx — Displays a tool call with its result.
import { ToolCall } from '../../stores/chatStore';
import styles from '../../styles/chat.module.css';

interface Props {
  toolCall: ToolCall;
}

export function ToolCallBlock({ toolCall }: Props) {
  const isRunning = toolCall.status === 'running';
  const icon = getToolIcon(toolCall.name);

  return (
    <div className={`${styles.toolCall} ${styles[`tool_${toolCall.status}`]}`}>
      <div className={styles.toolCallHeader}>
        <span className={styles.toolIcon}>{icon}</span>
        <span className={styles.toolName}>{toolCall.name}</span>
        {isRunning && <span className={styles.toolSpinner} />}
        {!isRunning && toolCall.status === 'done' && <span className={styles.toolCheck}>✓</span>}
        {!isRunning && toolCall.status === 'error' && <span className={styles.toolError}>✗</span>}
      </div>
      {toolCall.result && (
        <pre className={styles.toolResult}>
          <code>{truncateResult(toolCall.result, 2000)}</code>
        </pre>
      )}
    </div>
  );
}

function getToolIcon(name: string): string {
  const icons: Record<string, string> = {
    bash: '⌨',
    read: '📖',
    write: '✏️',
    edit: '📝',
    grep: '🔍',
    find: '📂',
    ls: '📋',
  };
  return icons[name] || '🔧';
}

function truncateResult(text: string, maxLen: number): string {
  if (text.length <= maxLen) return text;
  return text.slice(0, maxLen) + '\n... (truncated)';
}

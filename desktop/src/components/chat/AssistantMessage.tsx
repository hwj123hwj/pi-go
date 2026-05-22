// AssistantMessage.tsx — AI assistant message with streaming, markdown, and tool calls.
import { Message, ToolCall } from '../../stores/chatStore';
import { ToolCallBlock } from './ToolCallBlock';
import { StreamingContent } from './StreamingContent';
import { MarkdownRenderer } from './MarkdownRenderer';
import styles from '../../styles/chat.module.css';

interface Props {
  message: Message;
}

export function AssistantMessage({ message }: Props) {
  const hasToolCalls = message.toolCalls.length > 0;
  const hasText = message.text.length > 0;

  return (
    <div className={styles.assistantMessage}>
      <div className={styles.assistantAvatar}>Pi</div>
      <div className={styles.assistantContent}>
        {hasText && (
          message.streaming ? (
            <StreamingContent text={message.text} />
          ) : (
            <MarkdownRenderer content={message.text} />
          )
        )}
        {hasToolCalls && (
          <div className={styles.toolCallsContainer}>
            {message.toolCalls.map((tc) => (
              <ToolCallBlock key={tc.id} toolCall={tc} />
            ))}
          </div>
        )}
        {message.streaming && !hasText && !hasToolCalls && (
          <div className={styles.typingIndicator}>
            <span></span><span></span><span></span>
          </div>
        )}
      </div>
    </div>
  );
}

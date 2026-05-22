// InputArea.tsx — Message input with send/stop buttons.
import { useState, useRef, KeyboardEvent } from 'react';
import styles from '../../styles/input.module.css';

interface Props {
  onSend: (text: string) => void;
  onCancel: () => void;
  isStreaming: boolean;
}

export function InputArea({ onSend, onCancel, isStreaming }: Props) {
  const [text, setText] = useState('');
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const handleSend = () => {
    const trimmed = text.trim();
    if (!trimmed || isStreaming) return;
    onSend(trimmed);
    setText('');
    // Reset textarea height
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
    }
  };

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const handleInput = () => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
      textareaRef.current.style.height = Math.min(textareaRef.current.scrollHeight, 200) + 'px';
    }
  };

  return (
    <div className={styles.inputArea}>
      <div className={styles.inputContainer}>
        <textarea
          ref={textareaRef}
          className={styles.textarea}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={handleKeyDown}
          onInput={handleInput}
          placeholder="Type your message..."
          rows={1}
          disabled={isStreaming}
        />
        {isStreaming ? (
          <button className={styles.stopButton} onClick={onCancel} title="Stop generation">
            ■
          </button>
        ) : (
          <button
            className={styles.sendButton}
            onClick={handleSend}
            disabled={!text.trim()}
            title="Send (Enter)"
          >
            ↑
          </button>
        )}
      </div>
    </div>
  );
}

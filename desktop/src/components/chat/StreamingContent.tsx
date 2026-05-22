// StreamingContent.tsx — Live streaming text with cursor.
import { MarkdownRenderer } from './MarkdownRenderer';
import styles from '../../styles/chat.module.css';

interface Props {
  text: string;
}

export function StreamingContent({ text }: Props) {
  if (!text) return null;

  return (
    <div className={styles.streamingContent}>
      <MarkdownRenderer content={text} />
      <span className={styles.cursor} />
    </div>
  );
}

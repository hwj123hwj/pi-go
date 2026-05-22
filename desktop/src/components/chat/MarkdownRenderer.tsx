// MarkdownRenderer.tsx — Renders markdown content with code highlighting.
import ReactMarkdown from 'react-markdown';
import rehypeHighlight from 'rehype-highlight';
import 'highlight.js/styles/github-dark.css';
import styles from '../../styles/chat.module.css';

interface Props {
  content: string;
}

export function MarkdownRenderer({ content }: Props) {
  return (
    <div className={styles.markdownBody}>
      <ReactMarkdown rehypePlugins={[rehypeHighlight]}>
        {content}
      </ReactMarkdown>
    </div>
  );
}

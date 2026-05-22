// UserMessage.tsx — User message bubble.
import { Message } from '../../stores/chatStore';
import styles from '../../styles/chat.module.css';

interface Props {
  message: Message;
}

export function UserMessage({ message }: Props) {
  return (
    <div className={styles.userMessage}>
      <div className={styles.userBubble}>
        <div className={styles.messageText}>{message.text}</div>
      </div>
      <div className={styles.userAvatar}>You</div>
    </div>
  );
}

// LoadingIndicator.tsx — Loading spinner.
import styles from '../../styles/common.module.css';

interface Props {
  text?: string;
}

export function LoadingIndicator({ text = 'Loading...' }: Props) {
  return (
    <div className={styles.loading}>
      <div className={styles.spinner} />
      <div>{text}</div>
    </div>
  );
}

// ErrorBanner.tsx — Full-screen error display.
import styles from '../../styles/common.module.css';

interface Props {
  message: string;
}

export function ErrorBanner({ message }: Props) {
  return (
    <div className={styles.errorBanner}>
      <div className={styles.errorIcon}>⚠️</div>
      <div className={styles.errorText}>{message}</div>
    </div>
  );
}

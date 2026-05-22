// UpdateNotification.tsx — Banner showing available app update.
import { useUpdateStore } from '../../stores/updateStore';
import styles from '../../styles/update-notification.module.css';

export function UpdateNotification() {
  const updateInfo = useUpdateStore((s) => s.updateInfo);
  const dismissed = useUpdateStore((s) => s.dismissed);
  const dismissUpdate = useUpdateStore((s) => s.dismissUpdate);

  if (!updateInfo || dismissed) {
    return null;
  }

  const handleDownload = () => {
    if (window.piAPI?.openDownloadPage) {
      window.piAPI.openDownloadPage(updateInfo.downloadUrl);
    }
  };

  return (
    <div className={styles.banner}>
      <div className={styles.content}>
        <span className={styles.icon}>🚀</span>
        <span>
          发现新版本 <span className={styles.version}>v{updateInfo.version}</span>
        </span>
      </div>
      <div className={styles.actions}>
        <button className={styles.downloadBtn} onClick={handleDownload}>
          下载更新
        </button>
        <button className={styles.laterBtn} onClick={dismissUpdate}>
          稍后
        </button>
      </div>
    </div>
  );
}

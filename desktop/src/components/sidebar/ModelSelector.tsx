// ModelSelector.tsx — Dropdown to switch between LLM models.
import { useModelStore, Model } from '../../stores/modelStore';
import { useSessionStore } from '../../stores/sessionStore';
import styles from '../../styles/sidebar.module.css';

export function ModelSelector() {
  const models = useModelStore((s) => s.models);
  const currentModel = useModelStore((s) => s.currentModel);
  const switchModel = useModelStore((s) => s.switchModel);
  const currentSessionId = useSessionStore((s) => s.currentSessionId);

  const handleChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const modelId = e.target.value;
    if (currentSessionId) {
      switchModel(currentSessionId, modelId);
    }
  };

  return (
    <div className={styles.modelSelector}>
      <label className={styles.modelLabel}>Model</label>
      <select
        className={styles.modelSelect}
        value={currentModel?.model || ''}
        onChange={handleChange}
      >
        {models.map((model) => (
          <option key={model.id} value={model.id}>
            {model.name}
          </option>
        ))}
      </select>
    </div>
  );
}

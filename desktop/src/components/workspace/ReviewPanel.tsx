/**
 * ReviewPanel.tsx — Diff review panel for the right sidebar.
 * Reuses the existing DiffPane against the active session's git diff.
 */

import { useStore } from '../../store';
import { DiffPane } from '../panes/DiffPane';
import { useT } from '../../i18n/useT';

export function ReviewPanel() {
  const activeId = useStore((s) => s.activeSessionId);
  const view = useStore((s) => (activeId ? s.sessions[activeId] : undefined));
  const t = useT();

  if (!view) {
    return (
      <div className="ws-panel">
        <div className="empty">{t('files.noProject')}</div>
      </div>
    );
  }
  return (
    <div className="ws-panel ws-panel-flush">
      <DiffPane view={view} />
    </div>
  );
}

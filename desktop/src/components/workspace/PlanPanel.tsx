/**
 * PlanPanel.tsx — Plan panel for the right sidebar.
 * Shows the agent's current execution plan / TODO list.
 */

import { useStore } from '../../store';
import { Icon } from '../Icon';
import { useT } from '../../i18n/useT';

export function PlanPanel() {
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
    <div className="ws-panel">
      <div className="ws-panel-head">
        <Icon name="plan" size={15} />
        <span>{t('pane.plan')}</span>
        <span className="grow" />
        <span style={{ color: 'var(--text-faint)', fontSize: 11 }}>
          {view.plan.filter((p) => p.status === 'completed').length}/{view.plan.length}
        </span>
      </div>
      <div className="ws-panel-list">
        {view.plan.length === 0 ? (
          <div className="empty">{t('plan.empty')}</div>
        ) : (
          <div className="plan-list">
            {view.plan.map((e, i) => (
              <div key={i} className={`plan-entry ${e.status}`}>
                <PlanMark status={e.status} />
                <span className="plan-text">{e.content}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function PlanMark({ status }: { status: string }) {
  const icon = status === 'completed' ? 'circle-check' : status === 'in_progress' ? 'circle-dot' : 'circle';
  return (
    <span className={`plan-mark ${status}`}>
      <Icon name={icon} size={16} />
    </span>
  );
}

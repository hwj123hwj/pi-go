/**
 * WorkspaceToggles.tsx — The top-right layout toggle buttons (Codex-style):
 * show/hide the left session sidebar, the right feature sidebar, and the
 * bottom terminal panel.
 */

import { useStore } from '../../store';
import { Icon } from '../Icon';
import { useT } from '../../i18n/useT';
import { isElectron } from '../../platform';

export function WorkspaceToggles() {
  const workspace = useStore((s) => s.workspace);
  const toggleSidebar = useStore((s) => s.toggleSidebar);
  const toggleBottom = useStore((s) => s.toggleWorkspaceBottom);
  const toggleRight = useStore((s) => s.toggleWorkspaceRight);
  const t = useT();

  return (
    <div className="ws-toggles">
      {/* Sidebar toggle: hidden on mobile (use hamburger button instead) */}
      {isElectron && (
        <button
          className={`ws-toggle ${workspace.sidebarOpen ? 'active' : ''}`}
          title={workspace.sidebarOpen ? t('sidebar.collapse') : t('sidebar.expand')}
          aria-pressed={workspace.sidebarOpen}
          onClick={toggleSidebar}
        >
          <Icon name="panel" size={16} />
        </button>
      )}
      {/* Bottom terminal toggle: hidden on mobile */}
      {isElectron && (
        <button
          className={`ws-toggle ${workspace.bottomOpen ? 'active' : ''}`}
          title={t('workspace.toggleBottom')}
          aria-pressed={workspace.bottomOpen}
          onClick={toggleBottom}
        >
          <Icon name="panel-bottom" size={16} />
        </button>
      )}
      <button
        className={`ws-toggle ${workspace.rightOpen ? 'active' : ''}`}
        title={t('workspace.toggleRight')}
        aria-pressed={workspace.rightOpen}
        onClick={toggleRight}
      >
        <Icon name="panel-right" size={16} />
      </button>
    </div>
  );
}

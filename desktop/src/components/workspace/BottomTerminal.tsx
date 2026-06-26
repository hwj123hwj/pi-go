/**
 * BottomTerminal.tsx — VSCode-style bottom panel showing aggregated terminal
 * output (command executions) from the current session.
 *
 * This is the panel that appears when the bottom toggle is activated.
 * It renders a read-only log of all shell/bash commands and their output
 * from the active session's transcript.
 */

import { useEffect, useRef } from 'react';
import type { SessionView } from '../../store';
import { Icon } from '../Icon';
import { useT } from '../../i18n/useT';

export function BottomTerminal({ view, height }: { view: SessionView; height: number }) {
  const t = useT();
  const scrollRef = useRef<HTMLDivElement>(null);

  // Collect terminal output from tool calls
  const tools = view.transcript.filter(
    (item) => item.kind === 'tool',
  ) as Extract<SessionView['transcript'][number], { kind: 'tool' }>[];

  const entries = tools
    .filter((tool) => tool.terminalOutput)
    .map((tool) => ({
      id: tool.id,
      title: tool.title,
      output: tool.terminalOutput!,
      status: tool.status,
    }));

  // Auto-scroll to bottom on new output
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [entries.length]);

  return (
    <div className="bottom-terminal" style={{ height }}>
      <div className="bottom-terminal-head">
        <Icon name="terminal" size={14} />
        <span>{t('pane.terminal')}</span>
        <span className="grow" />
        <span className="bottom-terminal-count">
          {entries.length} {entries.length === 1 ? t('terminal.command') : t('terminal.commands')}
        </span>
      </div>
      <div className="bottom-terminal-body" ref={scrollRef}>
        {entries.length === 0 ? (
          <div className="empty">{t('terminal.noOutput')}</div>
        ) : (
          entries.map((entry) => (
            <div key={entry.id} className="terminal-entry">
              <div className="terminal-entry-cmd">
                <span className="terminal-prompt">$</span>
                <span className="terminal-cmd-text">{entry.title}</span>
                <span className={`terminal-status ${entry.status}`}>●</span>
              </div>
              {entry.output && (
                <pre className="terminal-output">{entry.output}</pre>
              )}
            </div>
          ))
        )}
      </div>
    </div>
  );
}

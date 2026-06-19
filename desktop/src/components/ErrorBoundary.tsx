import { Component, type ErrorInfo, type ReactNode } from 'react';
import { useT } from '../i18n/useT';

interface Props {
  children: ReactNode;
  /** Optional label so we can tell which region failed. */
  label?: string;
}

interface State {
  error: Error | null;
  info: string;
}

/**
 * Catches render/runtime errors in the subtree so an exception shows a readable
 * panel (and keeps the rest of the app alive) instead of unmounting React into a
 * blank white window. The previous build had no boundary, so any throw while
 * rendering a session turned the whole window white.
 */
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null, info: '' };

  static getDerivedStateFromError(error: Error): Partial<State> {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    // Surface to the console so it shows up in devtools / the backend log tail.
    console.error('[ErrorBoundary]', this.props.label ?? '', error, info.componentStack);
    this.setState({ info: info.componentStack ?? '' });
  }

  private reset = () => this.setState({ error: null, info: '' });

  render(): ReactNode {
    const { error, info } = this.state;
    if (!error) return this.props.children;
    return <ErrorCard message={error.message || String(error)} stack={info} onReset={this.reset} />;
  }
}

/**
 * Functional card so the (class) boundary's fallback can pull localized strings
 * via `useT()` and re-render when the language changes.
 */
function ErrorCard({
  message,
  stack,
  onReset,
}: {
  message: string;
  stack: string;
  onReset: () => void;
}) {
  const t = useT();
  return (
    <div className="error-boundary">
      <div className="error-boundary-card">
        <div className="error-boundary-title">{t('error.boundaryTitle')}</div>
        <div className="error-boundary-msg">{message}</div>
        {stack ? <pre className="error-boundary-stack">{stack.trim()}</pre> : null}
        <button className="btn primary" onClick={onReset}>
          {t('error.retry')}
        </button>
      </div>
    </div>
  );
}

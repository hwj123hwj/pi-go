import { useEffect, useRef, useState } from 'react';
import { useStore, type SessionView } from '../store';
import { Icon } from './Icon';
import { useT } from '../i18n/useT';
import { isElectron } from '../platform';
import { useVoiceInput } from '../hooks/useVoiceInput';

export function PromptBar({ view }: { view: SessionView }) {
  const sendPrompt = useStore((s) => s.sendPrompt);
  const cancel = useStore((s) => s.cancel);
  const setModel = useStore((s) => s.setModel);
  const models = useStore((s) => s.models);
  const t = useT();

  const [text, setText] = useState('');
  const taRef = useRef<HTMLTextAreaElement>(null);
  const isComposingRef = useRef(false);

  const meta = view.meta;
  const busy = meta.status === 'thinking' || meta.status === 'starting';

  const appendText = (newText: string) => {
    setText((prev) => {
      const needsSpace = prev && !prev.endsWith(' ') && !prev.endsWith('\n');
      return prev + (needsSpace ? ' ' : '') + newText;
    });
    // Focus the textarea so user can immediately review/edit
    taRef.current?.focus();
    // Place cursor at end
    requestAnimationFrame(() => {
      const ta = taRef.current;
      if (ta) {
        ta.selectionStart = ta.selectionEnd = ta.value.length;
      }
    });
  };

  const { recording, toggle: toggleVoice, transcribing, error: voiceError } = useVoiceInput({
    onText: appendText,
  });

  useEffect(() => {
    const ta = taRef.current;
    if (!ta) return;
    ta.style.height = 'auto';
    ta.style.height = Math.min(ta.scrollHeight, 200) + 'px';
  }, [text]);

  const submit = async () => {
    const trimmed = text.trim();
    if (!trimmed || busy) return;
    setText('');
    await sendPrompt(meta.id, trimmed);
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      if (e.nativeEvent.isComposing || e.keyCode === 229 || isComposingRef.current) return;
      e.preventDefault();
      void submit();
    }
  };

  return (
    <div className="promptbar">
      <div className="promptbar-inner">
        {/* Model selector */}
        <div className="prompt-config">
          <span className="chip">
            <Icon name="cpu" size={14} />
            <select value={meta.model ?? ''} onChange={(e) => void setModel(meta.id, e.target.value)}>
              {!meta.model && <option value="">{t('prompt.defaultModel')}</option>}
              {models.map((m) => (
                <option key={m.modelId} value={m.modelId}>
                  {m.name}
                </option>
              ))}
            </select>
          </span>
        </div>

        <div className="prompt-input-wrap" style={{ position: 'relative' }}>
          <textarea
            ref={taRef}
            className="prompt-input"
            rows={1}
            placeholder={busy ? t('prompt.busyPlaceholder') : t('prompt.placeholder')}
            value={text}
            onChange={(e) => setText(e.target.value)}
            onKeyDown={onKeyDown}
            onCompositionStart={() => { isComposingRef.current = true; }}
            onCompositionEnd={() => { isComposingRef.current = false; }}
          />
          {/* Voice input button */}
          <button
            className={`btn-voice ${recording ? 'recording' : ''}`}
            disabled={transcribing || busy}
            title={recording ? '点击停止录音' : transcribing ? '识别中...' : '语音输入'}
            onClick={toggleVoice}
          >
            {transcribing ? (
              <span className="spinner-xs" />
            ) : recording ? (
              <Icon name="stop" size={16} />
            ) : (
              <Icon name="mic" size={16} />
            )}
          </button>
          {voiceError && (
            <span className="voice-error-toast">{voiceError}</span>
          )}
          {busy ? (
            <button className="btn-stop" onClick={() => void cancel(meta.id)}>
              <Icon name="stop" size={14} />
              {isElectron ? t('common.stop') : ''}
            </button>
          ) : null}
          <button
            className="btn-send"
            disabled={!text.trim()}
            onClick={() => void submit()}
          >
            <Icon name="send" size={16} />
          </button>
        </div>
        {/* Hint with keyboard shortcut — desktop only */}
        {isElectron && <div className="hint">{t('prompt.hint', { paste: '⌘V' })}</div>}
      </div>
    </div>
  );
}

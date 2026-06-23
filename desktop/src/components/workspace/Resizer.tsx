/**
 * Resizer.tsx — A thin draggable divider used to resize a neighbouring panel.
 * Captures the pointer on mousedown, then reports an absolute target size
 * on each move (startValue + sign * pointerDelta).
 */

import { useCallback } from 'react';

export function Resizer({
  axis,
  getValue,
  onChange,
  sign = -1,
  title,
}: {
  axis: 'x' | 'y';
  getValue: () => number;
  onChange: (next: number) => void;
  sign?: 1 | -1;
  title?: string;
}) {
  const onPointerDown = useCallback(
    (e: React.PointerEvent<HTMLDivElement>) => {
      e.preventDefault();
      const startPos = axis === 'x' ? e.clientX : e.clientY;
      const startVal = getValue();
      const onMove = (ev: PointerEvent) => {
        const pos = axis === 'x' ? ev.clientX : ev.clientY;
        onChange(startVal + sign * (pos - startPos));
      };
      const onUp = () => {
        window.removeEventListener('pointermove', onMove);
        window.removeEventListener('pointerup', onUp);
        document.body.classList.remove('resizing');
      };
      window.addEventListener('pointermove', onMove);
      window.addEventListener('pointerup', onUp);
      document.body.classList.add('resizing');
    },
    [axis, getValue, onChange, sign],
  );

  return (
    <div
      className={`resizer resizer-${axis}`}
      role="separator"
      aria-orientation={axis === 'x' ? 'vertical' : 'horizontal'}
      title={title}
      onPointerDown={onPointerDown}
    />
  );
}

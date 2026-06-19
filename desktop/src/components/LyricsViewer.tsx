import { useState, useEffect, useRef, useMemo } from 'react';

interface LyricsViewerProps {
  /** Raw LRC text from music_lyrics tool result */
  lrcText: string;
  /** Current playback time in seconds (optional, for sync) */
  currentTime?: number;
}

interface LyricLine {
  time: number; // seconds
  text: string;
}

/** Parse LRC format into timed lines. */
function parseLRC(lrc: string): LyricLine[] {
  const lines: LyricLine[] = [];
  const re = /\[(\d{2}):(\d{2})\.(\d{2,3})\]\s*(.*)/;

  for (const raw of lrc.split('\n')) {
    const m = raw.match(re);
    if (!m) continue;
    const min = parseInt(m[1], 10);
    const sec = parseInt(m[2], 10);
    const ms = parseInt(m[3].padEnd(3, '0'), 10);
    const text = m[4].trim();
    if (text) {
      lines.push({ time: min * 60 + sec + ms / 1000, text });
    }
  }

  return lines.sort((a, b) => a.time - b.time);
}

/** Find the index of the current lyric line. */
function findCurrentLine(lines: LyricLine[], time: number): number {
  for (let i = lines.length - 1; i >= 0; i--) {
    if (time >= lines[i].time) return i;
  }
  return 0;
}

export function LyricsViewer({ lrcText, currentTime = 0 }: LyricsViewerProps) {
  const lines = useMemo(() => parseLRC(lrcText), [lrcText]);
  const containerRef = useRef<HTMLDivElement>(null);
  const [activeIdx, setActiveIdx] = useState(-1);

  useEffect(() => {
    if (lines.length === 0) return;
    const idx = findCurrentLine(lines, currentTime);
    if (idx !== activeIdx) {
      setActiveIdx(idx);
      // Scroll the active line into view
      const el = containerRef.current?.querySelector('.lyrics-line.active');
      el?.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
  }, [currentTime, lines, activeIdx]);

  if (lines.length === 0) {
    return (
      <div className="lyrics-viewer lyrics-empty">
        <pre className="lyrics-raw">{lrcText}</pre>
      </div>
    );
  }

  return (
    <div className="lyrics-viewer" ref={containerRef}>
      {lines.map((line, i) => (
        <div
          key={i}
          className={`lyrics-line ${i === activeIdx ? 'active' : ''} ${i < activeIdx ? 'past' : ''}`}
        >
          {line.text}
        </div>
      ))}
    </div>
  );
}

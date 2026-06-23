/**
 * GlobalMusicBar.tsx — A persistent, app-wide music player bar.
 *
 * The audio element lives here (mounted once in App.tsx) so that switching
 * sessions never unmounts it — music keeps playing across session switches.
 *
 * MusicPlayer.tsx (inline in chat) now dispatches play actions to the global
 * store instead of creating its own audio element.
 */

import { useRef, useEffect } from 'react';
import { useStore } from '../store';
import { useT } from '../i18n/useT';

function formatTime(seconds: number): string {
  if (!isFinite(seconds) || seconds < 0) return '0:00';
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${s.toString().padStart(2, '0')}`;
}

export function GlobalMusicBar() {
  const music = useStore((s) => s.music);
  const setMusicPlaying = useStore((s) => s.setMusicPlaying);
  const setMusicTime = useStore((s) => s.setMusicTime);
  const setMusicDuration = useStore((s) => s.setMusicDuration);
  const setMusicError = useStore((s) => s.setMusicError);
  const toggleMusic = useStore((s) => s.toggleMusic);
  const t = useT();

  const audioRef = useRef<HTMLAudioElement>(null);

  // Sync the audio element with store state
  useEffect(() => {
    const audio = audioRef.current;
    if (!audio) return;

    const onTime = () => setMusicTime(audio.currentTime);
    const onLoaded = () => setMusicDuration(audio.duration);
    const onEnded = () => setMusicPlaying(false);
    const onError = () => setMusicError(true);

    audio.addEventListener('timeupdate', onTime);
    audio.addEventListener('loadedmetadata', onLoaded);
    audio.addEventListener('ended', onEnded);
    audio.addEventListener('error', onError);

    return () => {
      audio.removeEventListener('timeupdate', onTime);
      audio.removeEventListener('loadedmetadata', onLoaded);
      audio.removeEventListener('ended', onEnded);
      audio.removeEventListener('error', onError);
    };
  }, [setMusicTime, setMusicDuration, setMusicPlaying, setMusicError]);

  // Auto play / pause when store state changes
  useEffect(() => {
    const audio = audioRef.current;
    if (!audio || !music.current) return;

    if (music.playing) {
      audio.play().catch(() => setMusicError(true));
    } else {
      audio.pause();
    }
  }, [music.playing, music.current, setMusicError]);

  // Nothing to show if no song loaded
  if (!music.current) return null;

  const audioURL = music.current.audioURL;

  return (
    <div className="global-music-bar">
      <audio
        ref={audioRef}
        src={audioURL}
        preload="metadata"
      />

      <div className="gm-info">
        <span className="gm-icon">🎵</span>
        <div className="gm-meta">
          <span className="gm-song">{music.current.songName}</span>
          {music.current.artist && <span className="gm-artist">{music.current.artist}</span>}
        </div>
      </div>

      <div className="gm-controls">
        <button
          className="gm-btn"
          onClick={toggleMusic}
          disabled={music.error}
          title={music.playing ? t('music.pause') : t('music.play')}
        >
          {music.error ? '✗' : music.playing ? '⏸' : '▶'}
        </button>

        <span className="gm-time">{formatTime(music.currentTime)}</span>

        <input
          className="gm-progress"
          type="range"
          min={0}
          max={music.duration || 0}
          step={0.1}
          value={music.currentTime}
          disabled={music.error}
          onChange={(e) => {
            const time = parseFloat(e.target.value);
            if (audioRef.current) audioRef.current.currentTime = time;
            setMusicTime(time);
          }}
        />

        <span className="gm-time">{formatTime(music.duration)}</span>
      </div>

      {music.error && <div className="gm-error">{t('music.loadFailed')}</div>}
    </div>
  );
}

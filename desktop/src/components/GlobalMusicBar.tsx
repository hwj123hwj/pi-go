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
  const clearMusic = useStore((s) => s.clearMusic);
  const t = useT();

  const audioRef = useRef<HTMLAudioElement>(null);

  // Sync the audio element with store state
  useEffect(() => {
    const audio = audioRef.current;
    if (!audio) return;

    const onTime = () => setMusicTime(audio.currentTime);
    // Only use audio element's duration if the store doesn't already have one
    // from the backend's structured PlayDetails. Bilibili CDN uses chunked
    // transfer without Content-Length, making audio.duration = NaN.
    const onLoaded = () => {
      if (isFinite(audio.duration) && audio.duration > 0) {
        setMusicDuration(audio.duration);
      }
    };
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

  // When the audio src changes (new song selected), force-reload the media
  // element and auto-play. React updates the src attribute but the media
  // element needs .load() to actually abandon the old source.
  // We also call .play() here (not in a separate effect) to avoid a race
  // where play() fires before the new src has finished loading.
  const audioURL = music.current?.audioURL ?? '';
  useEffect(() => {
    const audio = audioRef.current;
    if (!audio || !audioURL) return;

    const wasPlaying = music.playing;
    audio.load();

    // If the store says we should be playing, start playback after load.
    if (wasPlaying) {
      // canplay fires when enough data is loaded to begin playback.
      const onCanPlay = () => {
        audio.play().catch(() => setMusicError(true));
        audio.removeEventListener('canplay', onCanPlay);
      };
      audio.addEventListener('canplay', onCanPlay);
      return () => audio.removeEventListener('canplay', onCanPlay);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [audioURL]);

  // Auto play / pause when store state changes.
  // NOTE: This handles the toggle play/pause case (same song). New-song
  // auto-play is handled by the audioURL effect above.
  useEffect(() => {
    const audio = audioRef.current;
    if (!audio || !music.current) return;

    // Skip if the audio src is still loading the current track.
    // The audioURL effect handles initial play.
    if (audio.readyState < audio.HAVE_CURRENT_DATA) return;

    if (music.playing) {
      audio.play().catch(() => setMusicError(true));
    } else {
      audio.pause();
    }
  }, [music.playing, music.current, setMusicError]);

  // Nothing to show if no song loaded
  if (!music.current) return null;

  // Error recovery: directly reload the audio element and retry playback.
  // We don't go through playMusic() because the URL hasn't changed, so the
  // audioURL effect wouldn't fire. Instead, we manipulate the element directly.
  const handleRetry = () => {
    const audio = audioRef.current;
    if (!audio || !music.current) return;
    setMusicError(false);
    audio.load();
    audio.play().catch(() => setMusicError(true));
  };

  const handleButtonClick = () => {
    if (music.error) {
      handleRetry();
    } else {
      toggleMusic();
    }
  };

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
          onClick={handleButtonClick}
          title={music.error ? t('music.retry') : music.playing ? t('music.pause') : t('music.play')}
        >
          {music.error ? '↻' : music.playing ? '⏸' : '▶'}
        </button>

        <span className="gm-time">{formatTime(music.currentTime)}</span>

        <input
          className="gm-progress"
          type="range"
          min={0}
          max={music.duration || 0}
          step={0.1}
          value={music.currentTime}
          disabled={!music.duration}
          onChange={(e) => {
            const time = parseFloat(e.target.value);
            if (audioRef.current) audioRef.current.currentTime = time;
            setMusicTime(time);
          }}
        />

        <span className="gm-time">{music.duration ? formatTime(music.duration) : '—'}</span>
      </div>

      {/* Close button — stops playback, clears src, and hides the bar */}
      <button
        className="gm-close"
        onClick={() => {
          const audio = audioRef.current;
          if (audio) {
            audio.pause();
            audio.removeAttribute('src');
            audio.load();
          }
          clearMusic();
        }}
        title={t('music.close')}
        aria-label={t('music.close')}
      >
        ✕
      </button>

      {music.error && <span className="gm-error">{t('music.loadFailed')}</span>}
    </div>
  );
}

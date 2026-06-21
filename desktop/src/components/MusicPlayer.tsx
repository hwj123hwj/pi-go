import { useState, useRef, useEffect, useCallback } from 'react';
import { useT } from '../i18n/useT';

/** music_play 的结构化结果（后端 PlayDetails 的 JSON 镜像） */
interface MusicPlayDetails {
  song_id?: number;
  song_name?: string;
  artist?: string;
  direct_url?: string;
  proxy_url?: string;
}

interface MusicPlayerProps {
  /** 结构化结果（优先用，来自后端 ToolResult.Details） */
  details?: Record<string, unknown>;
  /** 自由文本（结构化缺失时的 fallback，旧格式兼容） */
  resultText: string;
}

/** 从自由文本正则提取播放信息（仅当 details 缺失时用，向后兼容）。 */
function parsePlayResult(text: string) {
  const songMatch = text.match(/🎵\s*Now playing:\s*(.+?)\s*[—–-]\s*(.+?)(?:\n|$)/);
  const directMatch = text.match(/Direct URL:\s*(\S+)/);
  const proxyMatch = text.match(/Proxy URL:\s*(\S+)/);
  return {
    songName: songMatch?.[1]?.trim() || '',
    artist: songMatch?.[2]?.trim() || '',
    directURL: directMatch?.[1] || '',
    proxyURL: proxyMatch?.[1] || '',
  };
}

function formatTime(seconds: number): string {
  if (!isFinite(seconds) || seconds < 0) return '0:00';
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${s.toString().padStart(2, '0')}`;
}

export function MusicPlayer({ resultText, details }: MusicPlayerProps) {
  const t = useT();

  // 优先用结构化 details，缺失则 fallback 正则解析文本
  const det = details as MusicPlayDetails | undefined;
  const parsed = parsePlayResult(resultText);
  const songName = det?.song_name || parsed.songName || t('music.unknownSong');
  const artist = det?.artist || parsed.artist;
  const directURL = det?.direct_url || parsed.directURL;
  const proxyURL = det?.proxy_url || parsed.proxyURL;
  const audioURL = proxyURL || directURL;

  const audioRef = useRef<HTMLAudioElement>(null);
  const [playing, setPlaying] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [volume, setVolume] = useState(0.8);
  const [error, setError] = useState(false);

  useEffect(() => {
    const audio = audioRef.current;
    if (!audio) return;

    const onTime = () => setCurrentTime(audio.currentTime);
    const onDuration = () => setDuration(audio.duration);
    const onEnded = () => setPlaying(false);
    const onError = () => setError(true);

    audio.addEventListener('timeupdate', onTime);
    audio.addEventListener('loadedmetadata', onDuration);
    audio.addEventListener('ended', onEnded);
    audio.addEventListener('error', onError);

    return () => {
      audio.removeEventListener('timeupdate', onTime);
      audio.removeEventListener('loadedmetadata', onDuration);
      audio.removeEventListener('ended', onEnded);
      audio.removeEventListener('error', onError);
    };
  }, [audioURL]);

  const togglePlay = useCallback(() => {
    const audio = audioRef.current;
    if (!audio || error) return;
    if (playing) {
      audio.pause();
      setPlaying(false);
    } else {
      audio.play().then(() => setPlaying(true)).catch(() => setError(true));
    }
  }, [playing, error]);

  const seek = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const audio = audioRef.current;
    if (!audio) return;
    const time = parseFloat(e.target.value);
    audio.currentTime = time;
    setCurrentTime(time);
  }, []);

  const changeVolume = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const v = parseFloat(e.target.value);
    setVolume(v);
    if (audioRef.current) audioRef.current.volume = v;
  }, []);

  if (!audioURL) {
    return (
      <div className="music-player music-player-error">
        <span>🎵 {songName}{artist ? ` — ${artist}` : ''}</span>
        <span className="music-player-err-msg">{t('music.noAudio')}</span>
      </div>
    );
  }

  return (
    <div className="music-player">
      <audio ref={audioRef} src={audioURL} preload="metadata" />

      <div className="music-player-info">
        <span className="music-player-icon">🎵</span>
        <div className="music-player-meta">
          <span className="music-player-song">{songName}</span>
          {artist && <span className="music-player-artist">{artist}</span>}
        </div>
      </div>

      <div className="music-player-controls">
        <button
          className="music-player-btn"
          onClick={togglePlay}
          disabled={error}
          title={playing ? t('music.pause') : t('music.play')}
        >
          {error ? '✗' : playing ? '⏸' : '▶'}
        </button>

        <span className="music-player-time">{formatTime(currentTime)}</span>

        <input
          className="music-player-progress"
          type="range"
          min={0}
          max={duration || 0}
          step={0.1}
          value={currentTime}
          onChange={seek}
          disabled={error}
        />

        <span className="music-player-time">{formatTime(duration)}</span>

        <span className="music-player-vol-icon" title={t('music.volume')}>🔊</span>
        <input
          className="music-player-volume"
          type="range"
          min={0}
          max={1}
          step={0.05}
          value={volume}
          onChange={changeVolume}
        />
      </div>

      {error && <div className="music-player-error-bar">{t('music.loadFailed')}</div>}
    </div>
  );
}

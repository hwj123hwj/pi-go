import { useStore } from '../store';
import { useT } from '../i18n/useT';
import { getBaseUrl } from '../store';

/** music_play 的结构化结果（后端 PlayDetails 的 JSON 镜像） */
interface MusicPlayDetails {
  song_id?: string;
  song_name?: string;
  artist?: string;
  direct_url?: string;
  proxy_url?: string;
  duration?: number; // seconds
  source?: string;
  is_fallback?: boolean;
}

interface MusicPlayerProps {
  /** 结构化结果（优先用，来自后端 ToolResult.Details） */
  details?: Record<string, unknown>;
  /** 自由文本（结构化缺失时的 fallback，旧格式兼容） */
  resultText: string;
}

/** 从自由文本正则提取播放信息（仅当 details 缺失时用，向后兼容）。 */
function parsePlayResult(text: string) {
  // English format
  const songMatchEN = text.match(/🎵\s*Now playing:\s*(.+?)\s*[—–-]\s*(.+?)(?:\n|$)/);
  const directMatch = text.match(/Direct URL:\s*(\S+)/);
  const proxyMatch = text.match(/Proxy URL:\s*(\S+)/);
  // Chinese format: "曲名：xxx" and "歌手：xxx" and "播放链接：xxx"
  const songMatchZH = text.match(/曲名[：:]\s*(.+?)(?:\n|$)/);
  const artistMatchZH = text.match(/歌手[：:]\s*(.+?)(?:\n|$)/);
  const proxyMatchZH = text.match(/播放链接[：:]\s*(\S+)/);
  return {
    songName: songMatchEN?.[1]?.trim() || songMatchZH?.[1]?.trim() || '',
    artist: songMatchEN?.[2]?.trim() || artistMatchZH?.[1]?.trim() || '',
    directURL: directMatch?.[1] || '',
    proxyURL: proxyMatch?.[1] || proxyMatchZH?.[1] || '',
  };
}

/**
 * Rewrite a stored audio proxy URL to use the current server's base URL.
 * Historical sessions may contain URLs with old ports (from previous app launches).
 * The path portion (e.g. /music/audio/netease_576466) remains valid since it
 * identifies the resource, so we just replace the origin.
 */
function rewriteAudioURL(storedURL: string): string {
  if (!storedURL) return storedURL;
  try {
    const currentBase = getBaseUrl();
    const storedUrl = new URL(storedURL);
    const currentUrl = new URL(currentBase);
    // Only rewrite if the origin differs (port changed after restart)
    if (storedUrl.origin !== currentUrl.origin) {
      storedUrl.protocol = currentUrl.protocol;
      storedUrl.hostname = currentUrl.hostname;
      storedUrl.port = currentUrl.port;
    }
    return storedUrl.toString();
  } catch {
    // If URL parsing fails, return as-is
    return storedURL;
  }
}

/**
 * Inline music card — shown inside chat transcripts.
 * Clicking play dispatches the song to the global player (GlobalMusicBar),
 * so the audio keeps playing even when switching sessions.
 */
export function MusicPlayer({ resultText, details }: MusicPlayerProps) {
  const t = useT();
  const playMusic = useStore((s) => s.playMusic);
  const globalMusic = useStore((s) => s.music);

  // 优先用结构化 details，缺失则 fallback 正则解析文本
  const det = details as MusicPlayDetails | undefined;
  const parsed = parsePlayResult(resultText);
  const songName = det?.song_name || parsed.songName || t('music.unknownSong');
  const artist = det?.artist || parsed.artist;
  const directURL = det?.direct_url || parsed.directURL;
  const proxyURL = det?.proxy_url || parsed.proxyURL;
  const audioURL = rewriteAudioURL(proxyURL) || directURL;

  // Is this song currently the active global track?
  const isActive = globalMusic.current?.audioURL === audioURL;

  if (!audioURL) {
    return (
      <div className="music-player music-player-error">
        <span>🎵 {songName}{artist ? ` — ${artist}` : ''}</span>
        <span className="music-player-err-msg">{t('music.noAudio')}</span>
      </div>
    );
  }

  const handlePlay = () => {
    // If already the active track, toggle play/pause (or retry on error)
    if (isActive) {
      if (globalMusic.error) {
        playMusic({ songName, artist, audioURL, duration: det?.duration || 0 });
      } else {
        useStore.getState().toggleMusic();
      }
    } else {
      // Otherwise, load this song into the global player
      playMusic({ songName, artist, audioURL, duration: det?.duration || 0 });
    }
  };

  const showPause = isActive && globalMusic.playing && !globalMusic.error;

  return (
    <div className="music-player">
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
          onClick={handlePlay}
          disabled={false}
          title={isActive && globalMusic.error ? t('music.retry') : showPause ? t('music.pause') : t('music.play')}
        >
          {isActive && globalMusic.error ? '↻' : showPause ? '⏸' : '▶'}
        </button>

        {isActive && (
          <>
            <span className="music-player-time">
              {formatTime(globalMusic.currentTime)}{globalMusic.duration ? ` / ${formatTime(globalMusic.duration)}` : ''}
            </span>
          </>
        )}
      </div>

      {isActive && globalMusic.error && (
        <div className="music-player-error-bar">{t('music.loadFailed')}</div>
      )}
    </div>
  );
}

function formatTime(seconds: number): string {
  if (!isFinite(seconds) || seconds < 0) return '0:00';
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${s.toString().padStart(2, '0')}`;
}

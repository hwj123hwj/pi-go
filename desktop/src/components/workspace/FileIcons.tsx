/**
 * FileIcons.tsx — Per-extension file/folder icons for the file explorer.
 * Maps common file extensions to colored emoji/symbol badges.
 */

import { Icon } from '../Icon';

const EXT_EMOJI: Record<string, string> = {
  // languages
  go: '🐹', rs: '🦀', ts: '📘', tsx: '📘', js: '📙', mjs: '📙', cjs: '📙', jsx: '📙',
  py: '🐍', java: '☕', kt: '☕', kts: '☕', rb: '💎', php: '🐘', swift: '🍎',
  c: '🔵', h: '🔵', cpp: '🔵', cc: '🔵', cxx: '🔵', hpp: '🔵', cs: '🟣',
  lua: '🌙', r: '📊', scala: '🔴', clj: '🟢', ex: '💧', exs: '💧', erl: 'abra',
  // data / config
  json: '🔧', jsonc: '🔧', yml: '⚙️', yaml: '⚙️', toml: '⚙️', ini: '⚙️', conf: '⚙️',
  env: '🔐', lock: '🔒',
  // markup / style
  html: '🌐', htm: '🌐', css: '🎨', scss: '🎨', less: '🎨', vue: '💚', svelte: '🔥',
  md: '📝', markdown: '📝', txt: '📄', pdf: '📕',
  // media
  png: '🖼️', jpg: '🖼️', jpeg: '🖼️', gif: '🖼️', webp: '🖼️', bmp: '🖼️', svg: '🖼️',
  ico: '🖼️',
  // audio / video
  mp3: '🎵', wav: '🎵', flac: '🎵', ogg: '🎵', m4a: '🎵', mp4: '🎬', webm: '🎬',
  // build / misc
  sql: '🗄️', graphql: '⟡', gql: '⟡', proto: '🔧', dockerfile: '🐳',
  sh: '💲', bash: '💲', zsh: '💲', fish: '💲', ps1: '💲',
  gitignore: '🚫', gitattributes: '🚫', gitmodules: '🚫',
};

export function FileIcon({
  name,
  isDir,
  open,
  size = 15,
}: {
  name: string;
  isDir?: boolean;
  open?: boolean;
  size?: number;
}) {
  if (isDir) {
    return <Icon name={open ? 'folder-open' : 'folder'} size={size} />;
  }
  const base = name.toLowerCase();
  // Extensionless special filenames
  if (base === 'dockerfile') return <span style={{ fontSize: size }}>🐳</span>;
  if (base === 'makefile') return <span style={{ fontSize: size }}>🔨</span>;
  const dot = base.lastIndexOf('.');
  const ext = dot >= 0 ? base.slice(dot + 1) : '';
  const emoji = EXT_EMOJI[ext];
  if (emoji) {
    return <span style={{ fontSize: Math.round(size * 0.85), lineHeight: 1 }}>{emoji}</span>;
  }
  return <Icon name="file" size={size} />;
}

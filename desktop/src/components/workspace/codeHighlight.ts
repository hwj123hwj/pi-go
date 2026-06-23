/**
 * codeHighlight.ts — Syntax highlighting for the file viewer and Markdown code fences.
 * Uses highlight.js' "common" bundle (~40 mainstream languages, synchronous, no wasm).
 * Emits HTML with `hljs-*` token classes; colours are mapped to theme CSS vars.
 */

import hljs from 'highlight.js/lib/common';

const EXT_LANG: Record<string, string> = {
  js: 'javascript', mjs: 'javascript', cjs: 'javascript', jsx: 'javascript',
  ts: 'typescript', tsx: 'typescript', mts: 'typescript', cts: 'typescript',
  json: 'json', jsonc: 'json',
  py: 'python', pyw: 'python',
  rb: 'ruby', rs: 'rust', go: 'go', java: 'java', kt: 'kotlin', kts: 'kotlin',
  swift: 'swift', c: 'c', h: 'c', cpp: 'cpp', cc: 'cpp', cxx: 'cpp', hpp: 'cpp',
  cs: 'csharp', php: 'php', lua: 'lua', pl: 'perl', r: 'r', sql: 'sql',
  sh: 'bash', bash: 'bash', zsh: 'bash', ps1: 'powershell',
  html: 'xml', htm: 'xml', xml: 'xml', svg: 'xml', vue: 'xml',
  css: 'css', scss: 'scss', less: 'less',
  md: 'markdown', markdown: 'markdown',
  yml: 'yaml', yaml: 'yaml', toml: 'ini', ini: 'ini', conf: 'ini',
  diff: 'diff', patch: 'diff', dockerfile: 'dockerfile', makefile: 'makefile',
  graphql: 'graphql', gql: 'graphql',
};

export function languageForFile(name: string): string | undefined {
  const base = name.toLowerCase();
  if (base === 'dockerfile') return 'dockerfile';
  if (base === 'makefile') return 'makefile';
  const dot = base.lastIndexOf('.');
  if (dot < 0) return undefined;
  const ext = base.slice(dot + 1);
  const lang = EXT_LANG[ext] ?? ext;
  return hljs.getLanguage(lang) ? lang : undefined;
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

export function highlightCode(code: string, hint: string): { html: string; language: string } {
  try {
    const lang = hint.includes('.') || !hljs.getLanguage(hint) ? languageForFile(hint) : hint;
    if (lang) {
      const res = hljs.highlight(code, { language: lang, ignoreIllegals: true });
      return { html: res.value, language: lang };
    }
    const auto = hljs.highlightAuto(code);
    return { html: auto.value, language: auto.language ?? 'plaintext' };
  } catch {
    return { html: escapeHtml(code), language: 'plaintext' };
  }
}

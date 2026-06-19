export const PROVISIONAL_TITLE_MAX = 30;

export function stripImageHints(text: string): string {
  return text.replace(/\n*\[IMAGE:[^\]]*\][ \t]*/g, '').trim();
}

export function deriveTitleFromMessage(text: string): string {
  const cleaned = stripImageHints(text).replace(/\s+/g, ' ').trim();
  if (!cleaned) return '';
  if (cleaned.length <= PROVISIONAL_TITLE_MAX) return cleaned;
  return cleaned.slice(0, PROVISIONAL_TITLE_MAX) + '…';
}

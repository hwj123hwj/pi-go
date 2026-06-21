import { Fragment, type ReactNode } from 'react';
import { Mermaid } from './Mermaid';

/**
 * A compact, dependency-free Markdown renderer. Handles the subset that shows up
 * in agent transcripts: fenced code blocks, inline code, headings, bold/italic,
 * links, ordered/unordered lists, and paragraphs. Everything is escaped by
 * React (we only build elements, never dangerouslySetInnerHTML).
 */
export function Markdown({ text }: { text: string }) {
  return <div className="md">{renderBlocks(text)}</div>;
}

function renderBlocks(src: string): ReactNode[] {
  const out: ReactNode[] = [];
  const lines = src.split('\n');
  let i = 0;
  let key = 0;

  while (i < lines.length) {
    const line = lines[i];

    // fenced code block
    const fence = line.match(/^```(\w*)\s*$/);
    if (fence) {
      const lang = fence[1];
      const buf: string[] = [];
      i++;
      while (i < lines.length && !/^```\s*$/.test(lines[i])) {
        buf.push(lines[i]);
        i++;
      }
      i++; // skip closing fence
      if (lang.toLowerCase() === 'mermaid') {
        out.push(<Mermaid key={key++} code={buf.join('\n')} />);
      } else {
        out.push(
          <pre key={key++}>
            <code data-lang={lang}>{buf.join('\n')}</code>
          </pre>,
        );
      }
      continue;
    }

    // heading
    const heading = line.match(/^(#{1,4})\s+(.*)$/);
    if (heading) {
      const level = heading[1].length;
      const Tag = (`h${Math.min(level, 3)}` as 'h1' | 'h2' | 'h3');
      out.push(<Tag key={key++}>{renderInline(heading[2])}</Tag>);
      i++;
      continue;
    }

    // unordered list
    if (/^\s*[-*]\s+/.test(line)) {
      const items: string[] = [];
      while (i < lines.length && /^\s*[-*]\s+/.test(lines[i])) {
        items.push(lines[i].replace(/^\s*[-*]\s+/, ''));
        i++;
      }
      out.push(
        <ul key={key++}>
          {items.map((it, idx) => (
            <li key={idx}>{renderInline(it)}</li>
          ))}
        </ul>,
      );
      continue;
    }

    // ordered list
    if (/^\s*\d+\.\s+/.test(line)) {
      const items: string[] = [];
      while (i < lines.length && /^\s*\d+\.\s+/.test(lines[i])) {
        items.push(lines[i].replace(/^\s*\d+\.\s+/, ''));
        i++;
      }
      out.push(
        <ol key={key++}>
          {items.map((it, idx) => (
            <li key={idx}>{renderInline(it)}</li>
          ))}
        </ol>,
      );
      continue;
    }

    // GFM table: a header row immediately followed by a delimiter row.
    if (isTableStart(lines, i)) {
      const header = splitTableRow(lines[i]);
      const aligns = splitTableRow(lines[i + 1]).map(parseAlign);
      i += 2;
      const rows: string[][] = [];
      while (i < lines.length && lines[i].trim() !== '' && lines[i].includes('|')) {
        rows.push(splitTableRow(lines[i]));
        i++;
      }
      const align = (col: number) => aligns[col] ?? undefined;
      out.push(
        <div className="md-table-wrap" key={key++}>
          <table>
            <thead>
              <tr>
                {header.map((cell, idx) => (
                  <th key={idx} style={{ textAlign: align(idx) }}>
                    {renderInline(cell)}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((row, ridx) => (
                <tr key={ridx}>
                  {header.map((_, idx) => (
                    <td key={idx} style={{ textAlign: align(idx) }}>
                      {renderInline(row[idx] ?? '')}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>,
      );
      continue;
    }

    // blank line
    if (line.trim() === '') {
      i++;
      continue;
    }

    // paragraph (consume consecutive non-blank, non-special lines)
    const para: string[] = [];
    while (
      i < lines.length &&
      lines[i].trim() !== '' &&
      !/^```/.test(lines[i]) &&
      !/^(#{1,4})\s+/.test(lines[i]) &&
      !/^\s*[-*]\s+/.test(lines[i]) &&
      !/^\s*\d+\.\s+/.test(lines[i]) &&
      !isTableStart(lines, i)
    ) {
      para.push(lines[i]);
      i++;
    }
    out.push(<p key={key++}>{renderInline(para.join('\n'))}</p>);
  }

  return out;
}

/**
 * Split a markdown table row into trimmed cells. Strips one optional leading
 * and trailing pipe and respects escaped pipes (`\|`).
 */
function splitTableRow(line: string): string[] {
  let s = line.trim();
  if (s.startsWith('|')) s = s.slice(1);
  if (s.endsWith('|')) s = s.slice(0, -1);
  const cells: string[] = [];
  let cur = '';
  for (let k = 0; k < s.length; k++) {
    if (s[k] === '\\' && s[k + 1] === '|') {
      cur += '|';
      k++;
      continue;
    }
    if (s[k] === '|') {
      cells.push(cur);
      cur = '';
      continue;
    }
    cur += s[k];
  }
  cells.push(cur);
  return cells.map((c) => c.trim());
}

/** A GFM delimiter row, e.g. `|---|:--:|` — every cell is `:?-+:?`. */
function isDelimiterRow(line: string): boolean {
  if (!line.includes('-') || !line.includes('|')) return false;
  const cells = splitTableRow(line);
  return cells.length > 0 && cells.every((c) => /^:?-{1,}:?$/.test(c));
}

/** Column alignment from a delimiter cell. */
function parseAlign(cell: string): 'left' | 'center' | 'right' | undefined {
  const left = cell.startsWith(':');
  const right = cell.endsWith(':');
  if (left && right) return 'center';
  if (right) return 'right';
  if (left) return 'left';
  return undefined;
}

/** True when `lines[i]` is a table header followed by a delimiter row. */
function isTableStart(lines: string[], i: number): boolean {
  return (
    i + 1 < lines.length &&
    lines[i].includes('|') &&
    lines[i].trim() !== '' &&
    isDelimiterRow(lines[i + 1])
  );
}

/** Inline: `code`, **bold**, *italic*, [text](url). */
function renderInline(text: string): ReactNode[] {
  const nodes: ReactNode[] = [];
  // Split on inline code first to avoid formatting inside it.
  const parts = text.split(/(`[^`]+`)/g);
  let key = 0;
  for (const part of parts) {
    if (part.startsWith('`') && part.endsWith('`') && part.length > 1) {
      const codeContent = part.slice(1, -1);
      // Check if the code content is a file path
      if (isFilePath(codeContent)) {
        nodes.push(
          <code
            key={key++}
            className="file-path-link"
            onClick={(e) => {
              e.preventDefault();
              // Dispatch a custom event to open the file in FilePane
              window.dispatchEvent(new CustomEvent('open-file', { detail: { path: codeContent } }));
            }}
            style={{ cursor: 'pointer', textDecoration: 'underline' }}
          >
            {codeContent}
          </code>
        );
      } else {
        nodes.push(<code key={key++}>{codeContent}</code>);
      }
    } else {
      nodes.push(<Fragment key={key++}>{renderEmphasis(part)}</Fragment>);
    }
  }
  return nodes;
}

// Check if a string looks like a file path.
// Matches: paths with known extensions, directory paths (ending with /),
// and paths with ≥2 segments but no extension (e.g. /Users/.../work/something).
function isFilePath(text: string): boolean {
  // Strip trailing slash for analysis
  const stripped = text.replace(/\/+$/, '');
  // Must start with / or ~/
  if (!/^~?\//.test(stripped)) return false;
  // Exclude URLs
  if (/^https?:\/\//i.test(text)) return false;
  // Known extensions (covers most source/config/doc files)
  if (/^~?\/[^\s]+\.(md|txt|json|js|ts|go|py|yaml|yml|toml|xml|html|css|sh|bash|rs|java|c|cpp|h|rb|php|sql|graphql|proto|tf|vue|svelte|jsx|tsx|mdx|csv|log|cfg|conf|ini|env|lock|sum|mod)$/i.test(text)) return true;
  // Directory path (ends with / and has ≥2 segments)
  if (/\/$/.test(text) && stripped.split('/').filter(Boolean).length >= 2) return true;
  // File/dir with ≥2 segments, no extension in last segment (e.g. /Users/foo/bar/something)
  const lastSegment = stripped.split('/').pop() || '';
  if (stripped.split('/').filter(Boolean).length >= 2 && !lastSegment.includes('.')) return true;
  return false;
}

function renderEmphasis(text: string): ReactNode[] {
  const nodes: ReactNode[] = [];
  // Image `![alt](src)` must come before the link alternative so the leading
  // `!` is consumed as part of the image rather than left as literal text.
  const re = /(!\[[^\]]*\]\([^)]+\)|\*\*[^*]+\*\*|\*[^*]+\*|\[[^\]]+\]\([^)]+\))/g;
  let last = 0;
  let m: RegExpExecArray | null;
  let key = 0;
  while ((m = re.exec(text)) !== null) {
    if (m.index > last) nodes.push(...renderTextWithFilePaths(text.slice(last, m.index), key));
    key += 100; // reserve keyspace for path nodes
    const token = m[0];
    if (token.startsWith('![')) {
      const im = token.match(/^!\[([^\]]*)\]\(([^)]+)\)$/);
      if (im) {
        // The URL part may carry an optional CommonMark title: `(src "title")`.
        const inner = im[2].trim();
        const titleMatch = inner.match(/^(\S+)\s+["'(](.*)["']$/);
        const src = titleMatch ? titleMatch[1] : inner;
        const title = titleMatch ? titleMatch[2] : undefined;
        nodes.push(
          <img key={key++} className="md-img" src={src} alt={im[1]} title={title} />,
        );
      } else {
        nodes.push(<Fragment key={key++}>{token}</Fragment>);
      }
    } else if (token.startsWith('**')) {
      nodes.push(<strong key={key++}>{token.slice(2, -2)}</strong>);
    } else if (token.startsWith('*')) {
      nodes.push(<em key={key++}>{token.slice(1, -1)}</em>);
    } else {
      const lm = token.match(/^\[([^\]]+)\]\(([^)]+)\)$/);
      if (lm) {
        nodes.push(
          <a
            key={key++}
            href={lm[2]}
            onClick={(e) => {
              e.preventDefault();
              void window.open(lm[2], '_blank');
            }}
          >
            {lm[1]}
          </a>,
        );
      } else {
        nodes.push(<Fragment key={key++}>{token}</Fragment>);
      }
    }
    last = m.index + token.length;
  }
  if (last < text.length) nodes.push(...renderTextWithFilePaths(text.slice(last), key));
  return nodes;
}

// Scan plain text for file paths and make them clickable.
const PATH_RE = /(\/(?:[^\s\n]*\/)+[^\s\n/.]+)/g;

function renderTextWithFilePaths(text: string, baseKey: number): ReactNode[] {
  const nodes: ReactNode[] = [];
  let last = 0;
  let m: RegExpExecArray | null;
  let key = baseKey;
  PATH_RE.lastIndex = 0;
  while ((m = PATH_RE.exec(text)) !== null) {
    const path = m[0];
    if (!isFilePath(path)) continue;
    if (m.index > last) nodes.push(text.slice(last, m.index));
    nodes.push(
      <code
        key={key++}
        className="file-path-link"
        onClick={(e) => {
          e.preventDefault();
          window.dispatchEvent(new CustomEvent('open-file', { detail: { path } }));
        }}
        style={{ cursor: 'pointer', textDecoration: 'underline' }}
      >
        {path}
      </code>,
    );
    last = m.index + path.length;
  }
  if (last < text.length) nodes.push(text.slice(last));
  if (nodes.length === 0) return [text]; // no paths found, return plain string
  return nodes;
}

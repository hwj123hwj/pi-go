// Lightweight Markdown renderer — zero dependencies
// Handles: headings, bold, italic, code blocks, inline code, lists, links, blockquotes, tables, hr

const escapeHtml = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');

// Language keywords for minimal syntax highlighting
const KEYWORDS = {
  go: /\b(func|package|import|return|if|else|for|range|var|const|type|struct|interface|map|chan|go|defer|select|case|switch|default|break|continue|nil|true|false)\b/g,
  js: /\b(const|let|var|function|return|if|else|for|while|switch|case|break|continue|class|import|export|from|async|await|try|catch|throw|new|this|null|undefined|true|false)\b/g,
  python: /\b(def|class|import|from|return|if|elif|else|for|while|try|except|finally|with|as|lambda|yield|pass|break|continue|True|False|None|and|or|not|in|is)\b/g,
  bash: /\b(if|then|else|elif|fi|for|while|do|done|case|esac|function|echo|exit|return|local|export|source|cd|ls|grep|sed|awk|cat|rm|mv|cp|mkdir|chmod|chown)\b/g,
  sql: /\b(SELECT|FROM|WHERE|AND|OR|NOT|INSERT|UPDATE|DELETE|CREATE|DROP|ALTER|TABLE|INDEX|JOIN|INNER|LEFT|RIGHT|OUTER|ON|GROUP|BY|ORDER|HAVING|LIMIT|OFFSET|UNION|AS|SET|VALUES|INTO)\b/gi,
};

function highlightCode(code, lang) {
  let escaped = escapeHtml(code);
  const kw = KEYWORDS[lang] || KEYWORDS[lang?.split('-')[0]];
  if (kw) {
    escaped = escaped.replace(kw, '<span class="keyword">$&</span>');
  }
  // Highlight strings
  escaped = escaped.replace(/(["'`])(?:(?!\1|\\).|\\.)*\1/g, '<span class="string">$&</span>');
  // Highlight comments (// and #)
  escaped = escaped.replace(/(\/\/.*$|#.*$)/gm, '<span class="comment">$&</span>');
  // Highlight numbers
  escaped = escaped.replace(/\b(\d+\.?\d*)\b/g, '<span class="number">$1</span>');
  return escaped;
}

export function renderMarkdown(text) {
  if (!text) return '';

  // Normalize line endings
  let src = text.replace(/\r\n/g, '\n');

  // Split into blocks
  const blocks = [];
  let i = 0;

  while (i < src.length) {
    // Code block (fenced)
    if (src.slice(i, i + 3) === '```') {
      const langMatch = src.slice(i + 3).match(/^(\S*)\n/);
      const lang = langMatch ? langMatch[1] : '';
      const start = i + 3 + (langMatch ? langMatch[0].length : 1);
      const end = src.indexOf('\n```', start);
      if (end !== -1) {
        const code = src.slice(start, end);
        blocks.push(`<pre><code class="lang-${escapeHtml(lang)}">${highlightCode(code, lang.toLowerCase())}</code></pre>`);
        i = end + 4;
        continue;
      }
    }

    // Blockquote
    if (src[i] === '>' && (i === 0 || src[i - 1] === '\n')) {
      let line = '';
      let j = i + 1;
      if (src[j] === ' ') j++;
      while (j < src.length && src[j] !== '\n') {
        line += src[j];
        j++;
      }
      blocks.push(`<blockquote>${renderInline(line)}</blockquote>`);
      i = j + 1;
      continue;
    }

    // Heading
    const headingMatch = src.slice(i).match(/^(#{1,6})\s+(.+)\n/);
    if (headingMatch) {
      const level = headingMatch[1].length;
      blocks.push(`<h${level}>${renderInline(headingMatch[2])}</h${level}>`);
      i += headingMatch[0].length;
      continue;
    }

    // Horizontal rule
    if (src.slice(i).match(/^(-{3,}|\*{3,}|_{3,})\n/)) {
      blocks.push('<hr>');
      i += src.slice(i).indexOf('\n') + 1;
      continue;
    }

    // Unordered list
    if (src.slice(i).match(/^[\-\*]\s/)) {
      let items = '';
      while (i < src.length && src.slice(i).match(/^[\-\*]\s/)) {
        const lineEnd = src.indexOf('\n', i);
        const line = lineEnd === -1 ? src.slice(i + 2) : src.slice(i + 2, lineEnd);
        items += `<li>${renderInline(line)}</li>`;
        i = lineEnd === -1 ? src.length : lineEnd + 1;
      }
      blocks.push(`<ul>${items}</ul>`);
      continue;
    }

    // Ordered list
    if (src.slice(i).match(/^\d+\.\s/)) {
      let items = '';
      while (i < src.length && src.slice(i).match(/^\d+\.\s/)) {
        const lineEnd = src.indexOf('\n', i);
        const lineStart = src.indexOf(' ', i) + 1;
        const line = lineEnd === -1 ? src.slice(lineStart) : src.slice(lineStart, lineEnd);
        items += `<li>${renderInline(line)}</li>`;
        i = lineEnd === -1 ? src.length : lineEnd + 1;
      }
      blocks.push(`<ol>${items}</ol>`);
      continue;
    }

    // Table (simple detection)
    if (src.slice(i).match(/^\|.+\|/) && src.slice(i).match(/\n\|[-:|\s]+\|/)) {
      const tableEnd = src.indexOf('\n\n', i);
      const tableSrc = src.slice(i, tableEnd === -1 ? src.length : tableEnd);
      const rows = tableSrc.split('\n').filter(r => r.trim());
      let table = '<table>';
      rows.forEach((row, ri) => {
        if (row.match(/^\|[-:|\s]+\|$/)) return; // separator row
        const tag = ri === 0 ? 'th' : 'td';
        const cells = row.split('|').filter((_, ci, arr) => ci > 0 && ci < arr.length - 1);
        table += '<tr>' + cells.map(c => `<${tag}>${renderInline(c.trim())}</${tag}>`).join('') + '</tr>';
      });
      table += '</table>';
      blocks.push(table);
      i = tableEnd === -1 ? src.length : tableEnd + 2;
      continue;
    }

    // Paragraph
    const lineEnd = src.indexOf('\n', i);
    const line = lineEnd === -1 ? src.slice(i) : src.slice(i, lineEnd);
    if (line.trim()) {
      blocks.push(`<p>${renderInline(line)}</p>`);
    }
    i = lineEnd === -1 ? src.length : lineEnd + 1;
  }

  return blocks.join('\n');
}

function renderInline(text) {
  if (!text) return '';

  let result = escapeHtml(text);

  // Inline code (must be first to prevent inner processing)
  result = result.replace(/`([^`]+)`/g, '<code>$1</code>');

  // Bold + italic
  result = result.replace(/\*\*\*(.+?)\*\*\*/g, '<strong><em>$1</em></strong>');
  // Bold
  result = result.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');
  // Italic
  result = result.replace(/\*(.+?)\*/g, '<em>$1</em>');
  // Strikethrough
  result = result.replace(/~~(.+?)~~/g, '<del>$1</del>');

  // Links [text](url)
  result = result.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');
  // Auto-link URLs
  result = result.replace(/(^|[^"=])(https?:\/\/[^\s<]+)/g, '$1<a href="$2" target="_blank" rel="noopener">$2</a>');

  // Images ![alt](url)
  result = result.replace(/!\[([^\]]*)\]\(([^)]+)\)/g, '<img src="$2" alt="$1" style="max-width:100%;border-radius:var(--radius-sm);">');

  return result;
}

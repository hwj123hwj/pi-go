// API 访问层：token 管理、401 登录浮层、fetch 封装

const TOKEN_KEY = 'pi_go_api_token';

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || '';
}

export function saveToken(token) {
  if (token) {
    localStorage.setItem(TOKEN_KEY, token);
  } else {
    localStorage.removeItem(TOKEN_KEY);
  }
}

export function authHeaders(extra = {}) {
  const headers = { ...extra };
  const token = getToken();
  if (token) {
    headers['Authorization'] = 'Bearer ' + token;
  }
  return headers;
}

// wsURL 为 WebSocket 连接附加 ?token=（浏览器 WS 无法自定义 header）
export function wsURL(baseUrl, path) {
  const token = getToken();
  const suffix = token ? '?token=' + encodeURIComponent(token) : '';
  return baseUrl.replace(/^http/, 'ws') + path + suffix;
}

export class ApiError extends Error {
  constructor(message, status) {
    super(message);
    this.status = status;
  }
}

async function parseError(res) {
  try {
    const data = await res.clone().json();
    if (data && data.error) return data.error;
  } catch (e) { /* not json */ }
  return res.statusText || ('HTTP ' + res.status);
}

// request 执行一次 API 调用；401 时弹出登录浮层并抛出 ApiError。
async function request(method, path, body, opts = {}) {
  const headers = authHeaders(body !== undefined ? { 'Content-Type': opts.contentType || 'application/json' } : {});
  const res = await fetch(path, {
    method,
    headers,
    body: body === undefined ? undefined : (typeof body === 'string' ? body : JSON.stringify(body)),
  });
  if (res.status === 401) {
    showLogin();
    throw new ApiError('unauthorized', 401);
  }
  if (!res.ok) {
    throw new ApiError(await parseError(res), res.status);
  }
  const ct = res.headers.get('Content-Type') || '';
  if (ct.includes('application/json')) {
    return res.json();
  }
  return res.text();
}

export const api = {
  get: (path) => request('GET', path),
  post: (path, body) => request('POST', path, body ?? {}),
  postRaw: (path, raw, contentType) => request('POST', path, raw, { contentType }),
  put: (path, body) => request('PUT', path, body),
  del: (path) => request('DELETE', path, {}),
};

let overlayEl = null;

// showLogin 弹出登录浮层；保存后重载页面让各模块用新 token 重新初始化。
export function showLogin() {
  if (overlayEl) {
    overlayEl.hidden = false;
    document.getElementById('nav-auth').textContent = '未登录';
    return;
  }
  overlayEl = document.getElementById('login-overlay');
  const input = document.getElementById('login-token');
  const err = document.getElementById('login-error');

  document.getElementById('login-save-btn').onclick = async () => {
    saveToken(input.value.trim());
    err.textContent = '';
    overlayEl.hidden = true;
    document.getElementById('nav-auth').textContent = '已登录';
    // 验证新 token
    try {
      await api.get('/sessions');
      window.location.reload();
    } catch (e) {
      overlayEl.hidden = false;
      err.textContent = '令牌无效：' + e.message;
    }
  };
  document.getElementById('login-skip-btn').onclick = () => {
    saveToken('');
    overlayEl.hidden = true;
    window.location.reload();
  };
  overlayEl.hidden = false;
  input.focus();
}

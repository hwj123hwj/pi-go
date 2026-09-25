// 工作流台：表单编辑器（默认）/ YAML 高级模式、模板、运行列表、详情轮询、审批操作

import { api } from './api.js';

const ACTIVE_STATUSES = new Set(['running', 'waiting_approval']);

const EXAMPLE_YAML = `name: demo-pipeline
vars:
  topic: Go context
steps:
  - id: outline
    prompt: "为「{{vars.topic}}」写一份 5 点研究大纲"
  - id: expand
    foreach: ["要点1", "要点2", "要点3"]
    concurrency: 2
    retries: 1
    prompt: "展开要点「{{item}}」，给出 3 条细节"
    depends_on: [outline]
  - id: merge
    prompt: "把以下内容整理成短文：\\n{{steps.expand.output}}"
    depends_on: [expand]
  - id: publish
    prompt: "最终确认并输出全文：\\n{{steps.merge.output}}"
    confirm: true
    depends_on: [merge]
`;

// 内置模板：企业常用场景，一键载入表单微调即可运行
const TEMPLATES = [
  {
    label: '研究报告',
    form: {
      name: 'research-report',
      vars: [{ k: 'topic', v: '输入研究主题' }],
      steps: [
        { id: 'outline', prompt: '围绕「{{vars.topic}}」列出研究大纲', model: '', retries: 0, confirm: false, foreach: '' },
        { id: 'research', prompt: '深入研究维度「{{item}}」，给出要点、证据与结论', foreach: '技术方案, 成本, 风险', retries: 1, model: '', confirm: false },
        { id: 'merge', prompt: '把以下研究内容整理成结构化报告：\n\n{{steps.research.output}}', model: '', retries: 0, confirm: false, foreach: '' },
        { id: 'publish', prompt: '这是最终报告，请通读并输出定稿：\n\n{{steps.merge.output}}', model: '', retries: 0, confirm: true, foreach: '' },
      ],
    },
  },
  {
    label: '批量翻译',
    form: {
      name: 'batch-translate',
      vars: [{ k: 'target_lang', v: '英文' }],
      steps: [
        { id: 'translate', prompt: '把下面内容翻译成{{vars.target_lang}}，只输出译文：\n{{item}}', foreach: '第一段原文, 第二段原文, 第三段原文', retries: 1, model: '', confirm: false },
        { id: 'assemble', prompt: '以下是各段译文，按顺序合并输出，不要额外说明：\n\n{{steps.translate.output}}', model: '', retries: 0, confirm: false, foreach: '' },
      ],
    },
  },
  {
    label: '内容创作',
    form: {
      name: 'content-pipeline',
      vars: [{ k: 'topic', v: '输入主题' }],
      steps: [
        { id: 'draft', prompt: '为主题「{{vars.topic}}」写一篇短文初稿', model: '', retries: 0, confirm: false, foreach: '' },
        { id: 'review', prompt: '审阅下面这篇文章，列出问题并直接给出修改后的全文：\n\n{{steps.draft.output}}', model: '', retries: 0, confirm: false, foreach: '' },
        { id: 'finalize', prompt: '定稿发布以下内容：\n\n{{steps.review.output}}', model: '', retries: 0, confirm: true, foreach: '' },
      ],
    },
  },
];

// ─── YAML 生成：表单 → YAML（JSON 字符串转义在 YAML 双引号标量下合法）───────

function yamlScalar(s) {
  s = String(s ?? '');
  if (s === '') return '""';
  if (/^[A-Za-z0-9_.\- ]+$/.test(s) && s.trim() === s) return s;
  return JSON.stringify(s);
}

function yamlNum(s) {
  return /^-?\d+(\.\d+)?$/.test(String(s).trim()) ? String(s).trim() : null;
}

function formToYAML(f) {
  const lines = [`name: ${yamlScalar(f.name)}`];
  const vars = f.vars.filter(v => v.k.trim() !== '');
  if (vars.length) {
    lines.push('vars:');
    for (const v of vars) {
      const n = yamlNum(v.v);
      lines.push(`  ${yamlScalar(v.k.trim())}: ${n !== null ? n : yamlScalar(v.v)}`);
    }
  }
  lines.push('steps:');
  for (const s of f.steps) {
    lines.push(`  - id: ${yamlScalar(s.id || 'step')}`);
    lines.push(`    prompt: ${JSON.stringify(s.prompt)}`);
    const fe = (s.foreach || '').trim();
    if (fe) {
      // 逗号分隔且不含模板语法 → 内联列表；否则按变量名/模板输出
      if (!fe.includes('{{') && !fe.includes('\n') && fe.includes(',')) {
        const items = fe.split(',').map(x => x.trim()).filter(Boolean);
        lines.push(`    foreach: [${items.map(yamlScalar).join(', ')}]`);
      } else {
        lines.push(`    foreach: ${yamlScalar(fe)}`);
      }
    }
    if (s.model && s.model.trim()) lines.push(`    model: ${yamlScalar(s.model.trim())}`);
    if (s.retries > 0) lines.push(`    retries: ${s.retries}`);
    if (s.confirm) lines.push(`    confirm: true`);
  }
  return lines.join('\n') + '\n';
}

// ─── 页面 ────────────────────────────────────────────────────────────────────

function el(tag, className, text) {
  const e = document.createElement(tag);
  if (className) e.className = className;
  if (text !== undefined) e.textContent = text;
  return e;
}

export class WorkflowsPage {
  constructor(state) {
    this.state = state;
    this.mode = 'form';
    this.runs = [];
    this.currentRunID = null;
    this.pollTimer = null;
    this.listTimer = null;
    this.form = this.blankForm();

    this.$yaml = document.getElementById('wf-yaml');
    this.$preview = document.getElementById('wf-yaml-preview');
    this.$form = document.getElementById('wf-form');
    this.$submit = document.getElementById('wf-submit-btn');
    this.$submitMsg = document.getElementById('wf-submit-msg');
    this.$runList = document.getElementById('wf-run-list');
    this.$detail = document.getElementById('wf-detail');
    this.$events = document.getElementById('wf-events');
    this.$steps = document.getElementById('wf-steps');
    this.$outputs = document.getElementById('wf-outputs');

    // 模式切换
    document.getElementById('wf-mode-form').onclick = () => this.setMode('form');
    document.getElementById('wf-mode-yaml').onclick = () => this.setMode('yaml');
    document.getElementById('wf-preview-btn').onclick = () => this.togglePreview();

    // 模板 chips
    const chips = document.getElementById('wf-template-chips');
    for (const t of TEMPLATES) {
      const chip = el('button', 'chip', t.label);
      chip.onclick = () => { this.form = JSON.parse(JSON.stringify(t.form)); this.renderForm(); this.hidePreview(); };
      chips.appendChild(chip);
    }

    this.$submit.onclick = () => this.submit();
    document.getElementById('wf-refresh-btn').onclick = () => this.loadRuns();
    document.getElementById('wf-close-btn').onclick = () => this.closeDetail();
    document.getElementById('wf-approve-btn').onclick = () => this.gate('/approve');
    document.getElementById('wf-reject-btn').onclick = () => this.gate('/reject');
    document.getElementById('wf-cancel-btn').onclick = () => this.cancelRun();

    this.renderForm();
  }

  blankForm() {
    return {
      name: 'my-pipeline',
      vars: [{ k: 'topic', v: '你好' }],
      steps: [
        { id: 'draft', prompt: '写一个关于 {{vars.topic}} 的大纲', model: '', retries: 0, confirm: false, foreach: '' },
        { id: 'finalize', prompt: '基于以下大纲输出全文：\n{{steps.draft.output}}', model: '', retries: 0, confirm: false, foreach: '' },
      ],
    };
  }

  setMode(mode) {
    this.mode = mode;
    document.getElementById('wf-mode-form').classList.toggle('active', mode === 'form');
    document.getElementById('wf-mode-yaml').classList.toggle('active', mode === 'yaml');
    this.$form.hidden = mode !== 'form';
    this.$yaml.hidden = mode !== 'yaml';
    if (mode === 'yaml' && !this.$yaml.value.trim()) {
      this.$yaml.value = formToYAML(this.form);
    }
    this.hidePreview();
  }

  togglePreview() {
    if (!this.$preview.hidden) { this.hidePreview(); return; }
    this.$preview.textContent = this.currentYAML();
    this.$preview.hidden = false;
    document.getElementById('wf-preview-btn').textContent = '隐藏预览';
  }

  hidePreview() {
    this.$preview.hidden = true;
    document.getElementById('wf-preview-btn').textContent = '预览 YAML';
  }

  currentYAML() {
    return this.mode === 'form' ? formToYAML(this.form) : this.$yaml.value;
  }

  // ── 表单渲染 ──────────────────────────────────────────────────────────

  renderForm() {
    this.$form.innerHTML = '';
    const f = this.form;

    // 名称
    const nameRow = el('div', 'wf-field-row');
    nameRow.append(el('label', 'wf-label', '名称'));
    const nameInput = el('input', 'wf-input');
    nameInput.value = f.name;
    nameInput.oninput = () => { f.name = nameInput.value; this.hidePreview(); };
    nameRow.appendChild(nameInput);
    this.$form.appendChild(nameRow);

    // 变量
    const varsHead = el('div', 'wf-group-head');
    varsHead.append(el('span', 'wf-label', '变量'), el('span', 'wf-hint', '提示词里用 {{vars.名称}} 引用'));
    this.$form.appendChild(varsHead);
    f.vars.forEach((v, i) => {
      this.$form.appendChild(this.varRow(v, i));
    });
    const addVar = el('button', 'chip', '+ 变量');
    addVar.onclick = () => { f.vars.push({ k: '', v: '' }); this.renderForm(); };
    this.$form.appendChild(addVar);

    // 步骤
    const stepsHead = el('div', 'wf-group-head');
    stepsHead.append(el('span', 'wf-label', '步骤'), el('span', 'wf-hint', '从上到下依次执行，{{steps.上一步id.output}} 引用产出'));
    this.$form.appendChild(stepsHead);
    f.steps.forEach((s, i) => {
      this.$form.appendChild(this.stepCard(s, i));
    });
    const addStep = el('button', 'chip', '+ 步骤');
    addStep.onclick = () => {
      f.steps.push({ id: 'step' + (f.steps.length + 1), prompt: '', model: '', retries: 0, confirm: false, foreach: '' });
      this.renderForm();
    };
    this.$form.appendChild(addStep);
  }

  varRow(v, i) {
    const row = el('div', 'wf-var-row');
    const k = el('input', 'wf-input wf-var-k');
    k.placeholder = '名称';
    k.value = v.k;
    k.oninput = () => { v.k = k.value; this.hidePreview(); };
    const eq = el('span', 'wf-var-eq', '=');
    const val = el('input', 'wf-input wf-var-v');
    val.placeholder = '值';
    val.value = v.v;
    val.oninput = () => { v.v = val.value; this.hidePreview(); };
    const del = el('button', 'wf-icon-btn', '×');
    del.title = '删除变量';
    del.onclick = () => { this.form.vars.splice(i, 1); this.renderForm(); };
    row.append(k, eq, val, del);
    return row;
  }

  stepCard(s, i) {
    const f = this.form;
    const card = el('div', 'wf-step-card');

    const head = el('div', 'wf-step-head');
    head.append(el('span', 'wf-step-num', String(i + 1)));
    const idInput = el('input', 'wf-input wf-step-id-input');
    idInput.value = s.id;
    idInput.oninput = () => { s.id = idInput.value; this.hidePreview(); };
    head.appendChild(idInput);
    const moveUp = el('button', 'wf-icon-btn', '↑');
    moveUp.title = '上移';
    moveUp.disabled = i === 0;
    moveUp.onclick = () => { [f.steps[i - 1], f.steps[i]] = [f.steps[i], f.steps[i - 1]]; this.renderForm(); };
    const moveDown = el('button', 'wf-icon-btn', '↓');
    moveDown.title = '下移';
    moveDown.disabled = i === f.steps.length - 1;
    moveDown.onclick = () => { [f.steps[i + 1], f.steps[i]] = [f.steps[i], f.steps[i + 1]]; this.renderForm(); };
    const del = el('button', 'wf-icon-btn', '×');
    del.title = '删除步骤';
    del.onclick = () => { f.steps.splice(i, 1); this.renderForm(); };
    head.append(moveUp, moveDown, del);
    card.appendChild(head);

    const prompt = el('textarea', 'wf-input wf-prompt');
    prompt.placeholder = '这一步让 AI 做什么…（支持 {{vars.x}}、{{item}}、{{steps.其他步骤.output}}）';
    prompt.value = s.prompt;
    prompt.rows = 2;
    prompt.oninput = () => { s.prompt = prompt.value; this.hidePreview(); };
    card.appendChild(prompt);

    const opts = el('div', 'wf-step-opts');
    const feWrap = el('label', 'wf-opt');
    feWrap.append(el('span', 'wf-opt-label', '批量'));
    const fe = el('input', 'wf-input wf-opt-input');
    fe.placeholder = '逗号分隔的列表 / 变量名，留空=单步';
    fe.value = s.foreach;
    fe.oninput = () => { s.foreach = fe.value; this.hidePreview(); };
    feWrap.appendChild(fe);
    opts.appendChild(feWrap);

    const modelWrap = el('label', 'wf-opt');
    modelWrap.append(el('span', 'wf-opt-label', '模型'));
    const model = el('input', 'wf-input wf-opt-input wf-opt-model');
    model.placeholder = '默认';
    model.value = s.model;
    model.oninput = () => { s.model = model.value; this.hidePreview(); };
    modelWrap.appendChild(model);
    opts.appendChild(modelWrap);

    const retryWrap = el('label', 'wf-opt');
    retryWrap.append(el('span', 'wf-opt-label', '重试'));
    const retry = el('input', 'wf-input wf-opt-retry');
    retry.type = 'number';
    retry.min = '0'; retry.max = '10';
    retry.value = s.retries;
    retry.oninput = () => { s.retries = parseInt(retry.value, 10) || 0; this.hidePreview(); };
    retryWrap.appendChild(retry);
    opts.appendChild(retryWrap);

    const confirmWrap = el('label', 'wf-opt wf-opt-check');
    const confirm = el('input');
    confirm.type = 'checkbox';
    confirm.checked = s.confirm;
    confirm.onchange = () => { s.confirm = confirm.checked; this.hidePreview(); };
    confirmWrap.append(confirm, el('span', 'wf-opt-label', '运行前需人工确认'));
    opts.appendChild(confirmWrap);

    card.appendChild(opts);
    return card;
  }

  validateForm() {
    const f = this.form;
    if (!f.name.trim()) return '工作流名称不能为空';
    if (!f.steps.length) return '至少需要一个步骤';
    for (const s of f.steps) {
      if (!s.prompt.trim()) return `步骤「${s.id || '未命名'}」的提示词不能为空`;
    }
    return null;
  }

  // ── 提交 ─────────────────────────────────────────────────────────────

  async submit() {
    const yaml = this.currentYAML();
    if (!yaml.trim()) {
      this.$submitMsg.textContent = '请先填写工作流';
      return;
    }
    if (this.mode === 'form') {
      const err = this.validateForm();
      if (err) {
        this.$submitMsg.textContent = err;
        return;
      }
    }
    this.$submit.disabled = true;
    this.$submitMsg.textContent = '';
    try {
      const res = await api.postRaw('/workflows', yaml, 'text/yaml');
      this.$submitMsg.textContent = '已启动 ' + res.run_id;
      await this.loadRuns();
      this.showRun(res.run_id);
    } catch (e) {
      this.$submitMsg.textContent = '失败：' + e.message;
    } finally {
      this.$submit.disabled = false;
    }
  }

  // ── 运行列表与详情 ────────────────────────────────────────────────────

  activate() {
    this.loadRuns();
    clearInterval(this.listTimer);
    this.listTimer = setInterval(() => this.loadRuns(), 5000);
  }

  deactivate() {
    clearInterval(this.listTimer);
  }

  async loadRuns() {
    let runs;
    try {
      const res = await api.get('/workflows');
      runs = res.runs || [];
    } catch (e) {
      return; // 401 已由 api 层处理
    }
    this.runs = runs;
    this.$runList.innerHTML = '';
    if (!runs.length) {
      this.$runList.innerHTML = '<div class="wf-empty">还没有运行记录，从左侧提交或选个模板开始</div>';
      return;
    }
    for (const run of runs) {
      const row = el('div', 'wf-run-row' + (run.id === this.currentRunID ? ' selected' : ''));
      row.innerHTML = `
        <span class="badge badge-${run.status}">${run.status}</span>
        <span class="wf-run-name">${escapeHTML(run.name)}</span>
        <span class="wf-run-id">${run.id}</span>
        <span class="wf-run-time">${formatTime(run.started_at)}</span>`;
      row.onclick = () => this.showRun(run.id);
      this.$runList.appendChild(row);
    }
  }

  async showRun(runID) {
    this.currentRunID = runID;
    this.$detail.hidden = false;
    await this.refreshRun();
    clearInterval(this.pollTimer);
    this.pollTimer = setInterval(() => this.refreshRun(), 1500);
  }

  closeDetail() {
    this.currentRunID = null;
    this.$detail.hidden = true;
    clearInterval(this.pollTimer);
    this.loadRuns();
  }

  async refreshRun() {
    if (!this.currentRunID) return;
    let meta, events;
    try {
      const res = await api.get('/workflows/' + this.currentRunID);
      meta = res.meta;
      events = res.events || [];
    } catch (e) {
      if (e.status === 404) this.closeDetail();
      return;
    }

    document.getElementById('wf-detail-title').textContent = `${meta.name} · ${meta.id}`;
    this.renderActions(meta.status);
    this.renderSteps(meta);
    this.renderOutputs(meta.outputs || {});
    this.renderEvents(events);

    if (!ACTIVE_STATUSES.has(meta.status)) {
      clearInterval(this.pollTimer);
    }
  }

  renderActions(status) {
    const waiting = status === 'waiting_approval';
    const active = ACTIVE_STATUSES.has(status);
    document.getElementById('wf-approve-btn').hidden = !waiting;
    document.getElementById('wf-reject-btn').hidden = !waiting;
    document.getElementById('wf-cancel-btn').hidden = !active;
  }

  renderSteps(meta) {
    const order = Object.keys(meta.steps || {});
    this.$steps.innerHTML = '';
    for (const id of order) {
      const s = meta.steps[id];
      const items = s.items_total ? ` (${s.items_done}/${s.items_total})` : '';
      const row = el('div', 'wf-step-row');
      row.innerHTML = `
        <span class="badge badge-${s.status}">${s.status}${items}</span>
        <span class="wf-step-id">${escapeHTML(id)}</span>
        ${s.cached ? '<span class="wf-tag">cached</span>' : ''}
        ${s.error ? `<span class="wf-step-error">${escapeHTML(s.error)}</span>` : ''}`;
      this.$steps.appendChild(row);
    }
  }

  renderOutputs(outputs) {
    this.$outputs.innerHTML = '';
    const ids = Object.keys(outputs);
    if (!ids.length) {
      this.$outputs.innerHTML = '<div class="wf-empty">暂无产出</div>';
      return;
    }
    for (const id of ids) {
      const block = document.createElement('details');
      block.className = 'wf-output-block';
      const summary = document.createElement('summary');
      summary.textContent = id + (outputs[id].cached ? ' (cached)' : '');
      const pre = document.createElement('pre');
      pre.textContent = outputs[id].output || '';
      block.append(summary, pre);
      this.$outputs.appendChild(block);
    }
  }

  renderEvents(events) {
    const important = events.filter(e =>
      ['run_start', 'step_end', 'item_end', 'step_error', 'gate_wait', 'gate_result', 'run_end'].includes(e.event));
    this.$events.innerHTML = '';
    for (const e of important.slice(-200)) {
      const line = el('div', 'wf-event-line wf-event-' + e.event);
      const parts = [formatTime(e.ts), e.event, e.step || ''];
      if (e.item !== undefined && e.item !== null) parts.push('#' + e.item);
      if (e.error) parts.push('⚠ ' + e.error);
      line.textContent = parts.filter(Boolean).join('  ');
      this.$events.appendChild(line);
    }
    this.$events.scrollTop = this.$events.scrollHeight;
  }

  async gate(action) {
    if (!this.currentRunID) return;
    try {
      await api.post('/workflows/' + this.currentRunID + action, {});
      await this.refreshRun();
    } catch (e) {
      alert('操作失败：' + e.message);
    }
  }

  async cancelRun() {
    if (!this.currentRunID) return;
    try {
      await api.post('/workflows/' + this.currentRunID + '/cancel', {});
      await this.refreshRun();
    } catch (e) {
      alert('取消失败：' + e.message);
    }
  }
}

function escapeHTML(s) {
  return String(s ?? '').replace(/[&<>"']/g, c => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  }[c]));
}

function formatTime(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  return isNaN(d) ? '' : d.toLocaleTimeString();
}

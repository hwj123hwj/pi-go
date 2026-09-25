// 工作流台：YAML 提交、运行列表、详情轮询、审批操作

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

export class WorkflowsPage {
  constructor(state) {
    this.state = state;
    this.runs = [];
    this.currentRunID = null;
    this.pollTimer = null;
    this.listTimer = null;

    this.$yaml = document.getElementById('wf-yaml');
    this.$submit = document.getElementById('wf-submit-btn');
    this.$submitMsg = document.getElementById('wf-submit-msg');
    this.$runList = document.getElementById('wf-run-list');
    this.$detail = document.getElementById('wf-detail');
    this.$events = document.getElementById('wf-events');
    this.$steps = document.getElementById('wf-steps');
    this.$outputs = document.getElementById('wf-outputs');

    document.getElementById('wf-example-btn').onclick = () => { this.$yaml.value = EXAMPLE_YAML; };
    document.getElementById('wf-refresh-btn').onclick = () => this.loadRuns();
    this.$submit.onclick = () => this.submit();
    document.getElementById('wf-close-btn').onclick = () => this.closeDetail();
    document.getElementById('wf-approve-btn').onclick = () => this.gate('/approve');
    document.getElementById('wf-reject-btn').onclick = () => this.gate('/reject');
    document.getElementById('wf-cancel-btn').onclick = () => this.cancelRun();
  }

  // activate 在页签切入时调用
  activate() {
    this.loadRuns();
    clearInterval(this.listTimer);
    this.listTimer = setInterval(() => this.loadRuns(), 5000);
  }

  deactivate() {
    clearInterval(this.listTimer);
  }

  async submit() {
    const yaml = this.$yaml.value;
    if (!yaml.trim()) {
      this.$submitMsg.textContent = '请填写 YAML';
      return;
    }
    this.$submit.disabled = true;
    this.$submitMsg.textContent = '';
    try {
      const res = await api.postRaw('/workflows', yaml, 'text/yaml');
      this.$submitMsg.textContent = '已启动 ' + res.run_id;
      this.$yaml.value = '';
      await this.loadRuns();
      this.showRun(res.run_id);
    } catch (e) {
      this.$submitMsg.textContent = '失败：' + e.message;
    } finally {
      this.$submit.disabled = false;
    }
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
      this.$runList.innerHTML = '<div class="wf-empty">还没有运行记录</div>';
      return;
    }
    for (const run of runs) {
      const row = document.createElement('div');
      row.className = 'wf-run-row' + (run.id === this.currentRunID ? ' selected' : '');
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
      const row = document.createElement('div');
      row.className = 'wf-step-row';
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
    // 只展示关键事件，最新的在底部，自动滚动
    const important = events.filter(e =>
      ['run_start', 'step_end', 'item_end', 'step_error', 'gate_wait', 'gate_result', 'run_end'].includes(e.event));
    this.$events.innerHTML = '';
    for (const e of important.slice(-200)) {
      const line = document.createElement('div');
      line.className = 'wf-event-line wf-event-' + e.event;
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

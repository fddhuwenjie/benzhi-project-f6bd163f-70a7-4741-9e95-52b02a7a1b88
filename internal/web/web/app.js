"use strict";

const state = { cases: [], current: null, clockTimer: null };
const $ = (selector) => document.querySelector(selector);
const $$ = (selector, root = document) => Array.from(root.querySelectorAll(selector));
const statusNames = {
  draft: "情景草拟", pending_check: "待核验", ready: "已准备", running: "首演进行中",
  remediation: "待整改", retest_ready: "待复演", retest_running: "复演进行中",
  pending_review: "待独立复核", approved: "已批准", rejected: "已拒绝"
};
const checkNames = {
  personal_protection: "个人防护", isolation_equipment: "隔离器材", evacuation_route: "疏散通道",
  communications: "通信联络", observer_assignment: "观察员分工"
};
const actionNames = { discovery: "发现", alarm: "报警", isolation: "隔离", evacuation: "疏散", control: "控制", cleanup: "清理" };
const reviewNames = { scenario_summary: "情景摘要", preflight_evidence: "开始前核验", complete_timeline: "完整时间线", deviation_evidence: "偏差证据", retest_conclusion: "定向复演结论", role_independence: "角色独立性" };

function requestID() {
  return crypto.randomUUID ? crypto.randomUUID() : `req-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

async function api(path, options = {}) {
  const response = await fetch(path, options);
  let envelope;
  try { envelope = await response.json(); } catch (_) { throw new Error(`服务返回了无效响应 (${response.status})`); }
  if (!response.ok || envelope.error) throw new Error(envelope.error?.message || `请求失败 (${response.status})`);
  if (envelope.meta?.replayed) notify("请求已按原始结果重放");
  return envelope.data;
}

function command(extra = {}) {
  return { request_id: requestID(), expected_revision: state.current?.revision || 0, actor_id: $("#actor").value.trim(), ...extra };
}

function notify(message, isError = false) {
  const box = $("#notice");
  box.textContent = message;
  box.classList.toggle("error", isError);
  box.hidden = false;
  clearTimeout(box.hideTimer);
  box.hideTimer = setTimeout(() => { box.hidden = true; }, 5000);
}

async function run(task, success) {
  try {
    const result = await task();
    if (result?.case_id) state.current = result;
    if (success) notify(success);
    await loadCases(false);
    render();
    return result;
  } catch (error) {
    notify(error.message, true);
    return null;
  }
}

async function loadCases(selectNewest = false) {
  try {
    state.cases = await api("/api/cases");
    if (selectNewest && state.cases.length) state.current = state.cases[0];
    if (state.current) {
      const fresh = state.cases.find((item) => item.case_id === state.current.case_id);
      state.current = fresh || null;
    }
    renderCaseList();
  } catch (error) { notify(error.message, true); }
}

function renderCaseList() {
  const list = $("#case-list");
  list.replaceChildren();
  for (const item of state.cases) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = `case-item ${state.current?.case_id === item.case_id ? "selected" : ""}`;
    button.innerHTML = `<strong>${escapeHTML(item.title)}</strong><small>${escapeHTML(item.building)} · ${statusNames[item.status] || item.status}</small>`;
    button.addEventListener("click", () => { state.current = item; $("#actor").value ||= item.coordinator_id; render(); });
    list.append(button);
  }
  if (!state.cases.length) list.innerHTML = '<p class="muted">暂无案件</p>';
}

function render() {
  const c = state.current;
  renderCaseList();
  $("#empty-state").hidden = Boolean(c) || !$("#create-panel").hidden;
  $("#case-summary").hidden = !c;
  if (!c) {
    for (const id of ["baseline-panel", "baseline-view", "preflight-panel", "session-panel", "deviation-panel", "review-panel", "dossier-panel"]) $(id).hidden = true;
    updateSteps(null);
    stopClock();
    return;
  }
  $("#create-panel").hidden = true;
  $("#case-title").textContent = c.title;
  $("#case-building").textContent = c.building;
  $("#case-status").textContent = statusNames[c.status] || c.status;
  $("#case-revision").textContent = String(c.revision);
  $("#case-coordinator").textContent = c.coordinator_id;
  updateSteps(c.status);
  renderBaseline(c);
  renderPreflight(c);
  renderSession(c);
  renderDeviations(c);
  renderReview(c);
  renderDossier(c);
}

function updateSteps(status) {
  const order = ["draft", "pending_check", "ready", "remediation", "retest_ready", "pending_review", "approved"];
  let current = order.indexOf(status);
  if (status === "running") current = 2;
  if (status === "retest_running") current = 4;
  if (status === "rejected") current = 6;
  $$(".status-strip span").forEach((item, index) => {
    item.classList.toggle("active", index === current);
    item.classList.toggle("done", current > index);
  });
}

function renderBaseline(c) {
  $("#baseline-panel").hidden = c.status !== "draft";
  $("#baseline-view").hidden = !c.baseline;
  if (!c.baseline) return;
  $("#baseline-digest").textContent = c.baseline.content_digest;
  const values = [
    ["泄漏物质", c.baseline.chemical_name], ["危险类别", c.baseline.hazard_class],
    ["影响区域", c.baseline.affected_zones.join("、")], ["响应角色", c.baseline.required_roles.join("、")],
    ["观测点", c.baseline.observation_points.join("、")], ["冻结时间", formatTime(c.baseline.frozen_at)]
  ];
  $("#baseline-detail").innerHTML = values.map(([label, value]) => `<div><span>${label}</span><strong>${escapeHTML(value)}</strong></div>`).join("");
}

function renderPreflight(c) {
  const visible = ["pending_check", "ready", "running", "remediation", "retest_ready", "retest_running", "pending_review", "approved", "rejected"].includes(c.status);
  $("#preflight-panel").hidden = !visible;
  if (!visible) return;
  const byItem = Object.fromEntries((c.preflight_checks || []).map((check) => [check.item, check]));
  const passed = Object.values(byItem).filter((item) => item.passed).length;
  $("#check-progress").textContent = `${passed} / 5 合格`;
  $("#check-list").innerHTML = Object.entries(checkNames).map(([item, label]) => {
    const check = byItem[item];
    const canEdit = ["pending_check", "ready"].includes(c.status);
    return `<div class="check-row" data-check="${item}"><div><strong>${label}</strong>${check ? `<div class="check-state ${check.passed ? "pass" : "fail"}">${check.passed ? "合格" : "不合格"} · ${check.validity_status || ""}</div>` : ""}</div><label>证据摘要<input value="${escapeAttr(check?.evidence_summary || "")}" ${canEdit ? "" : "disabled"}></label><label>有效截止<input type="datetime-local" value="${check?.valid_until ? check.valid_until.slice(0,16) : ""}" ${canEdit ? "required" : "disabled"}></label><div class="check-actions">${canEdit ? '<button data-result="true" type="button">合格</button><button class="fail" data-result="false" type="button">不合格</button>' : ""}</div></div>`;
  }).join("");
  $$("[data-check] button").forEach((button) => button.addEventListener("click", submitCheck));
}

async function submitCheck(event) {
  const row = event.target.closest("[data-check]");
  const evidence = row.querySelector("input").value.trim();
  if (!evidence) return notify("请填写核验证据摘要", true);
  const validUntil = row.querySelectorAll("input")[1].value;
  if (!validUntil) return notify("请填写有效截止时间", true);
  await run(() => api(`/api/cases/${state.current.case_id}/preflight`, jsonOptions(command({ item: row.dataset.check, passed: event.target.dataset.result === "true", evidence_summary: evidence, valid_until: new Date(validUntil).toISOString() }))), "核验结果已保存");
}

function renderSession(c) {
  const visible = ["ready", "running", "remediation", "retest_ready", "retest_running", "pending_review", "approved", "rejected"].includes(c.status);
  $("#session-panel").hidden = !visible;
  if (!visible) { stopClock(); return; }
  const retest = ["retest_ready", "retest_running"].includes(c.status);
  $("#session-heading").textContent = retest ? "定向复演" : "演练场次与时间线";
  const running = ["running", "retest_running"].includes(c.status);
  $("#session-start").hidden = !["ready", "retest_ready"].includes(c.status);
  $("#session-live").hidden = !running;
  const active = running ? c.sessions[c.sessions.length - 1] : null;
  const scope = active?.scope_point_ids || (retest ? c.deviations.filter((d) => d.status === "remediated").map((d) => d.observation_point_id) : c.baseline?.observation_points || []);
  $("#session-scope").textContent = `冻结范围：${scope.join("、")}`;
  if (running) {
    startClock(active.started_at);
    const recorded = new Set(active.observations.map((item) => item.point_id));
    $("#observation-point").innerHTML = scope.filter((id) => !recorded.has(id)).map((id) => `<option value="${escapeAttr(id)}">${escapeHTML(id)}</option>`).join("");
    $("#event-form button").disabled = false;
    $("#observation-form button").disabled = recorded.size === scope.length;
  } else { stopClock(); }
  const timeline = [];
  for (const session of c.sessions || []) {
    timeline.push(`<div class="timeline-item"><time>${formatTime(session.started_at)}</time><strong>${session.session_kind === "retest" ? "定向复演" : "首次演练"}</strong><span>场次启动 · 范围 ${session.scope_point_ids.join("、")}</span></div>`);
    for (const item of session.event_sequence) timeline.push(`<div class="timeline-item"><time>${formatTime(item.occurred_at)}</time><small>#${item.sequence} ${actionNames[item.action] || item.action}</small><span>${escapeHTML(item.note || "-")} ${running ? `<button class="secondary correction-trigger" data-target="event" data-sequence="${item.sequence}" type="button">更正</button>` : ""}</span></div>`);
    for (const item of session.observations) timeline.push(`<div class="timeline-item"><time>${formatTime(item.observed_at)}</time><small>观测</small><span>${escapeHTML(item.point_id)} = ${item.value} · ${escapeHTML(item.evidence_summary)} ${running ? `<button class="secondary correction-trigger" data-target="observation" data-point="${escapeAttr(item.point_id)}" type="button">更正</button>` : ""}</span></div>`);
    for (const item of session.corrections || []) {
      const detail = item.target_type === "event_note" ? `动作备注：${item.original_note || "-"} → ${item.new_note || "-"}` : `观测值：${item.original_value ?? "-"} → ${item.new_value ?? "-"}`;
      timeline.push(`<div class="timeline-item"><time>${formatTime(item.corrected_at)}</time><small>更正 #${item.sequence}</small><span>${escapeHTML(detail)} · 原因：${escapeHTML(item.reason)}</span></div>`);
    }
    if (session.ended_at) timeline.push(`<div class="timeline-item"><time>${formatTime(session.ended_at)}</time><strong>${session.outcome === "passed" ? "通过" : "未通过"}</strong><span>场次评价完成</span></div>`);
  }
  $("#timeline").innerHTML = timeline.join("");
  $$(".correction-trigger").forEach((button) => button.addEventListener("click", () => openCorrection(button)));
}

function openCorrection(button) {
  const target = button.dataset.target; const value = target === "event" ? prompt("请输入新的动作备注") : prompt("请输入新的观测值"); const reason = prompt("请输入更正原因");
  if (value === null || reason === null) return;
  const body = { session_id: state.current.sessions[state.current.sessions.length - 1].session_id, target_type: target === "event" ? "event_note" : "observation_value", reason };
  if (target === "event") body.event_sequence = Number(button.dataset.sequence), body.new_note = value; else body.point_id = button.dataset.point, body.new_value = Number(value);
  run(() => api(`/api/cases/${state.current.case_id}/sessions/corrections`, jsonOptions(command(body))), "更正已追加");
}

function startClock(startedAt) {
  stopClock();
  const tick = () => {
    const seconds = Math.max(0, Math.floor((Date.now() - new Date(startedAt).getTime()) / 1000));
    $("#session-clock").textContent = `${String(Math.floor(seconds / 60)).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`;
  };
  tick(); state.clockTimer = setInterval(tick, 1000);
}
function stopClock() { if (state.clockTimer) clearInterval(state.clockTimer); state.clockTimer = null; $("#session-clock").textContent = "00:00"; }

function renderDeviations(c) {
  const visible = (c.deviations || []).length > 0;
  $("#deviation-panel").hidden = !visible;
  if (!visible) return;
  const open = c.deviations.filter((item) => item.status === "open").length;
  $("#deviation-count").textContent = `${open} 项待处置`;
  $("#deviation-list").innerHTML = `<div class="batch-controls"><input id="batch-owner" placeholder="统一责任人"><input id="batch-due" type="datetime-local"><button id="batch-remediate" type="button">批量登记选中偏差</button><input id="deviation-filter-owner" placeholder="按责任人筛选"><select id="deviation-filter-status"><option value="">全部状态</option><option value="pending_materials">待补资料</option><option value="registered">已登记</option><option value="overdue">已逾期</option><option value="verified">已复验</option></select></div>` + c.deviations.map((item) => {
    const editable = c.status === "remediation" && item.status === "open";
    const rule = item.threshold_snapshot.rule === "lte" ? "≤" : "≥";
    return `<article class="deviation-item" data-owner="${escapeAttr(item.owner_id || "")}" data-governance="${item.governance_status || item.status}"><div class="deviation-head"><strong>${editable ? `<input type="checkbox" class="deviation-select" data-deviation="${item.deviation_id}">` : ""}${escapeHTML(item.observation_point_id)}</strong><span>实测 ${item.measured_value} / 要求 ${rule} ${item.threshold_snapshot.target} ${escapeHTML(item.threshold_snapshot.unit)} · ${item.governance_status || item.status}</span></div>${editable ? `<form class="deviation-form" data-deviation="${item.deviation_id}"><label>原因<input name="cause" required></label><label>纠正措施<input name="corrective_action" required></label><label>责任人<input name="owner_id" required></label><label>完成期限<input name="due_at" type="datetime-local" required></label><label class="evidence">证据摘要<input name="evidence_digest" required></label><button type="submit">确认处置</button></form>` : `<div class="deviation-closed">${item.status === "verified" ? "复验合格" : "整改已登记"} · ${escapeHTML(item.corrective_action || "")}</div>`}</article>`;
  }).join("");
  $$(".deviation-form").forEach((form) => form.addEventListener("submit", submitRemediation));
  $("#batch-remediate").addEventListener("click", submitBatchRemediation);
  $("#deviation-filter-owner").addEventListener("input", filterDeviations);
  $("#deviation-filter-status").addEventListener("change", filterDeviations);
}

function filterDeviations() {
  const owner = $("#deviation-filter-owner").value.trim().toLowerCase(); const status = $("#deviation-filter-status").value;
  $$(".deviation-item").forEach((item) => { item.hidden = Boolean((owner && !item.dataset.owner.toLowerCase().includes(owner)) || (status && item.dataset.governance !== status)); });
}

async function submitBatchRemediation() {
  const selected = new Set($$(".deviation-select:checked").map((item) => item.dataset.deviation)); const owner = $("#batch-owner").value.trim(); const due = $("#batch-due").value;
  if (!selected.size || !owner || !due) return notify("请选择偏差并填写统一责任人和期限", true);
  const items = $$(".deviation-form").filter((form) => selected.has(form.dataset.deviation)).map((form) => { const values = Object.fromEntries(new FormData(form)); return { deviation_id: form.dataset.deviation, cause: values.cause, corrective_action: values.corrective_action, evidence_digest: values.evidence_digest }; });
  await run(() => api(`/api/cases/${state.current.case_id}/deviations/remediate-batch`, jsonOptions(command({ owner_id: owner, due_at: new Date(due).toISOString(), items }))), "批量整改已登记");
}

async function submitRemediation(event) {
  event.preventDefault(); const form = event.currentTarget; const values = Object.fromEntries(new FormData(form));
  values.due_at = new Date(values.due_at).toISOString(); values.deviation_id = form.dataset.deviation;
  await run(() => api(`/api/cases/${state.current.case_id}/deviations/remediate`, jsonOptions(command(values))), "整改已登记");
}

function renderReview(c) { $("#review-panel").hidden = c.status !== "pending_review"; if (c.status === "pending_review") $("#review-checklist").innerHTML = Object.entries(reviewNames).map(([item, label]) => `<div class="check-row" data-review="${item}"><strong>${label}</strong><label><input type="checkbox" data-passed> 确认通过</label><input placeholder="备注"></div>`).join(""); }

function renderDossier(c) {
  $("#dossier-panel").hidden = !c.dossier;
  if (!c.dossier) return;
  $("#dossier-decision").textContent = c.dossier.decision === "approve" ? "批准" : "拒绝";
  $("#dossier-digest").textContent = c.dossier.content_digest;
  $("#dossier-manifest").textContent = JSON.stringify(c.dossier.manifest, null, 2);
  $("#verify-result").textContent = "";
}

function addThresholdRow(values = {}) {
  const row = document.createElement("div"); row.className = "threshold-row";
  row.innerHTML = `<input data-key="point_id" aria-label="观测点 ID" placeholder="观测点 ID" value="${escapeAttr(values.point_id || "")}" required><input data-key="label" aria-label="名称" placeholder="名称" value="${escapeAttr(values.label || "")}" required><select data-key="rule" aria-label="规则"><option value="lte">不高于</option><option value="gte">不低于</option></select><input data-key="target" aria-label="目标值" type="number" step="any" placeholder="目标" value="${values.target ?? ""}" required><input data-key="unit" aria-label="单位" placeholder="单位" value="${escapeAttr(values.unit || "")}"><button type="button" title="删除观测点">×</button>`;
  row.querySelector("select").value = values.rule || "lte";
  row.querySelector("button").addEventListener("click", () => row.remove());
  $("#threshold-rows").append(row);
}

function jsonOptions(body) { return { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }; }
function csv(value) { return value.split(/[，,]/).map((item) => item.trim()).filter(Boolean); }
function formatTime(value) { return value ? new Intl.DateTimeFormat("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit" }).format(new Date(value)) : "-"; }
function escapeHTML(value) { const node = document.createElement("span"); node.textContent = String(value ?? ""); return node.innerHTML; }
function escapeAttr(value) { return escapeHTML(value).replaceAll('"', "&quot;"); }

$("#new-case").addEventListener("click", () => { state.current = null; $("#create-panel").hidden = false; $("#empty-state").hidden = true; renderCaseList(); updateSteps(null); });
$$("[data-cancel]").forEach((button) => button.addEventListener("click", () => { $("#create-panel").hidden = true; render(); }));
$("#reload").addEventListener("click", async () => { await loadCases(); render(); notify("案件已刷新"); });
$("#add-threshold").addEventListener("click", () => addThresholdRow());

$("#create-form").addEventListener("submit", async (event) => {
  event.preventDefault(); const values = Object.fromEntries(new FormData(event.currentTarget));
  const body = { request_id: requestID(), expected_revision: 0, actor_id: values.coordinator_id, title: values.title, building: values.building, coordinator_id: values.coordinator_id, observer_ids: csv(values.observer_ids) };
  const created = await run(() => api("/api/cases", jsonOptions(body)), "案件已创建");
  if (created) { $("#actor").value = created.coordinator_id; event.currentTarget.reset(); }
});

$("#baseline-form").addEventListener("submit", async (event) => {
  event.preventDefault(); const values = Object.fromEntries(new FormData(event.currentTarget));
  const thresholds = $$(".threshold-row").map((row) => Object.fromEntries($$("[data-key]", row).map((input) => [input.dataset.key, input.dataset.key === "target" ? Number(input.value) : input.value.trim()])));
  if (!thresholds.length) return notify("至少配置一个量化观测点", true);
  const baseline = { chemical_name: values.chemical_name, hazard_class: values.hazard_class, affected_zones: csv(values.affected_zones), required_roles: csv(values.required_roles), observation_points: thresholds.map((item) => item.point_id), thresholds };
  await run(() => api(`/api/cases/${state.current.case_id}/baseline/freeze`, jsonOptions(command({ baseline }))), "情景与阈值已冻结");
});

$("#precheck-baseline").addEventListener("click", async () => {
  const values = Object.fromEntries(new FormData($("#baseline-form"))); const thresholds = $$(".threshold-row").map((row) => Object.fromEntries($$("[data-key]", row).map((input) => [input.dataset.key, input.dataset.key === "target" ? Number(input.value) : input.value.trim()])));
  try {
    const result = await api(`/api/cases/${state.current.case_id}/baseline/precheck`, jsonOptions(command({ baseline: { chemical_name: values.chemical_name, hazard_class: values.hazard_class, affected_zones: csv(values.affected_zones), required_roles: csv(values.required_roles), observation_points: thresholds.map((item) => item.point_id), thresholds } })));
    const box = $("#baseline-precheck-result"); box.hidden = false; box.textContent = result.precheck.issues.length ? result.precheck.issues.map((item) => `${item.field}: ${item.message}`).join("；") : `预检通过，候选摘要 ${result.precheck.candidate_digest}`;
  } catch (error) { notify(error.message, true); }
});

$("#start-session").addEventListener("click", async () => {
  const kind = state.current.status === "retest_ready" ? "retest" : "initial";
  await run(() => api(`/api/cases/${state.current.case_id}/sessions/start`, jsonOptions(command({ kind }))), "场次已启动");
});

$("#event-form").addEventListener("submit", async (event) => {
  event.preventDefault(); const values = Object.fromEntries(new FormData(event.currentTarget)); const session = state.current.sessions[state.current.sessions.length - 1];
  const result = await run(() => api(`/api/cases/${state.current.case_id}/sessions/events`, jsonOptions(command({ sequence: session.event_sequence.length + 1, action: values.action, note: values.note }))), "响应动作已记录");
  if (result) event.currentTarget.elements.note.value = "";
});

$("#observation-form").addEventListener("submit", async (event) => {
  event.preventDefault(); const values = Object.fromEntries(new FormData(event.currentTarget)); values.value = Number(values.value);
  const result = await run(() => api(`/api/cases/${state.current.case_id}/sessions/observations`, jsonOptions(command(values))), "观测值已记录");
  if (result) { event.currentTarget.elements.value.value = ""; event.currentTarget.elements.evidence_summary.value = ""; }
});

$("#finish-session").addEventListener("click", async () => {
  await run(() => api(`/api/cases/${state.current.case_id}/sessions/finish`, jsonOptions(command())), "场次已结束并完成评价");
});

$("#review-form").addEventListener("submit", async (event) => {
  event.preventDefault(); const values = Object.fromEntries(new FormData(event.currentTarget)); const decision = event.submitter.value;
  $("#actor").value = values.reviewer_id;
  const checklist = $$("[data-review]").map((row) => ({ item: row.dataset.review, passed: row.querySelector("[data-passed]").checked, note: row.querySelector("input:not([data-passed])").value }));
  await run(() => api(`/api/cases/${state.current.case_id}/review`, jsonOptions(command({ decision, review_note: values.review_note, checklist }))), decision === "approve" ? "就绪档案已封存" : "拒绝结论已冻结");
});

$("#verify-dossier").addEventListener("click", async () => {
  try {
    const result = await api(`/api/cases/${state.current.case_id}/dossier/verify`); const box = $("#verify-result");
    box.className = `verify-result ${result.valid ? "valid" : "invalid"}`;
    box.textContent = result.valid ? "校验通过：规范化清单与冻结摘要一致" : `校验失败：重新计算值为 ${result.computed_digest}`;
  } catch (error) { notify(error.message, true); }
});
$("#download-dossier").addEventListener("click", async () => { const response = await fetch(`/api/cases/${state.current.case_id}/dossier/download`); if (!response.ok) { const body = await response.json(); return notify(body.error?.message || "下载失败", true); } const blob = await response.blob(); const link = document.createElement("a"); link.href = URL.createObjectURL(blob); link.download = response.headers.get("Content-Disposition")?.match(/filename="([^"]+)/)?.[1] || "readiness-dossier.json"; link.click(); URL.revokeObjectURL(link.href); });

addThresholdRow({ point_id: "alarm_time", label: "报警用时", rule: "lte", target: 60, unit: "秒" });
addThresholdRow({ point_id: "evacuation_rate", label: "疏散完成率", rule: "gte", target: 100, unit: "%" });
loadCases(true).then(render);

(() => {
  'use strict';
  const $ = (id) => document.getElementById(id);
  const q = (sel) => document.querySelector(sel);
  const editEndpoint = '/api/project/edit';
  const esc = (value) => String(value ?? '').replace(/[&<>"']/g, (c) => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'})[c]);

  function notify(message) {
    const toast = $('toast');
    if (!toast) return;
    toast.textContent = message;
    toast.classList.add('show');
    clearTimeout(toast._projectHide);
    toast._projectHide = setTimeout(() => toast.classList.remove('show'), 2800);
  }

  function installProjectPanel() {
    const inspector = q('.inspector');
    if (!inspector || $('projectPanel')) return;
    const panel = document.createElement('section');
    panel.className = 'panel';
    panel.id = 'projectPanel';
    panel.dataset.mobileSection = 'export';
    panel.dataset.editEndpoint = editEndpoint;
    panel.innerHTML = `
      <div class="panelHead"><h2>结构记录</h2><span id="projectHistoryCount">0 个结构</span></div>
      <label>项目名称<input id="projectName" value="Untitled Project" spellcheck="false"></label>
      <div class="exportGrid"><button id="projectExportBtn" type="button">保存项目包</button><button id="projectImportBtn" type="button">打开项目包</button></div>
      <input id="projectImportFile" type="file" accept="application/vnd.tialloystudio.project+zip,.tias-project" hidden>
      <div id="revisionHistory" class="revisionHistory" aria-label="Model revision history"></div>
      <p class="micro">每次成功生成都会保存为一个结构记录。选择任意结构后可以继续修改，不会覆盖原结构。</p>`;
    inspector.insertBefore(panel, inspector.firstChild);
  }

  function setNumber(id, value) { if ($(id) && Number.isFinite(Number(value))) $(id).value = String(value); }
  function setText(id, value) { if ($(id) && value !== undefined && value !== null) $(id).value = String(value); }
  function restoreControls(req) {
    const module = String(req.module || 'random').toLowerCase();
    let navModule = module;
    if (module === 'crystal' || module === 'random' || module === 'sqs') navModule = 'random';
    if (module === 'vacancy' || module === 'substitution') navModule = 'vacancy';
    if (module === 'surface') navModule = 'surface';
    q(`.nav[data-module="${navModule}"]`)?.click();
    setText('phase', req.phase || 'alpha');
    for (const [id, key] of [['nx','nx'],['ny','ny'],['nz','nz'],['targetX','target_x'],['targetY','target_y'],['targetZ','target_z'],['aAlpha','a_alpha'],['cAlpha','c_alpha'],['aBeta','a_beta'],['seed','seed'],['sqsSteps','sqs_steps'],['sqsShells','sqs_shells'],['siteId','site_id'],['vacuum','vacuum'],['interfaceMatchLimit','interface_max_repeat'],['interfaceCandidate','interface_candidate'],['interfaceDistance','interface_distance']]) setNumber(id, req[key]);
    if ($('composition') && req.composition_wt) $('composition').value = Object.entries(req.composition_wt).filter(([element]) => element !== 'Ti').map(([element,value]) => `${element}=${value}`).join(',');
    if ($('alloyType')) $('alloyType').value = req.alloy_mode || (['random','crystal','sqs'].includes(module) ? module : 'crystal');
    if ($('defectType') && ['vacancy','substitution'].includes(module)) $('defectType').value = module;
    setText('newSpecies', req.new_species);
    if (module === 'interface') {
      const topology = ['interface_periodic_bicrystal','interface_single_slab'].includes(req.surface_preset) ? req.surface_preset : 'interface_periodic_bicrystal';
      setText('interfaceTopology', topology);
    }
    else setText('surfacePreset', req.surface_preset);
    setText('sqsBackend', req.sqs_backend || 'native'); setText('atatDistro', req.atat_distro || '');
    setNumber('atatPairCutoff', req.atat_pair_cutoff_angstrom); setNumber('atatTripletCutoff', req.atat_triplet_cutoff_angstrom); setNumber('atatRunSeconds', req.atat_run_seconds);
    $('alloyType')?.dispatchEvent(new Event('change', {bubbles:true}));
    $('phase')?.dispatchEvent(new Event('change', {bubbles:true}));
    $('interfaceTopology')?.dispatchEvent(new Event('change', {bubbles:true}));
  }

  function compositionSummary(record) {
    const counts = {};
    for (const element of record.structure?.species || []) counts[element] = (counts[element] || 0) + 1;
    return Object.entries(counts).map(([element,count]) => `${element}${count}`).join(' / ') || 'no atoms';
  }

  function moduleLabel(module) {
    return ({random:'随机 Ti 合金',crystal:'Ti 单晶',sqs:'SQS Ti 合金',vacancy:'空位',substitution:'替换原子',surface:'表面',interface:'α/β 界面'})[module] || 'Ti 合金结构';
  }

  function renderHistory(manifest) {
    const container = $('revisionHistory');
    if (!container) return;
    const revisions = (manifest.history || []).map((record, index) => ({record, number:index + 1})).reverse();
    container.innerHTML = revisions.map(({record, number}) => {
      const active = record.id === manifest.active_revision_id;
      return `<article class="revisionCard${active ? ' active' : ''}" data-revision-id="${esc(record.id)}"><header><strong>结构 ${number}${active ? ' · 当前' : ''}</strong><span>${esc(moduleLabel(record.module))}</span></header><p>${esc(compositionSummary(record))} · ${(record.structure?.species || []).length} atoms</p><div class="revisionActions"><button type="button" data-revision-select="${esc(record.id)}">查看</button><button type="button" data-revision-edit="${esc(record.id)}">修改此结构</button></div><details class="revisionMore"><summary>操作</summary><div class="revisionActions"><button type="button" data-revision-derive="vacancy" data-parent="${esc(record.id)}">生成空位</button><button type="button" data-revision-derive="substitution" data-parent="${esc(record.id)}">替换原子</button></div></details></article>`;
    }).join('') || '<p class="micro">生成模型后会出现第一个结构记录。</p>';
  }

  async function refreshProject(updateName = false) {
    const name = $('projectName')?.value.trim() || '';
    const url = `/api/project${updateName && name ? `?name=${encodeURIComponent(name)}` : ''}`;
    try {
      const response = await fetch(url, {cache:'no-store'});
      if (!response.ok) throw Error('Project status request failed');
      const manifest = await response.json();
      if ($('projectName') && (!updateName || !name)) $('projectName').value = manifest.name || 'Untitled Project';
      if ($('projectHistoryCount')) $('projectHistoryCount').textContent = `${(manifest.history || []).length} 个结构`;
      window.TiAlloyStudio?.setActiveRevision(manifest.active_revision_id || '');
      renderHistory(manifest);
      return manifest;
    } catch (error) { notify(error.message); return null; }
  }

  async function loadRevision(id) {
    const response = await fetch(`/api/project/revision?id=${encodeURIComponent(id)}`, {cache:'no-store'});
    const record = await response.json();
    if (!response.ok) throw Error(record.error || '结构读取失败');
    window.TiAlloyStudio?.showRevision(record);
    return record;
  }

  async function selectRevision(id) {
    const response = await fetch('/api/project/select', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({revision_id:id})});
    const manifest = await response.json();
    if (!response.ok) throw Error(manifest.error || '结构选择失败');
    await loadRevision(id); renderHistory(manifest); window.TiAlloyStudio?.setActiveRevision(id); notify('已切换到选定结构');
  }

  async function deriveRevision(parentID, operation) {
    const body = {parent_revision_id:parentID,operation,site_id:Number($('siteId')?.value || 0)};
    if (operation === 'substitution') body.new_species = $('newSpecies')?.value.trim() || 'Al';
    const response = await fetch('/api/project/derive', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
    const manifest = await response.json();
    if (!response.ok) throw Error(manifest.error || '派生结构失败');
    await loadRevision(manifest.active_revision_id); renderHistory(manifest); notify('已基于选定结构生成新结构');
  }

  async function downloadProject() {
    const name = $('projectName')?.value.trim() || '';
    await refreshProject(true);
    try {
      const safeName = `${(name || 'TiAlloyStudio-project').replace(/[\\/:*?"<>|]+/g,'_')}.tias-project`;
      const response = await fetch('/api/project/save', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name,suggested_name:safeName})});
      const payload = await response.json();
      if (!response.ok) throw Error(payload.error || '项目包保存失败');
      if (!payload.cancelled) notify(`项目包已保存：${payload.filename || safeName}`);
    } catch (error) { notify(error.message); }
  }

  async function importProject(file) {
    try {
      const response = await fetch('/api/project/import', {method:'POST',headers:{'Content-Type':'application/vnd.tialloystudio.project+zip'},body:file});
      const payload = await response.json();
      if (!response.ok) throw Error(payload.error || '项目包打开失败');
      const manifest = await refreshProject(false); const record = await loadRevision(manifest.active_revision_id); restoreControls(record.request || {}); notify('项目包已打开');
    } catch (error) { notify(`项目导入：${error.message}`); }
  }

  installProjectPanel();
  $('projectExportBtn')?.addEventListener('click', downloadProject);
  $('projectImportBtn')?.addEventListener('click', () => $('projectImportFile')?.click());
  $('projectImportFile')?.addEventListener('change', (event) => { const file = event.target.files?.[0]; if (file) importProject(file); event.target.value = ''; });
  $('projectName')?.addEventListener('change', () => refreshProject(true));
  $('buildBtn')?.addEventListener('click', () => setTimeout(() => refreshProject(false), 500));
  $('revisionHistory')?.addEventListener('click', async (event) => {
    const select = event.target.closest('[data-revision-select]'); const edit = event.target.closest('[data-revision-edit]'); const derive = event.target.closest('[data-revision-derive]');
    try {
      if (select) await selectRevision(select.dataset.revisionSelect);
      if (edit) { const record = await loadRevision(edit.dataset.revisionEdit); restoreControls(record.request || {}); window.TiAlloyStudio?.editFromRevision(record.id); window.TiAlloyStudio?.switchMobilePanel('model'); notify('参数已恢复。修改后重新生成，会保存为新的结构记录。'); }
      if (derive) await deriveRevision(derive.dataset.parent, derive.dataset.revisionDerive);
    } catch (error) { notify(error.message); }
  });
  refreshProject(false);
})();

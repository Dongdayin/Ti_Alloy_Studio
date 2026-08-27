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
      <div class="panelHead"><h2>Project</h2><span id="projectHistoryCount">0 versions</span></div>
      <label>Project name<input id="projectName" value="Untitled Project" spellcheck="false"></label>
      <div class="exportGrid"><button id="projectExportBtn" type="button">Save project</button><button id="projectImportBtn" type="button">Open project</button></div>
      <input id="projectImportFile" type="file" accept="application/vnd.tialloystudio.project+zip,.tias-project" hidden>
      <div id="revisionHistory" class="revisionHistory" aria-label="Model revision history"></div>
      <p class="micro">Each successful model is saved as a version. Earlier versions remain available when you continue editing.</p>`;
    inspector.insertBefore(panel, inspector.firstChild);
  }

  function setNumber(id, value) { if ($(id) && Number.isFinite(Number(value))) $(id).value = String(value); }
  function setText(id, value) { if ($(id) && value !== undefined && value !== null) $(id).value = String(value); }
  function restoreControls(req) {
    const module = String(req.module || 'random').toLowerCase();
    let navModule = module;
    if (module === 'crystal' || module === 'random') navModule = 'random';
    if (module === 'vacancy' || module === 'substitution' || module === 'surface') navModule = 'vacancy';
    q(`.nav[data-module="${navModule}"]`)?.click();
    setText('phase', req.phase || 'alpha');
    for (const [id, key] of [['nx','nx'],['ny','ny'],['nz','nz'],['targetX','target_x'],['targetY','target_y'],['targetZ','target_z'],['aAlpha','a_alpha'],['cAlpha','c_alpha'],['aBeta','a_beta'],['seed','seed'],['sqsSteps','sqs_steps'],['sqsShells','sqs_shells'],['siteId','site_id'],['vacuum','vacuum'],['interfaceMax','interface_max_repeat'],['interfaceCandidate','interface_candidate'],['interfaceDistance','interface_distance'],['eosIndex','eos_index'],['gsfeSteps','gsfe_steps'],['gsfeIndex','gsfe_index']]) setNumber(id, req[key]);
    if ($('composition') && req.composition_wt) $('composition').value = Object.entries(req.composition_wt).filter(([element]) => element !== 'Ti').map(([element,value]) => `${element}=${value}`).join(',');
    if ($('alloyType') && ['random','crystal'].includes(module)) $('alloyType').value = module;
    if ($('defectType') && ['vacancy','substitution','surface'].includes(module)) $('defectType').value = module;
    setText('newSpecies', req.new_species); setText('surfacePreset', req.surface_preset); setText('sqsBackend', req.sqs_backend || 'native'); setText('atatDistro', req.atat_distro || '');
    setNumber('atatPairCutoff', req.atat_pair_cutoff_angstrom); setNumber('atatTripletCutoff', req.atat_triplet_cutoff_angstrom); setNumber('atatRunSeconds', req.atat_run_seconds);
    if ($('eosRatios') && Array.isArray(req.eos_ratios)) $('eosRatios').value = req.eos_ratios.join(',');
    setText('gsfePreset', req.gsfe_preset);
    $('phase')?.dispatchEvent(new Event('change', {bubbles:true}));
  }

  function compositionSummary(record) {
    const counts = {};
    for (const element of record.structure?.species || []) counts[element] = (counts[element] || 0) + 1;
    return Object.entries(counts).map(([element,count]) => `${element}${count}`).join(' / ') || 'no atoms';
  }

  function moduleLabel(module) {
    return ({random:'Random alloy',crystal:'Crystal',sqs:'SQS alloy',vacancy:'Vacancy',substitution:'Atom replacement',surface:'Surface',interface:'α/β interface',eos:'EOS structures',gsfe:'GSFE structures'})[module] || 'Structure';
  }

  function renderHistory(manifest) {
    const container = $('revisionHistory');
    if (!container) return;
    const revisions = (manifest.history || []).map((record, index) => ({record, number:index + 1})).reverse();
    container.innerHTML = revisions.map(({record, number}) => {
      const active = record.id === manifest.active_revision_id;
      return `<article class="revisionCard${active ? ' active' : ''}" data-revision-id="${esc(record.id)}"><header><strong>Version ${number}${active ? ' · Current' : ''}</strong><span>${esc(moduleLabel(record.module))}</span></header><p>${esc(compositionSummary(record))} · ${(record.structure?.species || []).length} atoms</p><div class="revisionActions"><button type="button" data-revision-select="${esc(record.id)}">View</button><button type="button" data-revision-edit="${esc(record.id)}">Continue editing</button></div><details class="revisionMore"><summary>More actions</summary><div class="revisionActions"><button type="button" data-revision-derive="vacancy" data-parent="${esc(record.id)}">Create vacancy</button><button type="button" data-revision-derive="substitution" data-parent="${esc(record.id)}">Replace atom</button></div></details></article>`;
    }).join('') || '<p class="micro">Generate a model to create the first revision.</p>';
  }

  async function refreshProject(updateName = false) {
    const name = $('projectName')?.value.trim() || '';
    const url = `/api/project${updateName && name ? `?name=${encodeURIComponent(name)}` : ''}`;
    try {
      const response = await fetch(url, {cache:'no-store'});
      if (!response.ok) throw Error('Project status request failed');
      const manifest = await response.json();
      if ($('projectName') && (!updateName || !name)) $('projectName').value = manifest.name || 'Untitled Project';
      if ($('projectHistoryCount')) $('projectHistoryCount').textContent = `${(manifest.history || []).length} versions`;
      window.TiAlloyStudio?.setActiveRevision(manifest.active_revision_id || '');
      renderHistory(manifest);
      return manifest;
    } catch (error) { notify(error.message); return null; }
  }

  async function loadRevision(id) {
    const response = await fetch(`/api/project/revision?id=${encodeURIComponent(id)}`, {cache:'no-store'});
    const record = await response.json();
    if (!response.ok) throw Error(record.error || 'Revision load failed');
    window.TiAlloyStudio?.showRevision(record);
    return record;
  }

  async function selectRevision(id) {
    const response = await fetch('/api/project/select', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({revision_id:id})});
    const manifest = await response.json();
    if (!response.ok) throw Error(manifest.error || 'Revision selection failed');
    await loadRevision(id); renderHistory(manifest); window.TiAlloyStudio?.setActiveRevision(id); notify('Historical revision selected');
  }

  async function deriveRevision(parentID, operation) {
    const body = {parent_revision_id:parentID,operation,site_id:Number($('siteId')?.value || 0)};
    if (operation === 'substitution') body.new_species = $('newSpecies')?.value.trim() || 'Al';
    const response = await fetch('/api/project/derive', {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
    const manifest = await response.json();
    if (!response.ok) throw Error(manifest.error || 'Structure derivation failed');
    await loadRevision(manifest.active_revision_id); renderHistory(manifest); notify(`${operation} revision created from the selected structure`);
  }

  async function downloadProject() {
    const name = $('projectName')?.value.trim() || '';
    await refreshProject(true);
    try {
      const response = await fetch(`/api/project/export?name=${encodeURIComponent(name)}`, {cache:'no-store'});
      if (!response.ok) throw Error('Project package export failed');
      const blob = await response.blob(); const url = URL.createObjectURL(blob); const a = document.createElement('a');
      a.href = url; a.download = `${(name || 'TiAlloyStudio-project').replace(/[\\/:*?"<>|]+/g,'_')}.tias-project`; document.body.appendChild(a); a.click(); a.remove(); setTimeout(() => URL.revokeObjectURL(url), 1500); notify('Portable project package saved');
    } catch (error) { notify(error.message); }
  }

  async function importProject(file) {
    try {
      const response = await fetch('/api/project/import', {method:'POST',headers:{'Content-Type':'application/vnd.tialloystudio.project+zip'},body:file});
      const payload = await response.json();
      if (!response.ok) throw Error(payload.error || 'Project import failed');
      const manifest = await refreshProject(false); const record = await loadRevision(manifest.active_revision_id); restoreControls(record.request || {}); notify('Project package opened without duplicating its history');
    } catch (error) { notify(`Project import: ${error.message}`); }
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
      if (edit) { const record = await loadRevision(edit.dataset.revisionEdit); restoreControls(record.request || {}); window.TiAlloyStudio?.editFromRevision(record.id); window.TiAlloyStudio?.switchMobilePanel('model'); notify('Recipe restored. Change parameters and generate to create a child revision.'); }
      if (derive) await deriveRevision(derive.dataset.parent, derive.dataset.revisionDerive);
    } catch (error) { notify(error.message); }
  });
  refreshProject(false);
})();

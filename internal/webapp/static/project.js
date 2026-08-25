(() => {
  'use strict';

  const $ = (id) => document.getElementById(id);
  const q = (sel) => document.querySelector(sel);

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
    panel.innerHTML = `
      <div class="panelHead"><h2>Project / Reproducibility</h2><span id="projectHistoryCount">0 builds</span></div>
      <label>Project name<input id="projectName" value="Untitled Project" spellcheck="false"></label>
      <p id="projectIdentity" class="micro">Loading project identity…</p>
      <div class="exportGrid">
        <button id="projectExportBtn" type="button">Export project.json</button>
        <button id="projectImportBtn" type="button">Import project.json</button>
      </div>
      <input id="projectImportFile" type="file" accept="application/json,.json" hidden>
      <p class="micro">Each tracked generation records normalized parameters, random seed, validation, lineage and SHA-256 hashes for core structure exports.</p>`;
    inspector.insertBefore(panel, inspector.firstChild);
  }

  async function refreshProject(updateName = false) {
    const name = $('projectName')?.value.trim() || '';
    const url = `/api/project${updateName && name ? `?name=${encodeURIComponent(name)}` : ''}`;
    try {
      const response = await fetch(url, { cache: 'no-store' });
      if (!response.ok) throw Error('Project status request failed');
      const manifest = await response.json();
      if ($('projectName') && (!updateName || !name)) $('projectName').value = manifest.name || 'Untitled Project';
      if ($('projectIdentity')) $('projectIdentity').textContent = `UUID: ${manifest.project_uuid || '—'} · updated ${manifest.updated_at || '—'}`;
      if ($('projectHistoryCount')) $('projectHistoryCount').textContent = `${(manifest.history || []).length} builds`;
      return manifest;
    } catch (error) {
      if ($('projectIdentity')) $('projectIdentity').textContent = error.message;
      return null;
    }
  }

  async function downloadProject() {
    const name = $('projectName')?.value.trim() || '';
    await refreshProject(true);
    try {
      const response = await fetch(`/api/project/export?name=${encodeURIComponent(name)}`, { cache: 'no-store' });
      if (!response.ok) throw Error('Project export failed');
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'project.json';
      document.body.appendChild(a);
      a.click();
      a.remove();
      setTimeout(() => URL.revokeObjectURL(url), 1500);
      notify('project.json exported');
    } catch (error) {
      notify(error.message);
    }
  }

  function setNumber(id, value) {
    if ($(id) && Number.isFinite(Number(value))) $(id).value = String(value);
  }
  function setText(id, value) {
    if ($(id) && value !== undefined && value !== null) $(id).value = String(value);
  }

  function restoreControls(req) {
    const module = String(req.module || 'random').toLowerCase();
    let navModule = module;
    if (module === 'crystal' || module === 'random') navModule = 'random';
    if (module === 'vacancy' || module === 'substitution' || module === 'surface') navModule = 'vacancy';
    q(`.nav[data-module="${navModule}"]`)?.click();

    setText('phase', req.phase || 'alpha');
    setNumber('nx', req.nx);
    setNumber('ny', req.ny);
    setNumber('nz', req.nz);
    setNumber('targetX', req.target_x);
    setNumber('targetY', req.target_y);
    setNumber('targetZ', req.target_z);
    setNumber('aAlpha', req.a_alpha);
    setNumber('cAlpha', req.c_alpha);
    setNumber('aBeta', req.a_beta);
    setNumber('seed', req.seed);
    if ($('composition') && req.composition_wt) {
      $('composition').value = Object.entries(req.composition_wt)
        .filter(([e]) => e !== 'Ti')
        .map(([e, v]) => `${e}=${v}`)
        .join(',');
    }
    if ($('alloyType') && (module === 'random' || module === 'crystal')) $('alloyType').value = module;
    if ($('defectType') && ['vacancy', 'substitution', 'surface'].includes(module)) $('defectType').value = module;
    setText('newSpecies', req.new_species);
    setText('surfacePreset', req.surface_preset);
    setNumber('vacuum', req.vacuum);
    setText('sqsBackend', req.sqs_backend || 'atat');
    setNumber('sqsSteps', req.sqs_steps);
    setNumber('sqsShells', req.sqs_shells);
    setText('atatDistro', req.atat_distro || '');
    setNumber('atatPairCutoff', req.atat_pair_cutoff_angstrom);
    setNumber('atatTripletCutoff', req.atat_triplet_cutoff_angstrom);
    setNumber('atatRunSeconds', req.atat_run_seconds);
    setNumber('siteId', req.site_id);
    setNumber('interfaceMax', req.interface_max_repeat);
    setNumber('interfaceCandidate', req.interface_candidate);
    setNumber('interfaceDistance', req.interface_distance);
    if ($('eosRatios') && Array.isArray(req.eos_ratios)) $('eosRatios').value = req.eos_ratios.join(',');
    setNumber('eosIndex', req.eos_index);
    setText('gsfePreset', req.gsfe_preset);
    setNumber('gsfeSteps', req.gsfe_steps);
    setNumber('gsfeIndex', req.gsfe_index);
  }

  async function importProject(file) {
    try {
      const text = await file.text();
      const manifest = JSON.parse(text);
      const history = manifest.history || [];
      if (!history.length) throw Error('Project history is empty');
      const response = await fetch('/api/project/import', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(manifest)
      });
      const payload = await response.json();
      if (!response.ok) throw Error(payload.error || 'Project import failed');
      if ($('projectName')) $('projectName').value = manifest.name || 'Imported Project';
      restoreControls(history[history.length - 1].request || {});
      // The server has already restored the referenced state. Regenerate once
      // through the normal tracked GUI path so the visible model and controls
      // are guaranteed to match the restored recipe on this client.
      $('buildBtn')?.click();
      setTimeout(() => refreshProject(false), 900);
      notify('Project imported and recipe restored');
    } catch (error) {
      notify(`Project import: ${error.message}`);
    }
  }

  installProjectPanel();
  $('projectExportBtn')?.addEventListener('click', downloadProject);
  $('projectImportBtn')?.addEventListener('click', () => $('projectImportFile')?.click());
  $('projectImportFile')?.addEventListener('change', (e) => {
    const file = e.target.files?.[0];
    if (file) importProject(file);
    e.target.value = '';
  });
  $('projectName')?.addEventListener('change', () => refreshProject(true));
  $('buildBtn')?.addEventListener('click', () => setTimeout(() => refreshProject(false), 800));
  refreshProject(false);
})();

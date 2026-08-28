(() => {
  'use strict';

  const $ = (id) => document.getElementById(id);
  const colors = {
    Ti: '#8193a6',
    Al: '#d49b35',
    V: '#4f78ba',
    alpha: '#477fa8',
    beta: '#d48643'
  };
  const phaseMetadata = {
    alpha: {
      hint: 'α-Ti 使用 HCP 晶格参数 aα 和 cα。',
      lattice: '晶格常数 · α-Ti HCP',
      surface: 'basal_0001'
    },
    beta: {
      hint: 'β-Ti 使用 BCC 晶格参数 aβ。',
      lattice: '晶格常数 · β-Ti BCC',
      surface: '100'
    }
  };
  const excludedAnalysisMetrics = new Set([
    'seed',
    'selected_index',
    'interface_equivalence_assumed'
  ]);

  let active = 'random';
  let model = null;
  let rotX = -0.35;
  let rotY = 0.55;
  let zoom = 1;
  let panX = 0;
  let panY = 0;
  let selected = -1;
  let orthographic = true;
  let drag = null;
  let chartData = { composition: [], analysis: [] };
  let lastExportPath = '';
	let activeRevisionID = '';
	let pendingEditParentID = '';

  const num = (id) => {
    const element = $(id);
    return element ? Number(element.value) || 0 : 0;
  };
  const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;'
  })[c]);

  function toast(message) {
    const t = $('toast');
    t.textContent = message;
    t.classList.add('show');
    clearTimeout(t._hideTimer);
    t._hideTimer = setTimeout(() => t.classList.remove('show'), 2500);
  }

  function compositionInput() {
    const out = {};
    $('composition').value.split(/[;,\n]+/).forEach((entry) => {
      const p = entry.split('=');
      if (p.length === 2 && Number.isFinite(+p[1])) out[p[0].trim()] = +p[1];
    });
    return out;
  }

  function numberList(id) {
    return ($(id)?.value || '').split(/[;,\s]+/).map(Number).filter(Number.isFinite);
  }

  function requestPayload() {
    let module = active;
    if (active === 'random') module = $('alloyType').value;
    if (active === 'vacancy') module = $('defectType').value;
    const interfaceMode = module === 'interface';
    const alloyMode = $('alloyType')?.value || 'crystal';
    return {
      module,
      alloy_mode: alloyMode,
      phase: interfaceMode ? 'alpha' : currentPhase(),
      nx: num('nx'),
      ny: num('ny'),
      nz: num('nz'),
      target_x: num('targetX'),
      target_y: num('targetY'),
      target_z: num('targetZ'),
      a_alpha: num('aAlpha'),
      c_alpha: num('cAlpha'),
      a_beta: num('aBeta'),
      composition_wt: compositionInput(),
      seed: num('seed'),
      sqs_backend: $('sqsBackend')?.value || 'native',
      sqs_steps: num('sqsSteps'),
      sqs_shells: num('sqsShells'),
      atat_distro: $('atatDistro')?.value.trim() || '',
      atat_pair_cutoff_angstrom: num('atatPairCutoff'),
      atat_triplet_cutoff_angstrom: num('atatTripletCutoff'),
      atat_run_seconds: num('atatRunSeconds'),
      site_id: num('siteId'),
      new_species: $('newSpecies')?.value || 'Al',
      surface_preset: interfaceMode ? ($('interfaceTopology')?.value || 'interface_periodic_bicrystal') : $('surfacePreset').value,
      vacuum: module === 'interface' ? num('interfaceVacuum') : num('vacuum'),
      interface_max_repeat: num('interfaceMatchLimit'),
      interface_candidate: num('interfaceCandidate'),
      interface_distance: num('interfaceDistance')
    };
  }

  function currentPhase() {
    return $('phase')?.value === 'beta' ? 'beta' : 'alpha';
  }

  function syncPhaseOptions(selectId, phase, fallback) {
    const select = $(selectId);
    if (!select) return;
    let currentAllowed = false;
    let firstAllowed = '';
    Array.from(select.options).forEach((option) => {
      const optionPhase = option.dataset.phaseOption || '';
      const allowed = !optionPhase || optionPhase === phase;
      option.hidden = !allowed;
      option.disabled = !allowed;
      if (allowed && !firstAllowed) firstAllowed = option.value;
      if (allowed && option.value === select.value) currentAllowed = true;
    });
    if (!currentAllowed) select.value = fallback || firstAllowed;
  }

  function setSinglePhaseControlsVisible(visible) {
    const phaseControl = $('phaseControl');
    if (phaseControl) phaseControl.hidden = !visible;
    const phaseHint = $('phaseHint');
    if (phaseHint) phaseHint.hidden = !visible;
  }

  function updateInterfaceTopologyControls() {
    const topology = $('interfaceTopology')?.value || 'interface_periodic_bicrystal';
    const vacuumLabel = $('interfaceVacuumLabel');
    if (vacuumLabel) vacuumLabel.hidden = topology !== 'interface_single_slab';
  }

  function updatePhaseControls() {
    const phase = currentPhase();
    const meta = phaseMetadata[phase] || phaseMetadata.alpha;
    const showBothLattices = active === 'interface';
    setSinglePhaseControlsVisible(active !== 'interface');
    document.querySelectorAll('[data-phase-field]').forEach((field) => {
      field.hidden = !showBothLattices && field.dataset.phaseField !== phase;
    });
    if ($('phaseHint')) {
      $('phaseHint').textContent = showBothLattices
        ? 'α/β interface models use both α and β lattice values.'
        : meta.hint;
    }
    if ($('latticeSummary')) $('latticeSummary').textContent = showBothLattices ? '晶格常数 · α 与 β' : meta.lattice;
    if (active !== 'interface') {
      syncPhaseOptions('surfacePreset', phase, meta.surface);
    }
    updateInterfaceTopologyControls();
  }

  function updateAlloyControls() {
    const alloyMode = $('alloyType')?.value || 'crystal';
    const needsComposition = alloyMode !== 'crystal';
    if ($('compositionLabel')) $('compositionLabel').hidden = !needsComposition;
    if ($('seedLabel')) $('seedLabel').hidden = !needsComposition;
    if ($('sqsControls')) $('sqsControls').style.display = alloyMode === 'sqs' ? 'block' : 'none';
  }

  function setModule(module) {
    active = module;
    document.querySelectorAll('.nav').forEach((b) => b.classList.toggle('active', b.dataset.module === module));
    ['defectControls', 'surfaceControls', 'interfaceControls']
      .forEach((id) => { if ($(id)) $(id).style.display = 'none'; });
    if (module === 'vacancy') $('defectControls').style.display = 'block';
    if (module === 'surface') $('surfaceControls').style.display = 'block';
    if (module === 'interface') $('interfaceControls').style.display = 'block';
    if ($('operationHint')) {
      $('operationHint').textContent = module === 'random'
        ? '先生成基础模型；需要缺陷、表面或 α/β 界面时再进入对应页设置参数。不填写/不选择操作就跳过。'
        : '当前操作会基于上方设定的 Ti 合金成分、晶体结构、晶格常数和超胞生成。';
    }
    updateAlloyControls();
    updatePhaseControls();
  }

  async function build() {
    try {
      $('statusBadge').textContent = '生成中…';
	  const target = pendingEditParentID ? '/api/project/edit' : '/api/build';
	  const body = pendingEditParentID
	    ? { parent_revision_id: pendingEditParentID, request: requestPayload() }
	    : requestPayload();
      const response = await fetch(target, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
      });
      const payload = await response.json();
      if (!response.ok) throw Error(payload.error || '生成失败');
	  if (pendingEditParentID) {
		activeRevisionID = payload.active_revision_id || '';
		const revisionResponse = await fetch(`/api/project/revision?id=${encodeURIComponent(activeRevisionID)}`, { cache: 'no-store' });
		if (!revisionResponse.ok) throw Error('无法读取修改后的结构');
		showRevision(await revisionResponse.json());
		pendingEditParentID = '';
	  } else {
		model = payload;
	  }
      selected = -1;
	  if (!pendingEditParentID) render();
      $('statusBadge').textContent = '模型已生成';
    } catch (error) {
      $('statusBadge').textContent = '错误';
      toast(error.message);
    }
  }

  function norm(v) { return Math.hypot(...v); }
  function dot(a, b) { return a[0] * b[0] + a[1] * b[1] + a[2] * b[2]; }
  function angle(a, b) {
    return Math.acos(Math.max(-1, Math.min(1, dot(a, b) / (norm(a) * norm(b))))) * 180 / Math.PI;
  }
  function determinant(m) {
    return m[0][0] * (m[1][1] * m[2][2] - m[1][2] * m[2][1])
      - m[0][1] * (m[1][0] * m[2][2] - m[1][2] * m[2][0])
      + m[0][2] * (m[1][0] * m[2][1] - m[1][1] * m[2][0]);
  }

  function currentRenderMode() {
    return $('renderMode')?.value || 'element';
  }

  function currentRenderQuality() {
    return $('renderQuality')?.value || 'fast';
  }

  function mixColor(hex, target, ratio) {
    const clean = String(hex || '').replace('#', '');
    if (clean.length !== 6) return hex || '#708090';
    const a = [0, 2, 4].map((i) => parseInt(clean.slice(i, i + 2), 16));
    const b = [0, 2, 4].map((i) => parseInt(target.replace('#', '').slice(i, i + 2), 16));
    const out = a.map((v, i) => Math.round(v + (b[i] - v) * Math.max(0, Math.min(1, ratio))));
    return '#' + out.map((v) => v.toString(16).padStart(2, '0')).join('');
  }

  function currentAtomColor(index, z, zMin, zMax) {
    const species = model?.structure?.species?.[index] || '';
    const label = model?.structure?.site_labels?.[index] || '';
    const mode = currentRenderMode();
    const key = mode === 'phase' && label ? label : species;
    let base = colors[key] || colors[species] || '#708090';
    if (mode === 'depth') {
      const span = Math.max(1e-9, zMax - zMin);
      const t = (z - zMin) / span;
      base = t > 0.5 ? mixColor(base, '#ffffff', (t - 0.5) * 0.55) : mixColor(base, '#17202a', (0.5 - t) * 0.35);
    }
    return base;
  }

  function drawQualityAtom(ctx, p, radius, color, isSelected, quality) {
    const r = isSelected ? radius + 2 : radius;
    ctx.save();
    if (quality === 'publication') {
      ctx.shadowColor = 'rgba(15, 23, 42, 0.24)';
      ctx.shadowBlur = 7;
      ctx.shadowOffsetX = 1.5;
      ctx.shadowOffsetY = 2.5;
    } else {
      ctx.shadowColor = 'rgba(15, 23, 42, 0.14)';
      ctx.shadowBlur = 3;
      ctx.shadowOffsetX = 0.8;
      ctx.shadowOffsetY = 1.2;
    }
    const gradient = ctx.createRadialGradient(p.x - r * 0.35, p.y - r * 0.35, Math.max(1, r * 0.1), p.x, p.y, r);
    gradient.addColorStop(0, mixColor(color, '#ffffff', 0.72));
    gradient.addColorStop(0.55, color);
    gradient.addColorStop(1, mixColor(color, '#111827', 0.28));
    ctx.beginPath();
    ctx.arc(p.x, p.y, r, 0, Math.PI * 2);
    ctx.fillStyle = gradient;
    ctx.fill();
    ctx.shadowColor = 'transparent';
    ctx.strokeStyle = isSelected ? '#111827' : 'rgba(255,255,255,0.88)';
    ctx.lineWidth = isSelected ? 1.7 : 0.9;
    ctx.stroke();
    ctx.restore();
  }

  function drawTachyonBackdrop(ctx, w, h) {
    const background = ctx.createLinearGradient(0, 0, 0, h);
    background.addColorStop(0, '#f9fbff');
    background.addColorStop(0.58, '#eef3f8');
    background.addColorStop(1, '#e1e8f0');
    ctx.fillStyle = background;
    ctx.fillRect(0, 0, w, h);

    const vignette = ctx.createRadialGradient(w * 0.5, h * 0.42, Math.min(w, h) * 0.1, w * 0.5, h * 0.5, Math.max(w, h) * 0.62);
    vignette.addColorStop(0, 'rgba(255,255,255,0)');
    vignette.addColorStop(1, 'rgba(36,52,72,0.10)');
    ctx.fillStyle = vignette;
    ctx.fillRect(0, 0, w, h);
  }

  function drawTachyonAtom(ctx, p, radius, color, isSelected, zMin, zMax) {
    const depthSpan = Math.max(1e-9, zMax - zMin);
    const depth = (p.z - zMin) / depthSpan;
    const r = (isSelected ? radius + 2.4 : radius) * (0.96 + depth * 0.11);

    ctx.save();
    ctx.globalAlpha = 0.34;
    const ao = ctx.createRadialGradient(p.x + r * 0.18, p.y + r * 0.34, r * 0.12, p.x + r * 0.18, p.y + r * 0.34, r * 1.25);
    ao.addColorStop(0, 'rgba(15,23,42,0.22)');
    ao.addColorStop(0.55, 'rgba(15,23,42,0.10)');
    ao.addColorStop(1, 'rgba(15,23,42,0)');
    ctx.fillStyle = ao;
    ctx.beginPath();
    ctx.ellipse(p.x + r * 0.16, p.y + r * 0.42, r * 1.06, r * 0.62, 0, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();

    ctx.save();
    ctx.shadowColor = 'rgba(15,23,42,0.28)';
    ctx.shadowBlur = 8 + depth * 7;
    ctx.shadowOffsetX = 1.4;
    ctx.shadowOffsetY = 2.4;
    const lit = mixColor(color, '#ffffff', 0.66);
    const rim = mixColor(color, '#dbeafe', 0.18);
    const shade = mixColor(color, '#0f172a', 0.38);
    const gradient = ctx.createRadialGradient(p.x - r * 0.42, p.y - r * 0.44, Math.max(1, r * 0.05), p.x, p.y, r * 1.03);
    gradient.addColorStop(0, '#ffffff');
    gradient.addColorStop(0.13, lit);
    gradient.addColorStop(0.55, rim);
    gradient.addColorStop(1, shade);
    ctx.beginPath();
    ctx.arc(p.x, p.y, r, 0, Math.PI * 2);
    ctx.fillStyle = gradient;
    ctx.fill();
    ctx.shadowColor = 'transparent';
    ctx.lineWidth = isSelected ? 2 : 1;
    ctx.strokeStyle = isSelected ? '#111827' : 'rgba(255,255,255,0.88)';
    ctx.stroke();

    ctx.globalAlpha = isSelected ? 0.5 : 0.28;
    ctx.strokeStyle = 'rgba(15,23,42,0.34)';
    ctx.lineWidth = Math.max(0.7, r * 0.08);
    ctx.beginPath();
    ctx.arc(p.x + r * 0.04, p.y + r * 0.04, r * 0.88, Math.PI * 0.1, Math.PI * 1.45);
    ctx.stroke();
    ctx.restore();
  }

  function bindColorControl(inputId, key) {
    const input = $(inputId);
    if (!input) return;
    input.value = colors[key] || input.value;
    input.addEventListener('input', () => {
      colors[key] = input.value;
      draw3d();
      if (model) charts();
    });
  }

  function moduleDisplayName(module) {
    return ({
      crystal: 'Ti 单晶',
      random: '随机 Ti 合金',
      sqs: 'SQS Ti 合金',
      vacancy: '缺陷模型',
      substitution: '替换原子模型',
      surface: '表面模型',
      interface: 'α/β 界面模型'
    })[module] || 'Ti 合金模型';
  }

  function modelInfo() {
    const s = model.structure;
    const meta = s.meta || {};
    const cell = s.cell;
    const rows = [
      ['模型', moduleDisplayName(model.module)],
      ['相', meta.phase || '—'],
      ['晶体', String(meta.bravais || '—').toUpperCase()],
      ['晶胞', meta.cell_setting || '—'],
      ['原子数', s.species.length],
      ['a / b / c', cell.map(norm).map((v) => v.toFixed(4)).join(' / ') + ' Å'],
      ['α / β / γ', [
        angle(cell[1], cell[2]),
        angle(cell[0], cell[2]),
        angle(cell[0], cell[1])
      ].map((v) => v.toFixed(3) + '°').join(' / ')],
      ['体积', Math.abs(determinant(cell)).toFixed(4) + ' Å³'],
      ['PBC', s.pbc.map((v) => v ? 'T' : 'F').join(' ')]
    ];
    $('modelInfo').innerHTML = rows.map((r) => `<dt>${esc(r[0])}</dt><dd>${esc(r[1])}</dd>`).join('');
    $('structureSummary').textContent = `${s.species.length} atoms · ${meta.phase || ''} ${String(meta.bravais || '').toUpperCase()}`;
  }

  function rotatedPoint(p) {
    let [x, y, z] = p;
    let c = Math.cos(rotY);
    let s = Math.sin(rotY);
    [x, z] = [c * x + s * z, -s * x + c * z];
    c = Math.cos(rotX);
    s = Math.sin(rotX);
    [y, z] = [c * y - s * z, s * y + c * z];
    return [x, y, z];
  }

  function draw3d() {
    if (!model) return;
    const cv = $('structureCanvas');
    const ctx = cv.getContext('2d');
    const w = cv.clientWidth;
    const h = cv.clientHeight;
    const dpr = window.devicePixelRatio || 1;
    const quality = currentRenderQuality();
    const qualityScale = quality === 'tachyon' ? 2.5 : quality === 'publication' ? 2 : 1;
    cv.width = Math.max(1, Math.round(w * dpr * qualityScale));
    cv.height = Math.max(1, Math.round(h * dpr * qualityScale));
    ctx.setTransform(dpr * qualityScale, 0, 0, dpr * qualityScale, 0, 0);
    if (quality === 'tachyon') {
      drawTachyonBackdrop(ctx, w, h);
    } else {
      ctx.clearRect(0, 0, w, h);
    }

    const positions = model.structure.positions;
    if (!positions.length) return;
    const min = [Infinity, Infinity, Infinity];
    const max = [-Infinity, -Infinity, -Infinity];
    positions.forEach((p) => p.forEach((v, i) => {
      min[i] = Math.min(min[i], v);
      max[i] = Math.max(max[i], v);
    }));
    const center = min.map((v, i) => (v + max[i]) / 2);
    const extent = Math.max(...max.map((v, i) => v - min[i]), 1);
    const baseScale = 0.72 * Math.min(w, h) / extent * zoom;
    const radius = Number($('atomRadius')?.value || 5);

    cv._pts = positions.map((p, i) => {
      const r = rotatedPoint(p.map((v, k) => v - center[k]));
      const perspective = orthographic ? 1 : 1 / Math.max(0.35, 1 + r[2] / (extent * 3));
      return {
        x: w / 2 + panX + r[0] * baseScale * perspective,
        y: h / 2 + panY - r[1] * baseScale * perspective,
        z: r[2],
        i,
        radius: Math.max(2, radius * perspective)
      };
    }).sort((a, b) => a.z - b.z);
    const zMin = Math.min(...cv._pts.map((p) => p.z));
    const zMax = Math.max(...cv._pts.map((p) => p.z));

    for (const p of cv._pts) {
      const color = currentAtomColor(p.i, p.z, zMin, zMax);
      if (quality === 'fast') {
        ctx.beginPath();
        ctx.arc(p.x, p.y, p.i === selected ? p.radius + 2 : p.radius, 0, Math.PI * 2);
        ctx.fillStyle = color;
        ctx.fill();
        ctx.strokeStyle = p.i === selected ? '#111' : '#fff';
        ctx.lineWidth = p.i === selected ? 1.5 : 1;
        ctx.stroke();
      } else if (quality === 'tachyon') {
        drawTachyonAtom(ctx, p, p.radius, color, p.i === selected, zMin, zMax);
      } else {
        drawQualityAtom(ctx, p, p.radius, color, p.i === selected, quality);
      }
    }

    const phaseMode = currentRenderMode() === 'phase' && model.structure.site_labels?.length;
    const labels = phaseMode
      ? model.structure.site_labels
      : model.structure.species;
    const elements = [...new Set(labels)];
    $('legend').innerHTML = elements
      .map((e) => `<span><i style="background:${colors[e] || '#708090'}"></i>${esc(e)}</span>`)
      .join('');
  }

  function atomAt(x, y) {
    let best = null;
    let bestD2 = 180;
    for (const p of $('structureCanvas')._pts || []) {
      const d2 = (p.x - x) ** 2 + (p.y - y) ** 2;
      if (d2 < bestD2) {
        bestD2 = d2;
        best = p;
      }
    }
    return best;
  }

  function setup3dInteraction() {
    const cv = $('structureCanvas');
    cv.addEventListener('wheel', (e) => {
      e.preventDefault();
      zoom = Math.max(0.2, Math.min(5, zoom * Math.exp(-e.deltaY * 0.001)));
      draw3d();
    }, { passive: false });

    cv.addEventListener('pointerdown', (e) => {
      drag = { x: e.clientX, y: e.clientY, shift: e.shiftKey, moved: false };
      cv.setPointerCapture(e.pointerId);
    });

    cv.addEventListener('pointermove', (e) => {
      const rect = cv.getBoundingClientRect();
      const atom = atomAt(e.clientX - rect.left, e.clientY - rect.top);
      const tip = $('atomTooltip');
      if (drag) {
        const dx = e.clientX - drag.x;
        const dy = e.clientY - drag.y;
        if (Math.abs(dx) + Math.abs(dy) > 1) drag.moved = true;
        if (drag.shift || e.shiftKey) {
          panX += dx;
          panY += dy;
        } else {
          rotY += dx * 0.008;
          rotX += dy * 0.008;
        }
        drag.x = e.clientX;
        drag.y = e.clientY;
        draw3d();
        tip.style.display = 'none';
      } else if (atom) {
        const p = model.structure.positions[atom.i];
        tip.innerHTML = `#${atom.i} · ${esc(model.structure.species[atom.i])}<br>Cartesian: ${p.map((v) => v.toFixed(5)).join(', ')} Å`;
        tip.style.display = 'block';
        tip.style.left = `${e.clientX - rect.left + 10}px`;
        tip.style.top = `${e.clientY - rect.top + 10}px`;
      } else {
        tip.style.display = 'none';
      }
    });

    cv.addEventListener('pointerleave', () => {
      $('atomTooltip').style.display = 'none';
    });

    cv.addEventListener('pointerup', (e) => {
      const rect = cv.getBoundingClientRect();
      const atom = atomAt(e.clientX - rect.left, e.clientY - rect.top);
      if (atom && (!drag || !drag.moved)) {
        selected = atom.i;
        $('atomSelection').textContent =
          `Atom #${atom.i} · ${model.structure.species[atom.i]} · ${model.structure.positions[atom.i].map((v) => v.toFixed(6)).join(', ')} Å`;
        draw3d();
      }
      drag = null;
    });
  }

  function exportStructurePNG() {
    if (!model) {
      toast('请先生成模型');
      return;
    }
    const canvas = $('structureCanvas');
    draw3d();
    const a = document.createElement('a');
    a.href = canvas.toDataURL('image/png');
    a.download = currentRenderQuality() === 'tachyon' ? 'TiAlloyStudio-tachyon-style.png' : 'TiAlloyStudio-structure.png';
    document.body.appendChild(a);
    a.click();
    a.remove();
    toast('PNG 已导出');
  }

  function validation() {
    $('validationStatus').textContent = model.validation.status;
    $('validationPanel').innerHTML = (model.validation.checks || []).map((c) =>
      `<div class="check ${c.status}"><b>${c.status}</b><div><strong>${esc(c.name)}</strong><p>${esc(c.message)}${Number.isFinite(c.value) ? ' · ' + Number(c.value).toPrecision(7) : ''}</p></div></div>`
    ).join('');
  }

  function engines() {
    $('enginePanel').innerHTML = (model.engines || []).map((r) =>
      `<div class="diagnosticRow ${esc(r.status)}"><strong>${esc(r.name)}</strong><span>${esc(r.status)}</span></div>`
    ).join('');
  }

  function table(rows) {
    return `<table>${rows.map((r) =>
      '<tr>' + r.map((v, i) => `<${i ? 'td' : 'th'}>${esc(v)}</${i ? 'td' : 'th'}>`).join('') + '</tr>'
    ).join('')}</table>`;
  }

  function canvasSize(canvas) {
    const w = Math.max(120, canvas.clientWidth || 300);
    const h = Math.max(100, canvas.clientHeight || 160);
    const dpr = window.devicePixelRatio || 1;
    canvas.width = Math.round(w * dpr);
    canvas.height = Math.round(h * dpr);
    const ctx = canvas.getContext('2d');
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    return { ctx, w, h };
  }

  function axisRange(values) {
    let min = Math.min(...values);
    let max = Math.max(...values);
    if (!Number.isFinite(min) || !Number.isFinite(max)) return [0, 1];
    if (Math.abs(max - min) < 1e-14) {
      const pad = Math.max(Math.abs(max) * 0.05, 1);
      min -= pad;
      max += pad;
    } else {
      const pad = (max - min) * 0.08;
      min -= pad;
      max += pad;
    }
    return [min, max];
  }

  function drawAxes(ctx, w, h, box, xLabel, yLabel, xMin, xMax, yMin, yMax) {
    ctx.clearRect(0, 0, w, h);
    ctx.strokeStyle = '#cbd4df';
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(box.left, box.top);
    ctx.lineTo(box.left, box.bottom);
    ctx.lineTo(box.right, box.bottom);
    ctx.stroke();
    ctx.fillStyle = '#637184';
    ctx.font = '11px Segoe UI, Arial, sans-serif';
    ctx.textAlign = 'center';
    ctx.fillText(xLabel, (box.left + box.right) / 2, h - 3);
    ctx.save();
    ctx.translate(9, (box.top + box.bottom) / 2);
    ctx.rotate(-Math.PI / 2);
    ctx.fillText(yLabel, 0, 0);
    ctx.restore();
    ctx.textAlign = 'right';
    ctx.fillText(yMax.toPrecision(4), box.left - 5, box.top + 4);
    ctx.fillText(yMin.toPrecision(4), box.left - 5, box.bottom);
    ctx.textAlign = 'left';
    ctx.fillText(xMin.toPrecision(4), box.left, box.bottom + 12);
    ctx.textAlign = 'right';
    ctx.fillText(xMax.toPrecision(4), box.right, box.bottom + 12);
  }

  function drawBarChart(canvas, points, xLabel, yLabel, tooltipId) {
    const { ctx, w, h } = canvasSize(canvas);
    const box = { left: 46, right: w - 10, top: 10, bottom: h - 30 };
    ctx.clearRect(0, 0, w, h);
    if (!points.length) return;
    const yMax = Math.max(1, ...points.map((p) => p.y)) * 1.08;
    drawAxes(ctx, w, h, box, xLabel, yLabel, 0, Math.max(1, points.length - 1), 0, yMax);
    const span = Math.max(1, box.right - box.left);
    const slot = span / points.length;
    const barWidth = Math.max(4, slot * 0.62);
    canvas._chartPoints = [];
    points.forEach((p, i) => {
      const x = box.left + slot * (i + 0.5);
      const y = box.bottom - (p.y / yMax) * (box.bottom - box.top);
      const top = Math.min(box.bottom, y);
      ctx.fillStyle = colors[p.label] || '#4f78ba';
      ctx.fillRect(x - barWidth / 2, top, barWidth, box.bottom - top);
      ctx.fillStyle = '#536174';
      ctx.font = '11px Segoe UI, Arial, sans-serif';
      ctx.textAlign = 'center';
      ctx.fillText(p.label, x, box.bottom + 12);
      canvas._chartPoints.push({
        x,
        y: top,
        hitX: slot / 2,
        hitY: Math.max(12, box.bottom - top),
        tooltip: `${p.label}<br>${esc(yLabel)}: ${Number(p.y).toPrecision(8)}`,
        tooltipId
      });
    });
  }

  function drawLineChart(canvas, points, xLabel, yLabel, tooltipId) {
    const { ctx, w, h } = canvasSize(canvas);
    const box = { left: 48, right: w - 12, top: 10, bottom: h - 30 };
    ctx.clearRect(0, 0, w, h);
    if (!points.length) return;
    const [xMin, xMax] = axisRange(points.map((p) => p.x));
    const [yMin, yMax] = axisRange(points.map((p) => p.y));
    drawAxes(ctx, w, h, box, xLabel, yLabel, xMin, xMax, yMin, yMax);
    const sx = (x) => box.left + ((x - xMin) / (xMax - xMin)) * (box.right - box.left);
    const sy = (y) => box.bottom - ((y - yMin) / (yMax - yMin)) * (box.bottom - box.top);
    ctx.strokeStyle = '#2166d1';
    ctx.lineWidth = 1.6;
    ctx.beginPath();
    points.forEach((p, i) => {
      const x = sx(p.x);
      const y = sy(p.y);
      if (i === 0) ctx.moveTo(x, y);
      else ctx.lineTo(x, y);
    });
    ctx.stroke();
    canvas._chartPoints = points.map((p) => {
      const x = sx(p.x);
      const y = sy(p.y);
      ctx.beginPath();
      ctx.arc(x, y, 3.2, 0, Math.PI * 2);
      ctx.fillStyle = '#2166d1';
      ctx.fill();
      return {
        x,
        y,
        hitX: 12,
        hitY: 12,
        tooltip: `${esc(p.label || `Point ${p.index ?? ''}`)}<br>${esc(xLabel)}: ${Number(p.x).toPrecision(8)}<br>${esc(yLabel)}: ${Number(p.y).toPrecision(8)}`,
        tooltipId
      };
    });
  }

  function nearestChartPoint(canvas, x, y) {
    let best = null;
    let bestD2 = Infinity;
    for (const p of canvas._chartPoints || []) {
      const dx = p.x - x;
      const dy = p.y - y;
      if (Math.abs(dx) <= p.hitX && Math.abs(dy) <= Math.max(p.hitY, 12)) {
        const d2 = dx * dx + dy * dy;
        if (d2 < bestD2) {
          best = p;
          bestD2 = d2;
        }
      }
    }
    return best;
  }

  function bindChartTooltip(canvasId, tooltipId) {
    const canvas = $(canvasId);
    const tooltip = $(tooltipId);
    canvas.addEventListener('pointermove', (e) => {
      const rect = canvas.getBoundingClientRect();
      const p = nearestChartPoint(canvas, e.clientX - rect.left, e.clientY - rect.top);
      if (!p) {
        tooltip.style.display = 'none';
        return;
      }
      tooltip.innerHTML = p.tooltip;
      tooltip.style.display = 'block';
      tooltip.style.left = `${Math.min(rect.width - 140, Math.max(4, e.clientX - rect.left + 10))}px`;
      tooltip.style.top = `${Math.max(4, e.clientY - rect.top - 34)}px`;
    });
    canvas.addEventListener('pointerleave', () => { tooltip.style.display = 'none'; });
  }

  function formatValue(value, digits = 4) {
    return Number.isFinite(Number(value)) ? Number(value).toFixed(digits).replace(/\.?0+$/, '') : '—';
  }

  function compositionHeadlineText(rows, total) {
    const parts = rows.map(([element, count, at]) => `${element} ${Number(at).toFixed(2)}%`);
    return `${total} atoms · ${parts.join(' · ')}`;
  }

  function topologyLabel(value) {
    if (value === 'single_interface_slab') return '单界面薄层';
    if (value === 'periodic_bicrystal') return '周期双晶胞';
    return value || '界面';
  }

  function analysisHeadlineText() {
    const analysis = model.analysis || {};
    const series = model.series || {};
    const atoms = model.structure?.species?.length || 0;
    if (model.module === 'random') {
      return `随机 Ti 合金 · ${atoms} atoms · 成分分辨率 ${formatValue(analysis.composition_resolution_at_percent, 3)} at.%`;
    }
    if (model.module === 'crystal') {
      return `Ti 单晶 · ${atoms} atoms · 可检查和导出`;
    }
    if (model.module === 'sqs') {
      return `SQS Ti 合金 · 目标函数 ${formatValue(analysis.objective, 6)} · pair error ${formatValue(analysis.max_abs_pair_error, 6)}`;
    }
    if (model.module === 'vacancy') {
      return `缺陷模型 · 空位原子编号 ${analysis.site_id ?? '—'} · ${atoms} atoms`;
    }
    if (model.module === 'substitution') {
      return `缺陷模型 · 替换原子编号 ${analysis.site_id ?? '—'} → ${analysis.new_species || '—'} · ${atoms} atoms`;
    }
    if (model.module === 'surface') {
      return `表面模型 · ${analysis.plane || '选定晶面'} · 真空层 ${formatValue(analysis.vacuum_angstrom, 2)} Å`;
    }
    if (model.module === 'interface') {
      const candidate = analysis.candidate || {};
      return `Burgers α/β 界面 · ${topologyLabel(analysis.interface_topology)} · 最大几何失配 ${formatValue(candidate.max_imposed_strain_percent, 3)}%`;
    }
    return `${atoms} atoms · 关键几何参数见下方`;
  }

  function setAnalysisChartVisible(show) {
    const wrap = $('analysisCanvas')?.closest('.chartWrap');
    if (wrap) wrap.hidden = !show;
    if (!show) {
      const cv = $('analysisCanvas');
      const ctx = cv?.getContext('2d');
      if (ctx) ctx.clearRect(0, 0, cv.width, cv.height);
      $('analysisTitle').textContent = '关键参数';
      $('analysisHeadline').classList.add('analysisMuted');
    } else {
      $('analysisHeadline').classList.remove('analysisMuted');
    }
  }

  function analysisPlotData() {
    const series = model.series || {};
    if (model.module === 'interface') {
      const candidates = (series.candidates || []).slice(0, 32);
      return {
        points: candidates.map((c, i) => ({
          x: i,
          y: Number(c.max_imposed_strain_percent),
          index: i,
          label: `Candidate #${i}`
        })),
        xLabel: '候选编号',
        yLabel: '最大几何失配 (%)'
      };
    }
    if (model.module === 'sqs' && Array.isArray(series.correlations)) {
      return {
        points: series.correlations.map((c, i) => ({
          x: Number(c.diameter),
          y: Math.abs(Number(c.difference)),
          index: i,
          label: `${c.points}-point cluster #${i}`
        })),
        xLabel: '团簇直径 (Å)',
        yLabel: '相关函数残差'
      };
    }
    if (model.module === 'sqs' && Array.isArray(series.convergence)) {
      return {
        points: series.convergence.map((v, i) => ({ x: i, y: Number(v), index: i, label: `Preview step ${i}` })),
        xLabel: '优化步',
        yLabel: '目标函数'
      };
    }
    return {
      points: [],
      xLabel: '参数',
      yLabel: '值'
    };
  }

  function charts() {
    const counts = {};
    model.structure.species.forEach((e) => { counts[e] = (counts[e] || 0) + 1; });
    const total = model.structure.species.length || 1;
    const compositionRows = Object.entries(counts).map(([e, n]) => [e, n, (100 * n / total).toFixed(5)]);
    $('compositionHeadline').textContent = compositionHeadlineText(compositionRows, total);
    $('compositionTable').innerHTML = table([['Element', 'Count', 'at.%'], ...compositionRows]);
    chartData.composition = compositionRows.map(([e, , at]) => ({ label: e, y: Number(at) }));
    drawBarChart($('compositionCanvas'), chartData.composition, 'Element', 'at.%', 'compositionTooltip');

    const analysis = model.analysis || {};
    const series = model.series || {};
    $('analysisHeadline').textContent = analysisHeadlineText();
    let rows = [
      ['Metric', 'Value'],
      ...Object.entries(analysis)
        .filter((x) => typeof x[1] !== 'object' && !excludedAnalysisMetrics.has(x[0]))
        .map(([k, v]) => [k, typeof v === 'number' ? v.toPrecision(8) : v])
    ];
    if (model.module === 'interface') {
      rows = [['#', 'α 重复', 'β 重复', 'X 失配 %', 'Y 失配 %', '最大失配 %'],
        ...(series.candidates || []).slice(0, 16).map((c, i) => [
          i,
          `${c.alpha_repeat_x}×${c.alpha_repeat_y}`,
          `${c.beta_repeat_x}×${c.beta_repeat_y}`,
          Number(c.mismatch_x_percent).toFixed(4),
          Number(c.mismatch_y_percent).toFixed(4),
          Number(c.max_imposed_strain_percent).toFixed(4)
        ])];
    }
    $('analysisTable').innerHTML = table(rows);

    const plot = analysisPlotData();
    chartData.analysis = plot.points;
    const showPlot = plot.points.length > 1;
    setAnalysisChartVisible(showPlot);
    if (showPlot) {
      $('analysisTitle').textContent = `${plot.xLabel} / ${plot.yLabel}`;
      drawLineChart($('analysisCanvas'), plot.points, plot.xLabel, plot.yLabel, 'chartTooltip');
    }
  }

  function render() {
    modelInfo();
    validation();
    engines();
    draw3d();
    charts();
  }

	function showRevision(record) {
	  model = {
		module: record.module,
		structure: record.structure,
		validation: record.validation,
		allocation: record.allocation,
		sqs: record.sqs,
		atat: record.atat,
		analysis: record.analysis || {},
		series: record.series || {},
		engines: record.engines || []
	  };
	  activeRevisionID = record.id || activeRevisionID;
	  selected = -1;
	  if ($('activeRevisionLabel')) $('activeRevisionLabel').textContent = activeRevisionID ? `结构 ${activeRevisionID.slice(0, 8)}` : '无当前结构';
	  render();
	}

  async function refreshCapabilities() {
    const summary = $('capabilitySummary');
    const panel = $('capabilityPanel');
    const status = $('capabilityStatus');
    if (!summary || !panel) return;
    summary.textContent = '正在检查软件内置建模组件…';
    if (status) status.textContent = '检查中';
    try {
      const response = await fetch('/api/capabilities', { cache: 'no-store' });
      if (!response.ok) throw Error('组件检查失败');
      const report = await response.json();
	  const visible = (report.capabilities || []).filter((item) => item.category !== 'external_connector');
	  const ready = visible.filter((item) => item.status === 'AVAILABLE' || item.status === 'SUPPORTED').length;
	  const allReady = ready === visible.length;
	  summary.textContent = allReady ? '内置建模组件可用' : '部分内置组件需要检查';
	  if (status) { status.textContent = allReady ? '可用' : '检查'; status.className = `badge ${allReady ? 'PASS' : 'WARN'}`; }
      panel.innerHTML = visible.map((item) => {
		return `<div class="diagnosticRow ${esc(item.status)}"><strong>${esc(item.name)}</strong><span>${esc(item.status)}</span></div>`;
      }).join('');
    } catch (error) {
	  summary.textContent = '组件检查失败，请打开故障诊断。';
	  if (status) { status.textContent = '检查'; status.className = 'badge WARN'; }
      panel.innerHTML = '';
    }
  }

	async function probeConnectors() {
	  const panel = $('connectorPanel');
	  if (!panel) return;
	  panel.innerHTML = '<p class="micro">Probing optional connectors…</p>';
	  try {
		const distro = $('atatDistro')?.value.trim() || '';
		const response = await fetch(`/api/connectors?probe=true&distro=${encodeURIComponent(distro)}`, { cache: 'no-store' });
		if (!response.ok) throw Error('Optional connector probe failed');
		const payload = await response.json();
		panel.innerHTML = (payload.report?.tools || []).map((tool) => {
		  return `<div class="diagnosticRow ${esc(tool.status)}"><strong>${esc(tool.name)}</strong><span>${esc(tool.status)}</span></div>`;
		}).join('');
	  } catch (error) {
		panel.textContent = error.message;
	  }
	}

  async function downloadBlob(url, fallback) {
    try {
      const response = await fetch(url, { cache: 'no-store' });
      if (!response.ok) throw Error('Export failed');
      const blob = await response.blob();
      const objectURL = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = objectURL;
      a.download = (/filename="?([^";]+)/i.exec(response.headers.get('content-disposition') || '') || [])[1] || fallback;
      document.body.appendChild(a);
      a.click();
      a.remove();
      setTimeout(() => URL.revokeObjectURL(objectURL), 2000);
      toast('Export complete');
    } catch (error) {
      toast(error.message);
    }
  }

  function suggestedExportName(format) {
    const suffix = ({ poscar: 'POSCAR', xyz: 'model.xyz', extxyz: 'model.extxyz', lammps: 'model.data', cif: 'model.cif' })[format] || 'model.dat';
    return suffix === 'POSCAR' ? 'POSCAR' : `TiAlloyStudio-${suffix}`;
  }

  function showExportResult(result) {
    const box = $('exportResult');
    if (!box) return;
    if (!result || !result.saved) {
      box.hidden = true;
      return;
    }
    lastExportPath = result.path || '';
    box.hidden = false;
    box.innerHTML = `<strong>已导出：${esc(result.filename || '')}</strong><br><span>${esc(result.path || '')}</span><br><span>${Number(result.bytes || 0)} bytes · SHA-256 ${esc(result.sha256 || '')}</span>`;
    if ($('openExportFolderBtn')) $('openExportFolderBtn').hidden = !lastExportPath;
  }

  async function saveExport(format) {
    if (!model) {
      toast('请先生成模型');
      return;
    }
    try {
      const response = await fetch('/api/export/save', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          format,
          revision_id: activeRevisionID,
          suggested_name: suggestedExportName(format)
        })
      });
      const payload = await response.json();
      if (!response.ok) throw Error(payload.error || '导出失败');
      if (payload.cancelled) return;
      showExportResult(payload);
      toast('结构文件已保存');
    } catch (error) {
      toast(error.message);
    }
  }

  async function openExportFolder() {
    if (!lastExportPath) {
      toast('还没有导出的文件');
      return;
    }
    try {
      const response = await fetch('/api/open-folder', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path: lastExportPath })
      });
      const payload = await response.json();
      if (!response.ok) throw Error(payload.error || '无法打开文件夹');
    } catch (error) {
      toast(error.message);
    }
  }

  function applyLanguage() {
    const lang = $('languageMode')?.value || 'zh';
    document.documentElement.lang = lang === 'en' ? 'en' : 'zh-CN';
    const text = lang === 'en'
      ? {
          sub: 'Titanium alloy modeling · visualization · export',
          model: 'Model',
          structure: 'Structure',
          validation: 'Check',
          export: 'Export',
          base: 'Base model',
          defect: 'Defects',
          surface: 'Surface',
          interface: 'α/β Interface',
          build: 'Generate & check model',
          manual: 'Manual',
          exit: 'Exit'
        }
      : {
          sub: '钛合金专属建模 · 可视化 · 导出',
          model: '建模',
          structure: '结构',
          validation: '检查',
          export: '导出',
          base: '基础模型',
          defect: '缺陷',
          surface: '表面',
          interface: 'α/β 界面',
          build: '生成并检查模型',
          manual: '手册',
          exit: '退出'
        };
    document.querySelector('.sub').textContent = text.sub;
    document.querySelector('.mobileTabs [data-mobile-panel="model"]').textContent = text.model;
    document.querySelector('.mobileTabs [data-mobile-panel="structure"]').textContent = text.structure;
    document.querySelector('.mobileTabs [data-mobile-panel="validation"]').textContent = text.validation;
    document.querySelector('.mobileTabs [data-mobile-panel="export"]').textContent = text.export;
    document.querySelector('[data-module="random"]').textContent = text.base;
    document.querySelector('[data-module="vacancy"]').textContent = text.defect;
    document.querySelector('[data-module="surface"]').textContent = text.surface;
    document.querySelector('[data-module="interface"]').textContent = text.interface;
    $('buildBtn').textContent = text.build;
    $('manualBtn').textContent = text.manual;
    $('exitBtn').textContent = text.exit;
  }

  $('viewReset').onclick = () => {
    rotX = -0.35;
    rotY = 0.55;
    zoom = 1;
    panX = 0;
    panY = 0;
    draw3d();
  };
  document.querySelectorAll('[data-view]').forEach((b) => {
    b.onclick = () => {
      panX = 0;
      panY = 0;
      if (b.dataset.view === 'xy') {
        rotX = 0;
        rotY = 0;
      } else if (b.dataset.view === 'xz') {
        rotX = Math.PI / 2;
        rotY = 0;
      } else {
        rotX = Math.PI / 2;
        rotY = Math.PI / 2;
      }
      draw3d();
    };
  });
  $('projectionBtn').onclick = () => {
    orthographic = !orthographic;
    $('projectionBtn').textContent = orthographic ? '正交' : '透视';
    draw3d();
  };
  $('renderMode').onchange = draw3d;
  $('renderQuality').onchange = draw3d;
  $('atomRadius').oninput = draw3d;
  $('exportPngBtn').onclick = exportStructurePNG;
  bindColorControl('colorTi', 'Ti');
  bindColorControl('colorAl', 'Al');
  bindColorControl('colorV', 'V');
  bindColorControl('colorAlpha', 'alpha');
  bindColorControl('colorBeta', 'beta');

  setup3dInteraction();
  bindChartTooltip('compositionCanvas', 'compositionTooltip');
  bindChartTooltip('analysisCanvas', 'chartTooltip');

  document.querySelectorAll('.nav').forEach((b) => { b.onclick = () => setModule(b.dataset.module); });
  $('phase').addEventListener('change', updatePhaseControls);
  $('alloyType')?.addEventListener('change', updateAlloyControls);
  $('languageMode')?.addEventListener('change', applyLanguage);
  $('interfaceTopology')?.addEventListener('change', updateInterfaceTopologyControls);
  $('buildBtn').onclick = build;
  document.querySelectorAll('[data-export]').forEach((b) => {
    b.onclick = () => model
	  ? saveExport(b.dataset.export)
      : toast('请先生成模型');
  });
  $('openExportFolderBtn')?.addEventListener('click', openExportFolder);
  $('manualBtn').onclick = () => downloadBlob('/manual', 'TiAlloyStudio-Manual.docx');
	$('refreshCapabilities').onclick = refreshCapabilities;
	$('probeConnectors')?.addEventListener('click', probeConnectors);
  $('exitBtn').onclick = () => fetch('/api/exit', { method: 'POST' });
  window.addEventListener('resize', () => {
    draw3d();
    if (model) charts();
  });
  setInterval(() => fetch('/api/heartbeat', { method: 'POST' }).catch(() => {}), 5000);
  fetch('/api/info')
    .then((r) => r.json())
    .then((j) => { $('diagnosticVersion').textContent = `${j.version} · ${j.engine}`; })
    .catch(() => {});

	function switchMobilePanel(panel) {
	  document.body.dataset.mobilePanel = panel;
	  document.querySelectorAll('.mobileTabs [data-mobile-panel]').forEach((button) => {
		const selectedPanel = button.dataset.mobilePanel === panel;
		button.classList.toggle('active', selectedPanel);
		button.setAttribute('aria-selected', String(selectedPanel));
	  });
	  window.dispatchEvent(new Event('resize'));
	}
	document.querySelectorAll('.mobileTabs [data-mobile-panel]').forEach((button) => {
	  button.addEventListener('click', () => switchMobilePanel(button.dataset.mobilePanel));
	});

	window.TiAlloyStudio = {
	  showRevision,
	  setActiveRevision(id) {
		activeRevisionID = id || '';
		if ($('activeRevisionLabel')) $('activeRevisionLabel').textContent = activeRevisionID ? `结构 ${activeRevisionID.slice(0, 8)}` : '无当前结构';
	  },
	  editFromRevision(id) { pendingEditParentID = id || ''; },
	  switchMobilePanel,
	  get activeRevisionID() { return activeRevisionID; }
	};

  applyLanguage();
  setModule('random');
	refreshCapabilities();
  build();
})();

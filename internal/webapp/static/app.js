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
      hint: 'α-Ti uses HCP lattice parameters aα and cα.',
      lattice: 'Lattice · α-Ti HCP',
      surface: 'basal_0001',
      gsfe: 'basal_a'
    },
    beta: {
      hint: 'β-Ti uses one BCC lattice parameter aβ.',
      lattice: 'Lattice · β-Ti BCC',
      surface: '100',
      gsfe: '110_111'
    }
  };

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
  let userEnergies = null;
	let activeRevisionID = '';
	let pendingEditParentID = '';

  const num = (id) => Number($(id).value) || 0;
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
    return $(id).value.split(/[;,\s]+/).map(Number).filter(Number.isFinite);
  }

  function requestPayload() {
    let module = active;
    if (active === 'random') module = $('alloyType').value;
    if (active === 'vacancy') module = $('defectType').value;
    return {
      module,
      phase: $('phase').value,
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
      sqs_backend: $('sqsBackend').value,
      sqs_steps: num('sqsSteps'),
      sqs_shells: num('sqsShells'),
      atat_distro: $('atatDistro').value.trim(),
      atat_pair_cutoff_angstrom: num('atatPairCutoff'),
      atat_triplet_cutoff_angstrom: num('atatTripletCutoff'),
      atat_run_seconds: num('atatRunSeconds'),
      site_id: num('siteId'),
      new_species: $('newSpecies').value,
      surface_preset: $('surfacePreset').value,
      vacuum: module === 'interface' ? num('interfaceVacuum') : num('vacuum'),
      interface_max_repeat: num('interfaceMax'),
      interface_candidate: num('interfaceCandidate'),
      interface_distance: num('interfaceDistance'),
      eos_ratios: numberList('eosRatios'),
      eos_index: num('eosIndex'),
      gsfe_preset: $('gsfePreset').value,
      gsfe_steps: num('gsfeSteps'),
      gsfe_index: num('gsfeIndex')
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

  function updatePhaseControls() {
    const phase = currentPhase();
    const meta = phaseMetadata[phase] || phaseMetadata.alpha;
    const showBothLattices = active === 'interface';
    document.querySelectorAll('[data-phase-field]').forEach((field) => {
      field.hidden = !showBothLattices && field.dataset.phaseField !== phase;
    });
    if ($('phaseHint')) {
      $('phaseHint').textContent = showBothLattices
        ? 'α/β interface models use both α and β lattice values.'
        : meta.hint;
    }
    if ($('latticeSummary')) $('latticeSummary').textContent = showBothLattices ? 'Lattice · α and β' : meta.lattice;
    syncPhaseOptions('surfacePreset', phase, meta.surface);
    syncPhaseOptions('gsfePreset', phase, meta.gsfe);
  }

  function setModule(module) {
    active = module;
    document.querySelectorAll('.nav').forEach((b) => b.classList.toggle('active', b.dataset.module === module));
    ['sqsControls', 'defectControls', 'interfaceControls', 'eosControls', 'gsfeControls']
      .forEach((id) => { $(id).style.display = 'none'; });
    $('compositionControls').style.display = (module === 'random' || module === 'sqs') ? 'block' : 'none';
    if (module === 'sqs') $('sqsControls').style.display = 'block';
    if (module === 'vacancy') $('defectControls').style.display = 'block';
    if (module === 'interface') $('interfaceControls').style.display = 'block';
    if (module === 'eos') $('eosControls').style.display = 'block';
    if (module === 'gsfe') $('gsfeControls').style.display = 'block';
    updatePhaseControls();
    userEnergies = null;
    if ($('energies')) $('energies').value = '';
  }

  async function build() {
    try {
      $('statusBadge').textContent = 'Building…';
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
      if (!response.ok) throw Error(payload.error || 'Build failed');
	  if (pendingEditParentID) {
		activeRevisionID = payload.active_revision_id || '';
		const revisionResponse = await fetch(`/api/project/revision?id=${encodeURIComponent(activeRevisionID)}`, { cache: 'no-store' });
		if (!revisionResponse.ok) throw Error('Edited revision could not be loaded');
		showRevision(await revisionResponse.json());
		pendingEditParentID = '';
	  } else {
		model = payload;
	  }
      selected = -1;
      userEnergies = null;
	  if (!pendingEditParentID) render();
      $('statusBadge').textContent = 'Model ready';
    } catch (error) {
      $('statusBadge').textContent = 'Error';
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

  function modelInfo() {
    const s = model.structure;
    const meta = s.meta || {};
    const cell = s.cell;
    const rows = [
      ['Module', model.module],
      ['Phase', meta.phase || '—'],
      ['Bravais', String(meta.bravais || '—').toUpperCase()],
      ['Cell setting', meta.cell_setting || '—'],
      ['Atoms', s.species.length],
      ['a / b / c', cell.map(norm).map((v) => v.toFixed(4)).join(' / ') + ' Å'],
      ['α / β / γ', [
        angle(cell[1], cell[2]),
        angle(cell[0], cell[2]),
        angle(cell[0], cell[1])
      ].map((v) => v.toFixed(3) + '°').join(' / ')],
      ['Volume', Math.abs(determinant(cell)).toFixed(4) + ' Å³'],
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
    cv.width = Math.max(1, Math.round(w * dpr));
    cv.height = Math.max(1, Math.round(h * dpr));
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, w, h);

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

    for (const p of cv._pts) {
      const element = (model.structure.site_labels || [])[p.i] || model.structure.species[p.i];
      ctx.beginPath();
      ctx.arc(p.x, p.y, p.i === selected ? p.radius + 2 : p.radius, 0, Math.PI * 2);
      ctx.fillStyle = colors[element] || '#708090';
      ctx.fill();
      ctx.strokeStyle = p.i === selected ? '#111' : '#fff';
      ctx.lineWidth = p.i === selected ? 1.5 : 1;
      ctx.stroke();
    }

    const labels = model.structure.site_labels?.length
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
    ctx.font = '10px Segoe UI, Arial, sans-serif';
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
      ctx.font = '10px Segoe UI, Arial, sans-serif';
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

  function analysisPlotData() {
    const series = model.series || {};
    const analysis = model.analysis || {};
    if (model.module === 'eos') {
      const x = series.volume_ratios || [];
      const rawY = userEnergies && userEnergies.length === x.length
        ? userEnergies
        : (series.volumes_angstrom3 || []);
      return {
        points: x.map((v, i) => ({ x: Number(v), y: Number(rawY[i]), index: i, label: `EOS #${i}` })),
        xLabel: 'V/V₀',
        yLabel: userEnergies && userEnergies.length === x.length ? 'Energy (eV)' : 'Volume (Å³)'
      };
    }
    if (model.module === 'gsfe') {
      const lambda = series.lambda || [];
      if (userEnergies && userEnergies.length === lambda.length) {
        const area = Number(analysis.area_angstrom2);
        const faults = Number(analysis.fault_count);
        if (area > 0 && faults > 0) {
          const e0 = userEnergies[0];
          const factor = 16021.76634;
          return {
            points: lambda.map((v, i) => ({
              x: Number(v),
              y: (Number(userEnergies[i]) - e0) / (area * faults) * factor,
              index: i,
              label: `GSFE #${i}`
            })),
            xLabel: 'λ',
            yLabel: 'γ (mJ/m²)'
          };
        }
      }
      const path = Array.isArray(analysis.path_angstrom) ? analysis.path_angstrom : [1, 0, 0];
      const pathLength = norm(path);
      return {
        points: lambda.map((v, i) => ({
          x: Number(v),
          y: Number(v) * pathLength,
          index: i,
          label: `GSFE #${i}`
        })),
        xLabel: 'λ',
        yLabel: '|u| (Å)'
      };
    }
    if (model.module === 'interface') {
      const candidates = (series.candidates || []).slice(0, 32);
      return {
        points: candidates.map((c, i) => ({
          x: i,
          y: Number(c.max_imposed_strain_percent),
          index: i,
          label: `Candidate #${i}`
        })),
        xLabel: 'Candidate index',
        yLabel: 'Max imposed strain (%)'
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
        xLabel: 'Cluster diameter (Å)',
        yLabel: '|correlation difference|'
      };
    }
    if (model.module === 'sqs' && Array.isArray(series.convergence)) {
      return {
        points: series.convergence.map((v, i) => ({ x: i, y: Number(v), index: i, label: `Preview step ${i}` })),
        xLabel: 'Recorded preview step',
        yLabel: 'Preview objective'
      };
    }
    const numeric = Object.entries(analysis).filter(([, v]) => typeof v === 'number' && Number.isFinite(v));
    return {
      points: numeric.map(([k, v], i) => ({ x: i, y: Number(v), index: i, label: k })),
      xLabel: 'Metric index',
      yLabel: 'Value'
    };
  }

  function charts() {
    const counts = {};
    model.structure.species.forEach((e) => { counts[e] = (counts[e] || 0) + 1; });
    const total = model.structure.species.length || 1;
    const compositionRows = Object.entries(counts).map(([e, n]) => [e, n, (100 * n / total).toFixed(5)]);
    $('compositionTable').innerHTML = table([['Element', 'Count', 'at.%'], ...compositionRows]);
    chartData.composition = compositionRows.map(([e, , at]) => ({ label: e, y: Number(at) }));
    drawBarChart($('compositionCanvas'), chartData.composition, 'Element', 'at.%', 'compositionTooltip');

    const analysis = model.analysis || {};
    const series = model.series || {};
    let rows = [
      ['Metric', 'Value'],
      ...Object.entries(analysis)
        .filter((x) => typeof x[1] !== 'object')
        .map(([k, v]) => [k, typeof v === 'number' ? v.toPrecision(8) : v])
    ];
    if (model.module === 'eos') {
      rows = [['Index', 'V/V₀', userEnergies ? 'Energy eV' : 'Volume Å³'],
        ...(series.volume_ratios || []).map((v, i) => [
          i,
          Number(v).toFixed(5),
          userEnergies && userEnergies.length === (series.volume_ratios || []).length
            ? Number(userEnergies[i]).toFixed(8)
            : Number(series.volumes_angstrom3[i]).toFixed(6)
        ])];
    }
    if (model.module === 'gsfe') {
      const plot = analysisPlotData();
      rows = [['Index', 'λ', plot.yLabel],
        ...(series.lambda || []).map((v, i) => [
          i,
          Number(v).toFixed(6),
          plot.points[i] ? Number(plot.points[i].y).toFixed(8) : '—'
        ])];
    }
    if (model.module === 'interface') {
      rows = [['#', 'α repeat', 'β repeat', 'raw X %', 'raw Y %', 'max strain %'],
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
    $('analysisTitle').textContent = `${plot.xLabel} / ${plot.yLabel}`;
    drawLineChart($('analysisCanvas'), plot.points, plot.xLabel, plot.yLabel, 'chartTooltip');
  }

  function applyEnergies() {
    if (!model) return toast('Generate a model first');
    if (model.module !== 'eos' && model.module !== 'gsfe') {
      return toast('Energy input is used for EOS or GSFE series');
    }
    const values = $('energies').value.split(/[\s,;]+/).map(Number).filter(Number.isFinite);
    const expected = model.module === 'eos'
      ? (model.series?.volume_ratios || []).length
      : (model.series?.lambda || []).length;
    if (values.length !== expected) {
      return toast(`Expected ${expected} energies, received ${values.length}`);
    }
    userEnergies = values;
    charts();
    if (model.module === 'gsfe') {
      toast('GSFE γ normalized using stored area and fault_count');
    } else {
      toast('EOS energies plotted; no equation-of-state fit is performed here');
    }
  }

  function render() {
    modelInfo();
    validation();
    engines();
    draw3d();
    charts();
    $('eosBatchBtn').disabled = model.module !== 'eos';
    $('gsfeBatchBtn').disabled = model.module !== 'gsfe';
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
	  userEnergies = null;
	  if ($('activeRevisionLabel')) $('activeRevisionLabel').textContent = activeRevisionID ? `Revision ${activeRevisionID.slice(0, 8)}` : 'No active revision';
	  render();
	}

  async function refreshCapabilities() {
    const summary = $('capabilitySummary');
    const panel = $('capabilityPanel');
    const status = $('capabilityStatus');
    if (!summary || !panel) return;
    summary.textContent = 'Checking the included modeling components…';
    if (status) status.textContent = 'Checking';
    try {
      const response = await fetch('/api/capabilities', { cache: 'no-store' });
      if (!response.ok) throw Error('Capability check failed');
      const report = await response.json();
	  const visible = (report.capabilities || []).filter((item) => item.category !== 'external_connector');
	  const ready = visible.filter((item) => item.status === 'AVAILABLE' || item.status === 'SUPPORTED').length;
	  const allReady = ready === visible.length;
	  summary.textContent = allReady ? 'Offline modeling package ready' : 'Some modeling components need attention';
	  if (status) { status.textContent = allReady ? 'Ready' : 'Check'; status.className = `badge ${allReady ? 'PASS' : 'WARN'}`; }
      panel.innerHTML = visible.map((item) => {
		return `<div class="diagnosticRow ${esc(item.status)}"><strong>${esc(item.name)}</strong><span>${esc(item.status)}</span></div>`;
      }).join('');
    } catch (error) {
	  summary.textContent = 'Component check failed. Open troubleshooting details.';
	  if (status) { status.textContent = 'Check'; status.className = 'badge WARN'; }
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
    $('projectionBtn').textContent = orthographic ? 'Orthographic' : 'Perspective';
    draw3d();
  };
  $('atomRadius').oninput = draw3d;

  setup3dInteraction();
  bindChartTooltip('compositionCanvas', 'compositionTooltip');
  bindChartTooltip('analysisCanvas', 'chartTooltip');

  document.querySelectorAll('.nav').forEach((b) => { b.onclick = () => setModule(b.dataset.module); });
  $('phase').addEventListener('change', updatePhaseControls);
  $('buildBtn').onclick = build;
  document.querySelectorAll('[data-export]').forEach((b) => {
    b.onclick = () => model
	  ? downloadBlob(`/api/export?format=${b.dataset.export}${activeRevisionID ? `&revision_id=${encodeURIComponent(activeRevisionID)}` : ''}`, 'model.dat')
      : toast('Generate a model first');
  });
  $('eosBatchBtn').onclick = () => downloadBlob('/api/export-batch?format=poscar', 'TiAlloyStudio-EOS-POSCAR.zip');
  $('gsfeBatchBtn').onclick = () => downloadBlob('/api/export-batch?format=poscar', 'TiAlloyStudio-GSFE-POSCAR.zip');
  $('manualBtn').onclick = () => downloadBlob('/manual', 'TiAlloyStudio-Manual.docx');
	$('refreshCapabilities').onclick = refreshCapabilities;
	$('probeConnectors').onclick = probeConnectors;
  $('applyEnergy').onclick = applyEnergies;
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
		if ($('activeRevisionLabel')) $('activeRevisionLabel').textContent = activeRevisionID ? `Revision ${activeRevisionID.slice(0, 8)}` : 'No active revision';
	  },
	  editFromRevision(id) { pendingEditParentID = id || ''; },
	  switchMobilePanel,
	  get activeRevisionID() { return activeRevisionID; }
	};

  setModule('random');
	refreshCapabilities();
  build();
})();

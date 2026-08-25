(() => {
  'use strict';

  // Interface topology is a physical model choice, not a visual option.
  // Use the already-defined surface_preset request field as a backward-compatible
  // transport field for interface-only requests; the app core ignores it elsewhere.
  const interfaceControls = document.getElementById('interfaceControls');
  let interfaceTopology = null;
  if (interfaceControls) {
    const label = document.createElement('label');
    label.textContent = 'Interface topology';
    interfaceTopology = document.createElement('select');
    interfaceTopology.id = 'interfaceTopology';
    interfaceTopology.innerHTML =
      '<option value="interface_periodic_bicrystal">Periodic bicrystal · recommended for bulk interface/VASP</option>' +
      '<option value="interface_single_slab">Single-interface slab + vacuum · surface/interface studies</option>';
    label.appendChild(interfaceTopology);
    interfaceControls.insertBefore(label, interfaceControls.firstChild);

    const note = document.createElement('p');
    note.className = 'micro';
    note.textContent = 'Periodic bicrystal contains two α/β interfaces and no free surface. Single-interface slab contains one α/β interface plus two vacuum surfaces. Do not mix their energies.';
    label.insertAdjacentElement('afterend', note);

    const vacuum = document.getElementById('interfaceVacuum');
    const vacuumLabel = vacuum && vacuum.closest('label');
    const syncTopologyUI = () => {
      if (vacuumLabel) vacuumLabel.style.display = interfaceTopology.value === 'interface_single_slab' ? '' : 'none';
    };
    interfaceTopology.addEventListener('change', syncTopologyUI);
    syncTopologyUI();
  }

  const nativeFetch = window.fetch.bind(window);
  window.fetch = (input, init) => {
    const url = typeof input === 'string' ? input : (input && input.url) || '';
    if (url === '/api/build' && init && typeof init.body === 'string' && interfaceTopology) {
      try {
        const body = JSON.parse(init.body);
        if (body.module === 'interface') {
          body.surface_preset = interfaceTopology.value;
          init = { ...init, body: JSON.stringify(body) };
        }
      } catch (_) {}
    }
    return nativeFetch(input, init);
  };

  const exitBtn = document.getElementById('exitBtn');
  if (exitBtn) {
    exitBtn.onclick = async () => {
      exitBtn.disabled = true;
      exitBtn.textContent = 'Exiting…';
      try {
        await window.fetch('/api/exit', { method: 'POST', cache: 'no-store', keepalive: true });
      } catch (_) {
        // The server intentionally closes during Exit; a rejected fetch is not an application error.
      }
      setTimeout(() => {
        try { window.close(); } catch (_) {}
        document.documentElement.innerHTML = '<head><title>Ti Alloy Studio</title></head><body style="font-family:Segoe UI,Arial;padding:40px;background:#edf1f5;color:#243448"><h2>Ti Alloy Studio has exited.</h2><p>You can close this window if it remains visible.</p></body>';
      }, 120);
    };
  }

  const canvas = document.getElementById('structureCanvas');
  const wrap = canvas && canvas.parentElement;
  if (canvas && wrap && window.ResizeObserver) {
    // CSS owns the viewer height. ResizeObserver only asks the existing renderer to redraw;
    // it never changes parent geometry, preventing the old canvas intrinsic-size feedback loop.
    let timer = 0;
    const observer = new ResizeObserver(() => {
      clearTimeout(timer);
      timer = setTimeout(() => window.dispatchEvent(new Event('resize')), 40);
    });
    observer.observe(wrap);
  }
})();

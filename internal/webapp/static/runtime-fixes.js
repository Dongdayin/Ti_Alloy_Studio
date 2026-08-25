(() => {
  'use strict';

  const exitBtn = document.getElementById('exitBtn');
  if (exitBtn) {
    exitBtn.onclick = async () => {
      exitBtn.disabled = true;
      exitBtn.textContent = 'Exiting…';
      try {
        await fetch('/api/exit', { method: 'POST', cache: 'no-store', keepalive: true });
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

(() => {
  if (!('serviceWorker' in navigator)) return;
  const path = window.location.pathname || '/';
  const protectedPath =
    path === '/login' ||
    path === '/app' ||
    path.startsWith('/app/') ||
    path === '/partner' ||
    path.startsWith('/partner/');
  if (protectedPath) {
    // The worker itself bypasses protected routes. Registration is kept so
    // the installed public PWA can update without caching authenticated data.
  }
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/service-worker.js', {scope: '/'}).catch(() => {
      // PWA support is supplementary; the online product remains functional.
    });
  });
})();

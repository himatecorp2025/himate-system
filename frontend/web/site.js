(() => {
  const header = document.querySelector('.site-nav');
  const button = header?.querySelector('.menu');
  const nav = header?.querySelector('.links');
  if (!header || !button || !nav) return;

  const setOpen = (open) => {
    header.classList.toggle('is-open', open);
    button.setAttribute('aria-expanded', String(open));
    button.setAttribute('aria-label', open ? 'Close navigation' : 'Open navigation');
    button.textContent = open ? '×' : '☰';
  };

  button.addEventListener('click', () => setOpen(!header.classList.contains('is-open')));
  nav.addEventListener('click', (event) => {
    const target = event.target;
    if (target instanceof Element && target.closest('a')) setOpen(false);
  });
  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape') setOpen(false);
  });
  window.addEventListener('resize', () => {
    if (window.innerWidth > 1000) setOpen(false);
  }, {passive: true});
})();

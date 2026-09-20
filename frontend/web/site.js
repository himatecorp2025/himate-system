(() => {
  const header = document.querySelector('.site-nav');
  const button = header?.querySelector('.menu');
  const nav = header?.querySelector('.links');

  if (header && button && nav) {
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
  }

  const publicPageSlug = () => {
    const path = window.location.pathname.replace(/^\/+|\/+$/g, '');
    return path === '' ? 'landing' : path.split('/')[0].toLowerCase();
  };

  const ensureMeta = (name) => {
    let node = document.head.querySelector(`meta[name="${name}"]`);
    if (!node) {
      node = document.createElement('meta');
      node.setAttribute('name', name);
      document.head.appendChild(node);
    }
    return node;
  };

  const applySEO = (seo) => {
    if (!seo || typeof seo !== 'object') return;
    if (typeof seo.title === 'string' && seo.title.trim()) document.title = seo.title.trim();
    if (typeof seo.meta_description === 'string' && seo.meta_description.trim()) {
      ensureMeta('description').setAttribute('content', seo.meta_description.trim());
    }
    if (seo.noindex === true) ensureMeta('robots').setAttribute('content', 'noindex,nofollow');
    if (typeof seo.canonical === 'string' && seo.canonical.trim()) {
      let canonical = document.head.querySelector('link[rel="canonical"]');
      if (!canonical) {
        canonical = document.createElement('link');
        canonical.setAttribute('rel', 'canonical');
        document.head.appendChild(canonical);
      }
      canonical.setAttribute('href', seo.canonical.trim());
    }
  };

  const sectionFallback = (componentType) => {
    switch ((componentType || '').toUpperCase()) {
      case 'HERO': return document.querySelector('.hero, .page-hero');
      case 'MISSION': return document.querySelector('.editorial');
      case 'IMPACT': return document.querySelector('.impact-story');
      case 'CTA': return document.querySelector('.cta');
      case 'CONTACT': return document.querySelector('#contact-form, .contact-form');
      default: return null;
    }
  };

  const sectionTarget = (section) => {
    const id = typeof section.id === 'string' ? section.id.trim() : '';
    if (id) {
      const safe = window.CSS?.escape ? window.CSS.escape(id) : id.replace(/[^a-zA-Z0-9_-]/g, '');
      const explicit = document.querySelector(`[data-cms-section="${safe}"]`);
      if (explicit) return explicit;
    }
    return sectionFallback(section.component_type);
  };

  const mediaURL = (id) => `/public/v1/cms/media/${encodeURIComponent(id)}`;

  const applySection = (section) => {
    if (!section || section.visible === false || section.component_type === 'STORY_VIDEO') return;
    const target = sectionTarget(section);
    if (!target) return;

    const heading = target.querySelector('[data-cms-heading], h1, h2, h3');
    if (heading && typeof section.heading === 'string' && section.heading.trim()) {
      heading.textContent = section.heading.trim();
    }

    const body = target.querySelector('[data-cms-body], p');
    if (body && typeof section.body === 'string' && section.body.trim()) {
      body.textContent = section.body.trim();
    }

    const cta = target.querySelector('[data-cms-cta], a.btn, a.caps');
    if (cta) {
      if (typeof section.cta_label === 'string' && section.cta_label.trim()) {
        cta.textContent = section.cta_label.trim();
      }
      if (typeof section.cta_url === 'string' && section.cta_url.trim()) {
        cta.setAttribute('href', section.cta_url.trim());
      }
    }

    if (typeof section.media_asset_id === 'string' && section.media_asset_id.trim()) {
      const image = target.querySelector('[data-cms-media], img');
      if (image instanceof HTMLImageElement) image.src = mediaURL(section.media_asset_id.trim());
    }
  };

  let activeStoryModal = null;

  const closeStory = () => {
    if (!activeStoryModal) return;
    const video = activeStoryModal.querySelector('video');
    if (video instanceof HTMLVideoElement) {
      video.pause();
      video.removeAttribute('src');
      video.load();
    }
    activeStoryModal.remove();
    activeStoryModal = null;
    document.documentElement.classList.remove('story-video-open');
  };

  const openStory = (section) => {
    const mediaId = typeof section?.media_asset_id === 'string' ? section.media_asset_id.trim() : '';
    if (!mediaId) return false;

    closeStory();
    const modal = document.createElement('div');
    modal.className = 'story-video-modal';
    modal.setAttribute('role', 'dialog');
    modal.setAttribute('aria-modal', 'true');
    modal.setAttribute('aria-label', section.heading?.trim() || 'HIMATE story video');
    modal.innerHTML = `
      <div class="story-video-backdrop" data-story-close></div>
      <div class="story-video-dialog">
        <button class="story-video-close" type="button" aria-label="Close story video" data-story-close>×</button>
        <video class="story-video-player" controls autoplay playsinline preload="metadata"></video>
      </div>`;
    document.body.appendChild(modal);
    activeStoryModal = modal;
    document.documentElement.classList.add('story-video-open');

    const video = modal.querySelector('video');
    video.src = mediaURL(mediaId);
    video.addEventListener('ended', closeStory, {once: true});
    modal.querySelectorAll('[data-story-close]').forEach((node) => node.addEventListener('click', closeStory));
    modal.querySelector('.story-video-close')?.focus();
    video.play().catch(() => {});
    return true;
  };

  const storyButton = document.querySelector('.watch-story');
  let storySection = null;
  let cmsLoaded = false;

  if (storyButton) {
    storyButton.setAttribute('href', '#watch-our-story');
    storyButton.setAttribute('aria-haspopup', 'dialog');
    storyButton.addEventListener('click', (event) => {
      event.preventDefault();
      if (storySection && openStory(storySection)) return;
      if (!cmsLoaded) {
        storyButton.classList.add('is-loading');
        return;
      }
      storyButton.classList.add('is-unavailable');
      const label = storyButton.querySelector('span:last-child');
      if (label) {
        const original = label.textContent;
        label.textContent = 'Story video coming soon';
        window.setTimeout(() => { label.textContent = original; }, 2200);
      }
    });
  }

  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && activeStoryModal) closeStory();
  });

  const loadPublishedCMS = async () => {
    const slug = publicPageSlug();
    try {
      const response = await fetch(`/public/v1/cms/pages/${encodeURIComponent(slug)}`, {
        method: 'GET',
        headers: {'Accept': 'application/json'},
        credentials: 'same-origin',
      });
      if (!response.ok) return;
      const page = await response.json();
      applySEO(page.seo);
      const sections = Array.isArray(page.sections) ? [...page.sections] : [];
      sections.sort((a, b) => Number(a.sort_order || 0) - Number(b.sort_order || 0));
      for (const section of sections) {
        if ((section.component_type || '').toUpperCase() === 'STORY_VIDEO' && section.visible !== false) {
          storySection = section;
          continue;
        }
        applySection(section);
      }
    } catch (_) {
      // Static public content remains the fail-safe if the CMS is unavailable.
    } finally {
      cmsLoaded = true;
      storyButton?.classList.remove('is-loading');
    }
  };

  void loadPublishedCMS();
})();

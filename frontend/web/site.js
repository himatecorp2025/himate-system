(() => {

  const publicLocale = (() => {
    const query = new URLSearchParams(window.location.search).get('lang');
    const normalize = (value) => {
      const raw = String(value || '').toLowerCase();
      return raw === 'hu' || raw === 'hu_hu' || raw.startsWith('hu-') ? 'hu_HU' : 'en_US';
    };
    if (query) return normalize(query);
    const stored = window.localStorage.getItem('himate_locale');
    if (stored === 'hu_HU' || stored === 'en_US') return stored;
    const cookie = document.cookie.split(';').map((item) => item.trim()).find((item) => item.startsWith('himate_public_locale='));
    if (cookie) return normalize(decodeURIComponent(cookie.split('=').slice(1).join('=')));
    return normalize(navigator.language);
  })();

  const publicHu = {
    'Platform':'Platform','Modules':'Modulok','Programs':'Programok','Impact':'Hatás','Partners':'Partnerek','Contact':'Kapcsolat','Login':'Bejelentkezés',
    'Culture fuels tomorrow':'A kultúra táplálja a holnapot','Culture Fuels Tomorrow.':'A kultúra táplálja a holnapot.',
    'Culture connects people':'A kultúra összeköti az embereket','CULTURE CONNECTS PEOPLE':'A KULTÚRA ÖSSZEKÖTI AZ EMBEREKET',
    'Culture builds brighter tomorrows':'A kultúra fényesebb holnapot épít','Culture measures what matters':'A kultúra azt méri, ami számít',
    'CULTURE CREATES OPPORTUNITY':'A KULTÚRA LEHETŐSÉGET TEREMT','CULTURE':'KULTÚRA',
    'EMPOWERING':'MEGERŐSÍTJÜK','CULTURE.':'KULTÚRÁT.','AMPLIFYING':'FELERŐSÍTJÜK','IMPACT.':'A HATÁST.',
    'The HIMATE System unites people, programs, and innovation to build a smarter, brighter future for arts and culture.':'A HIMATE System embereket, programokat és innovációt kapcsol össze, hogy intelligensebb, fényesebb jövőt építsen a művészet és a kultúra számára.',
    'The HIMATE System provides the tools, intelligence, and community to help arts and cultural organizations thrive in a changing world.':'A HIMATE System eszközöket, intelligenciát és közösséget ad a művészeti és kulturális szervezeteknek, hogy egy változó világban is fejlődhessenek.',
    'Explore the platform →':'Fedezd fel a platformot →','Watch our story':'Nézd meg a történetünket','Heritage meets innovation':'Örökség és innováció találkozása',
    'People':'Emberek','People reached':'Elért emberek','Partner institutions':'Partnerintézmények','Organizations':'Szervezetek',
    'POWERFUL':'ERŐSEBBEN','TOGETHER':'EGYÜTT','Powerful together':'Erősek együtt','One integrated system':'Egy integrált rendszer',
    'An integrated ecosystem designed for the unique world of arts and culture.':'Integrált ökoszisztéma, amelyet a művészet és kultúra egyedi világára terveztünk.',
    'People module':'Emberek modul','Programs module':'Programok modul','Impact module':'Hatás modul',
    'Partner teams, contacts and communities.':'Partnercsapatok, kapcsolatok és közösségek.','Programs, workflows and operations.':'Programok, munkafolyamatok és működés.','Metrics, evidence and reporting.':'Mérőszámok, bizonyítékok és jelentések.',
    'Explore all modules →':'Fedezd fel az összes modult →','Real culture. Real change.':'Valódi kultúra. Valódi változás.',
    'A MORE VIBRANT':'EGY ÉLŐBB','TOMORROW':'HOLNAP','TOMORROW.':'HOLNAP.','A more vibrant cultural tomorrow.':'Egy élőbb kulturális holnap.',
    'From local initiatives to global partnerships, HIMATE helps cultural organizations turn ambition into measurable impact.':'A helyi kezdeményezésektől a globális partnerségekig a HIMATE segít a kulturális szervezeteknek a célokat mérhető hatássá alakítani.',
    'From local initiatives to global collaboration, HIMATE helps cultural organizations turn ambition into impact.':'A helyi kezdeményezésektől a globális együttműködésig a HIMATE segít a kulturális szervezeteknek a célokat valódi hatássá alakítani.',
    'See the impact →':'Nézd meg a hatást →','LET’S BUILD WHAT’S NEXT':'ÉPÍTSÜK MEG, AMI KÖVETKEZIK','LET’S BUILD':'ÉPÍTSÜK MEG','WHAT’S NEXT.':'AMI KÖVETKEZIK.',
    'Join a global community shaping a brighter future for arts and culture.':'Csatlakozz egy globális közösséghez, amely fényesebb jövőt formál a művészet és kultúra számára.',
    'Get started →':'Kezdjük el →','Get in touch →':'Lépj kapcsolatba velünk →','A smarter future':'Intelligensebb jövő','for arts & culture':'a művészetért és kultúráért',
    'BUILT FOR':'ARRA ÉPÍTVE,','WHAT ENDURES':'AMI MARADANDÓ','HIMATE provides the tools, intelligence, and community to help cultural organizations thrive — today and for generations to come.':'A HIMATE eszközöket, intelligenciát és közösséget biztosít, hogy a kulturális szervezetek ma és a következő generációk számára is fejlődhessenek.',
    'Our approach →':'Megközelítésünk →','People · Ideas · Places · Possibilities':'Emberek · Ötletek · Helyek · Lehetőségek',
    'ideas':'ötletek','places':'helyek','possibilities':'lehetőségek','Heritage':'Örökség','innovation':'innováció',
    'A modern platform':'Modern platform','HIMATE Platform':'HIMATE Platform','Technology for a brighter tomorrow':'Technológia egy fényesebb holnapért',
    'TECHNOLOGY':'TECHNOLÓGIA','THAT EMPOWERS':'AMI MEGERŐSÍT','THAT INSPIRE':'AMI INSPIRÁL','FOR A BRIGHTER':'EGY FÉNYESEBB','CULTURAL TOMORROW':'KULTURÁLIS HOLNAPÉRT',
    'Integrated tools. Deeper insights. A stronger cultural future — governed centrally and tailored to each partner.':'Integrált eszközök. Mélyebb felismerések. Erősebb kulturális jövő — központilag irányítva, partnerenként testreszabva.',
    'Unified platform':'Egységes platform','Secure foundation':'Biztonságos alap','Connected ecosystem':'Összekapcsolt ökoszisztéma','Built to grow':'Növekedésre tervezve',
    'One shared platform foundation supports isolated partner environments and future growth across arts organizations.':'Egy közös platformalap támogatja az elkülönített partnerkörnyezeteket és a művészeti szervezetek jövőbeli növekedését.',
    'Trusted infrastructure':'Megbízható infrastruktúra','Integrated operations':'Integrált működés','Connected institutions':'Összekapcsolt intézmények','Scalable impact':'Skálázható hatás',
    'Technology should amplify the institutions and people carrying culture forward — not flatten what makes them distinctive.':'A technológiának fel kell erősítenie a kultúrát továbbvivő intézményeket és embereket, nem pedig eltüntetnie azt, ami egyedivé teszi őket.',
    'HIMATE Modules':'HIMATE Modulok','Our modules':'Moduljaink','MODULAR BY DESIGN.':'MODULÁRIS TERVEZÉS.','COHERENT BY EXPERIENCE.':'EGYSÉGES ÉLMÉNY.',
    'MANY CAPABILITIES.':'SOK KÉPESSÉG.','ONE SYSTEM.':'EGY RENDSZER.','Each capability belongs to a stable module identity, with central licensing and partner-specific access.':'Minden képesség stabil modulazonosítóhoz tartozik, központi licenceléssel és partnerenkénti hozzáféréssel.',
    'Module governance':'Modulirányítás','Commercial control':'Kereskedelmi kontroll','Partner lifecycle':'Partner-életciklus','System health':'Rendszerállapot',
    'Activate, license, maintain or extend functionality without fragmenting the product. The interface remains unmistakably HIMATE even as partner configurations evolve.':'Aktiválj, licencelj, tarts karban vagy bővíts funkciókat a termék széttöredezése nélkül. A felület a partnerkonfigurációk változása mellett is egyértelműen HIMATE marad.',
    'Canonical + custom modules':'Kanonikus + egyedi modulok','Partner ready':'Partnerre kész','Priced defaults':'Árazott alapértékek',
    'HIMATE Programs':'HIMATE Programok','People. Programs. Possibilities.':'Emberek. Programok. Lehetőségek.','PROGRAMS WITH':'PROGRAMOK','GREATER IMPACT':'NAGYOBB HATÁSSAL',
    'Connect talent, communities, and vision.':'Kapcsold össze a tehetséget, a közösségeket és a jövőképet.','Streamline operations. Expand reach.':'Egyszerűsítsd a működést. Növeld az elérést.',
    'Flexible modules and structured operations make it possible to scale without losing creative identity.':'A rugalmas modulok és strukturált működés lehetővé teszik a növekedést a kreatív identitás elvesztése nélkül.',
    'Talent and community':'Tehetség és közösség','Cultural programs':'Kulturális programok','Operations and reach':'Működés és elérés','Outcomes and evidence':'Eredmények és bizonyítékok',
    'People and participation':'Emberek és részvétel','Creative practice':'Kreatív gyakorlat','Community reach':'Közösségi elérés','Measurable outcomes':'Mérhető eredmények',
    'Explore programs →':'Programok felfedezése →','HIMATE Impact':'HIMATE Hatás','Measure what matters. Create lasting change.':'Mérd azt, ami számít. Teremts tartós változást.',
    'MEASURABLE':'MÉRHETŐ','CHANGE':'VÁLTOZÁS','Real progress. Lasting change.':'Valódi előrelépés. Tartós változás.',
    'Data-driven insights. Real-world outcomes. Stronger communities — with source, context and evidence preserved.':'Adatalapú felismerések. Valós eredmények. Erősebb közösségek — megőrzött forrással, kontextussal és bizonyítékkal.',
    'Metric definitions':'Mérőszám-definíciók','Source-linked records':'Forráshoz kötött rekordok','Auditable evidence':'Auditálható bizonyíték','Partner reporting':'Partnerjelentések',
    'Defined':'Definiált','Sourced':'Forrásolt','Verified':'Ellenőrzött','Exportable':'Exportálható','Impact records are designed around metric definition, period, partner, actor, source and supporting evidence.':'A hatásrekordok a mérőszám-definíció, időszak, partner, végrehajtó, forrás és alátámasztó bizonyíték köré épülnek.',
    'Evidence attached':'Bizonyíték csatolva','Provenance preserved':'Eredet megőrizve','Evidence history':'Bizonyítékelőzmények','Segmented insights':'Szegmentált felismerések',
    'Explore our impact →':'Fedezd fel a hatásunkat →','HIMATE Partners':'HIMATE Partnerek','Stronger together':'Együtt erősebbek','STRONGER TOGETHER':'EGYÜTT ERŐSEBBEK',
    'We partner with forward-thinking organizations, institutions, and leaders to expand what is possible for arts and culture.':'Jövőbe tekintő szervezetekkel, intézményekkel és vezetőkkel dolgozunk együtt, hogy tágítsuk a művészet és kultúra lehetőségeit.',
    'Connected partners':'Összekapcsolt partnerek','Mission-aligned support':'Küldetéshez illeszkedő támogatás','ROOM TO GROW':'TÉR A NÖVEKEDÉSHEZ',
    'HIMATE is designed for long-term relationships: one scalable technology foundation, distinct partner environments, and room to evolve together.':'A HIMATE hosszú távú kapcsolatokra készült: egy skálázható technológiai alap, elkülönített partnerkörnyezetek és közös fejlődési lehetőség.',
    'Arts organizations':'Művészeti szervezetek','Cultural institutions':'Kulturális intézmények','Foundations':'Alapítványok','Museums':'Múzeumok','Music':'Zene','Creative networks':'Kreatív hálózatok',
    'Build together':'Építsünk együtt','Shared possibility':'Közös lehetőség','Greater than the sum of parts':'Több, mint a részek összege','Become a partner →':'Legyél partner →',
    'Contact HIMATE':'Kapcsolat a HIMATE-tel','Start a conversation':'Indíts beszélgetést','A CONVERSATION':'EGY BESZÉLGETÉS','WORTH HAVING.':'AMIT ÉRDEMES MEGEJTENI.',
    'Start a conversation about how HIMATE can support your organization, programs, people and long-term cultural impact.':'Beszéljünk arról, hogyan támogathatja a HIMATE a szervezetedet, programjaidat, embereidet és hosszú távú kulturális hatásodat.',
    'Partnerships':'Partnerségek','Explore possibilities':'Fedezd fel a lehetőségeket','Global collaboration':'Globális együttműködés','Extend your reach':'Növeld az elérésedet',
    'Partner with HIMATE':'Lépj partnerségre a HIMATE-tel','Tell us about your organization and what you want technology to make possible. Commercial terms and platform configuration are tailored partner by partner.':'Mesélj a szervezetedről és arról, mit szeretnél a technológiával lehetővé tenni. A kereskedelmi feltételeket és a platform konfigurációját partnerenként alakítjuk.',
    'Arts':'Művészet','Culture':'Kultúra','Institutions':'Intézmények','Send inquiry →':'Megkeresés küldése →',
    'Your name':'Neved','Organization':'Szervezet','Email address':'E-mail-cím','Tell us what you are building':'Írd meg, mit építesz','Website':'Weboldal',
    'Sending…':'Küldés…','Sending your inquiry…':'Megkeresés küldése…','Thank you. Your inquiry has been received.':'Köszönjük. Megkaptuk a megkeresésedet.',
    'We could not send your inquiry. Please try again.':'A megkeresést nem sikerült elküldeni. Kérjük, próbáld újra.',
    '© 2026 HIMATE System. All rights reserved.':'© 2026 HIMATE System. Minden jog fenntartva.',
    'Culture connected':'Összekapcsolt kultúra','People empowered':'Megerősített emberek','Programs strengthened':'Megerősített programok','Measurable change':'Mérhető változás',
    'Community-rooted work':'Közösségben gyökerező munka','Global':'Globális','Local initiatives':'Helyi kezdeményezések','Real-world results':'Valós eredmények',
    'Performance':'Teljesítmény','Collaboration':'Együttműködés','Community':'Közösség','WHAT COMES NEXT':'AMI KÖVETKEZIK','THE HUMAN STORY':'AZ EMBERI TÖRTÉNET',
    'Heritage · Ideas · People · Progress':'Örökség · Ötletek · Emberek · Haladás','People · Partnerships · Progress':'Emberek · Partnerségek · Haladás',
    'BUILDING A BRIGHTER CULTURAL TOMORROW':'FÉNYESEBB KULTURÁLIS HOLNAPOT ÉPÍTÜNK','IMPACT. A BRIGHTER':'HATÁS. EGY FÉNYESEBB',
    'FOR ARTS & CULTURE':'A MŰVÉSZETÉRT ÉS KULTÚRÁÉRT','MORE OPPORTUNITIES':'TÖBB LEHETŐSÉG','STRONGER COMMUNITIES':'ERŐSEBB KÖZÖSSÉGEK',
    'EVIDENCE BEFORE CLAIMS.':'BIZONYÍTÉK AZ ÁLLÍTÁSOK ELŐTT.','WITHOUT LOSING':'ANÉLKÜL, HOGY ELVESZÍTENÉNK','Designed to inspire':'Inspirációra tervezve',
    'Better operations':'Jobb működés','Greater impact':'Nagyobb hatás','Stronger communities':'Erősebb közösségek',
    'Bring program activity, operational context and future impact evidence into one connected environment. Designed to support artists, institutions and cultural communities with clarity.':'Kapcsold össze a programtevékenységet, a működési kontextust és a jövőbeli hatás bizonyítékait egyetlen környezetben. Művészek, intézmények és kulturális közösségek világos támogatására tervezve.',
    'POWERFUL TOGETHER':'EGYÜTT ERŐSEK','Open navigation':'Navigáció megnyitása','Close navigation':'Navigáció bezárása',
    'A smarter future for arts & culture':'Intelligensebb jövő a művészet és kultúra számára',
    'BUILT TO SCALE':'NÖVEKEDÉSRE TERVEZVE','Cultural stewardship':'Kulturális gondoskodás','HIMATE System — Culture Fuels Tomorrow':'HIMATE System — A kultúra táplálja a holnapot',
    'IMPACT':'HATÁS','meets':'találkozik','MODULES FOR':'MODULOK','PEOPLE':'EMBEREK','PROGRAMS':'PROGRAMOK',
    'Unite people, data and opportunity in one refined control plane. HIMATE brings governance, partner operations, modules, commercial rules and infrastructure together without sacrificing partner separation.':'Kapcsold össze az embereket, adatokat és lehetőségeket egy kifinomult vezérlőplatformon. A HIMATE egyesíti az irányítást, partnerműködést, modulokat, kereskedelmi szabályokat és infrastruktúrát a partnerek elkülönítésének megőrzésével.',
    'Verified data':'Ellenőrzött adatok'
  };

  const tPublic = (value) => publicLocale === 'hu_HU' ? (publicHu[value] || value) : value;
  window.himateTranslate = tPublic;
  window.himatePublicLocale = publicLocale;
  document.documentElement.lang = publicLocale === 'hu_HU' ? 'hu' : 'en';
  if (publicLocale === 'hu_HU') document.title = tPublic(document.title);
  window.localStorage.setItem('himate_locale', publicLocale);
  document.cookie = 'himate_public_locale=' + encodeURIComponent(publicLocale) + '; Path=/; Max-Age=31536000; SameSite=Lax';

  const translatePublicDocument = () => {
    if (publicLocale !== 'hu_HU') return;
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
    const nodes = [];
    while (walker.nextNode()) {
      const parent = walker.currentNode.parentElement;
      if (!parent || ['SCRIPT','STYLE','NOSCRIPT'].includes(parent.tagName)) continue;
      nodes.push(walker.currentNode);
    }
    for (const node of nodes) {
      const original = node.nodeValue || '';
      const trimmed = original.trim();
      if (!trimmed) continue;
      const translated = tPublic(trimmed);
      if (translated !== trimmed) node.nodeValue = original.replace(trimmed, translated);
    }
    document.querySelectorAll('[placeholder],[aria-label],[title]').forEach((node) => {
      for (const attr of ['placeholder','aria-label','title']) {
        const value = node.getAttribute(attr);
        if (!value) continue;
        const translated = tPublic(value);
        if (translated !== value) node.setAttribute(attr, translated);
      }
    });
  };


  const safeDesignColor = (value, fallback) => /^#[0-9A-Fa-f]{6}$/.test(String(value || '')) ? String(value) : fallback;
  const safeDesignFont = (value, fallback) => ['Cormorant Garamond','Inter','Georgia','Arial'].includes(String(value || '')) ? String(value) : fallback;

  const applyPublishedDesign = (payload) => {
    const design = payload && typeof payload.design === 'object' ? payload.design : null;
    if (!design) return;
    const root = document.documentElement;
    root.style.setProperty('--design-navy', safeDesignColor(design.navy, '#06172C'));
    root.style.setProperty('--design-gold', safeDesignColor(design.gold, '#D7AE62'));
    root.style.setProperty('--design-background', safeDesignColor(design.background, '#F8F9FB'));
    root.style.setProperty('--design-text', safeDesignColor(design.text_color, '#1F2937'));
    root.style.setProperty('--design-heading-font', '"' + safeDesignFont(design.heading_font, 'Cormorant Garamond') + '"');
    root.style.setProperty('--design-body-font', '"' + safeDesignFont(design.body_font, 'Inter') + '"');
    const radius = Math.min(40, Math.max(0, Number(design.button_radius || 6)));
    root.style.setProperty('--design-button-radius', radius + 'px');

    const logoId = typeof design.logo_media_asset_id === 'string' ? design.logo_media_asset_id.trim() : '';
    if (logoId) {
      document.querySelectorAll('.brand-logo img').forEach((image) => {
        if (image instanceof HTMLImageElement) image.src = mediaURL(logoId);
      });
    }

    const navigation = Array.isArray(design.navigation)
      ? design.navigation.filter((item) => item && item.visible !== false).slice().sort((a,b) => Number(a.sort_order || 0) - Number(b.sort_order || 0))
      : [];
    if (navigation.length) {
      const primary = document.querySelector('.site-nav .links');
      if (primary) {
        primary.querySelectorAll('a:not(.nav-login-text):not(.login-pill)').forEach((node) => node.remove());
        const anchor = primary.querySelector('.locale-switch') || primary.querySelector('.nav-divider') || primary.firstChild;
        for (const item of navigation) {
          const link = document.createElement('a');
          const href = typeof item.url === 'string' && item.url.trim() ? item.url.trim() : '/';
          link.href = href;
          link.textContent = publicLocale === 'hu_HU'
            ? (item.label_hu || item.label_en || href)
            : (item.label_en || item.label_hu || href);
          if (window.location.pathname === new URL(href, window.location.origin).pathname) {
            link.classList.add('active');
            link.setAttribute('aria-current', 'page');
          }
          primary.insertBefore(link, anchor);
        }
      }
      const footer = document.querySelector('.footer-links');
      if (footer) {
        footer.textContent = '';
        for (const item of navigation) {
          const link = document.createElement('a');
          link.href = typeof item.url === 'string' && item.url.trim() ? item.url.trim() : '/';
          link.textContent = publicLocale === 'hu_HU'
            ? (item.label_hu || item.label_en || link.href)
            : (item.label_en || item.label_hu || link.href);
          footer.appendChild(link);
        }
      }
    }
  };

  const loadPublishedDesign = async () => {
    try {
      const response = await fetch('/public/v1/cms/design', {
        method: 'GET',
        headers: {'Accept':'application/json'},
        credentials: 'same-origin',
      });
      if (!response.ok) return;
      applyPublishedDesign(await response.json());
    } catch (_) {
      // The approved source-controlled design remains the safe fallback.
    }
  };

  const installLocaleSwitch = () => {
    const navigation = document.querySelector('.site-nav .links');
    if (!navigation || navigation.querySelector('.locale-switch')) return;
    const localeButton = document.createElement('button');
    localeButton.type = 'button';
    localeButton.className = 'locale-switch';
    localeButton.setAttribute('aria-label', publicLocale === 'hu_HU' ? 'Switch to English' : 'Váltás magyarra');
    localeButton.textContent = publicLocale === 'hu_HU' ? 'EN' : 'HU';
    localeButton.addEventListener('click', () => {
      const next = publicLocale === 'hu_HU' ? 'en_US' : 'hu_HU';
      window.localStorage.setItem('himate_locale', next);
      document.cookie = 'himate_public_locale=' + encodeURIComponent(next) + '; Path=/; Max-Age=31536000; SameSite=Lax';
      const targetUrl = new URL(window.location.href);
      targetUrl.searchParams.set('lang', next === 'hu_HU' ? 'hu' : 'en');
      window.location.assign(targetUrl.toString());
    });
    const divider = navigation.querySelector('.nav-divider');
    if (divider) divider.before(localeButton); else navigation.appendChild(localeButton);
  };

  const header = document.querySelector('.site-nav');
  const button = header?.querySelector('.menu');
  const nav = header?.querySelector('.links');

  if (header && button && nav) {
    const setOpen = (open) => {
      header.classList.toggle('is-open', open);
      button.setAttribute('aria-expanded', String(open));
      button.setAttribute('aria-label', open ? tPublic('Close navigation') : tPublic('Open navigation'));
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

  const ensurePropertyMeta = (property) => {
    let node = document.head.querySelector(`meta[property="${property}"]`);
    if (!node) {
      node = document.createElement('meta');
      node.setAttribute('property', property);
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
    ensureMeta('robots').setAttribute('content', seo.noindex === true ? 'noindex,nofollow' : 'index,follow');
    const title = typeof seo.title === 'string' ? seo.title.trim() : '';
    const description = typeof seo.meta_description === 'string' ? seo.meta_description.trim() : '';
    const ogTitle = typeof seo.og_title === 'string' && seo.og_title.trim() ? seo.og_title.trim() : title;
    const ogDescription = typeof seo.og_description === 'string' && seo.og_description.trim() ? seo.og_description.trim() : description;
    if (ogTitle) ensurePropertyMeta('og:title').setAttribute('content', ogTitle);
    if (ogDescription) ensurePropertyMeta('og:description').setAttribute('content', ogDescription);
    if (typeof seo.og_image_asset_id === 'string' && seo.og_image_asset_id.trim()) {
      ensurePropertyMeta('og:image').setAttribute('content', `/public/v1/cms/media/${encodeURIComponent(seo.og_image_asset_id.trim())}`);
    }
    if (typeof seo.canonical === 'string' && seo.canonical.trim()) {
      let canonical = document.head.querySelector('link[rel="canonical"]');
      if (!canonical) {
        canonical = document.createElement('link');
        canonical.setAttribute('rel', 'canonical');
        document.head.appendChild(canonical);
      }
      canonical.setAttribute('href', seo.canonical.trim());
      ensurePropertyMeta('og:url').setAttribute('content', seo.canonical.trim());
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
      const response = await fetch(`/public/v1/cms/pages/${encodeURIComponent(slug)}?locale=${encodeURIComponent(publicLocale)}`, {
        method: 'GET',
        headers: {'Accept': 'application/json'},
        credentials: 'same-origin',
      });
      if (!response.ok) return;
      const page = await response.json();
      applySEO(page.seo);
      const hidden = Array.isArray(page.hidden_sections) ? page.hidden_sections : [];
      for (const id of hidden) {
        if (typeof id !== 'string' || !id.trim()) continue;
        const safe = window.CSS?.escape ? window.CSS.escape(id.trim()) : id.trim().replace(/[^a-zA-Z0-9_-]/g, '');
        document.querySelector(`[data-cms-section="${safe}"]`)?.remove();
      }
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

  if ('serviceWorker' in navigator) {
    window.addEventListener('load', () => {
      navigator.serviceWorker.register('/service-worker.js', {scope: '/'}).catch(() => {});
    });
  }

  translatePublicDocument();
  installLocaleSwitch();
  void loadPublishedDesign();
  void loadPublishedCMS();
})();

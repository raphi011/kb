import { initTheme } from './theme.js';
import { initResize } from './resize.js';
import { initToc } from './toc.js';
import { initSidebar } from './sidebar.js';
import { initCommandPalette } from './command-palette.js';
import { initHTMXHooks } from './htmx-hooks.js';
import { initCalendar } from './calendar.js';
import { initKeys } from './keys.js';
import { initZen } from './zen.js';
import { initLightbox } from './lightbox.js';
import { recordVisit } from './history.js';

initTheme();
initResize();
initToc();
initSidebar();
initCommandPalette();
initHTMXHooks();
initCalendar();
initKeys();
initZen();
initLightbox();

// Record initial page load if it's a note.
if (location.pathname.startsWith('/notes/')) {
  recordVisit(decodeURIComponent(location.pathname).replace(/^\/note\//, ''));
}

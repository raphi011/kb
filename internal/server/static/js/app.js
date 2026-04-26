import { initTheme } from './theme.js';
import { initResize } from './components/resize.js';
import { initToc } from './components/toc.js';
import { initSidebar } from './components/sidebar.js';
import { initCommandPalette } from './components/command-palette.js';
import { initHTMXHooks } from './htmx-hooks.js';
import { initCalendar } from './components/calendar.js';
import { initKeys } from './keys.js';
import { initBookmarks } from './components/bookmark.js';
import { initShare } from './components/share.js';
import { initZen } from './zen.js';
import { initLightbox } from './components/lightbox.js';
import { initFlashcards } from './components/flashcards.js';
import { initMarp } from './components/marp.js';
import { initToast } from './toast.js';
import { recordVisit } from './history.js';
import { initPreview } from './components/preview.js';
import { restorePanels } from './panels.js';

initTheme();
initResize();
initToc();
initSidebar();
initCommandPalette();
initHTMXHooks();
initCalendar();
initKeys();
initBookmarks();
initShare();
initZen();
initLightbox();
initFlashcards();
initMarp();
initToast();
initPreview();
restorePanels();

// Record initial page load if it's a note.
if (location.pathname.startsWith('/notes/')) {
  recordVisit(decodeURIComponent(location.pathname).replace(/^\/notes\//, ''));
}

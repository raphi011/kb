import { initTheme } from './theme.js';
import { initResize } from './resize.js';
import { initToc } from './toc.js';
import { initSidebar } from './sidebar.js';
import { initCommandPalette } from './command-palette.js';
import { initHTMXHooks } from './htmx-hooks.js';
import { initCalendar } from './calendar.js';
import { initKeys } from './keys.js';
import { initBookmarks } from './bookmark.js';
import { initZen } from './zen.js';
import { initLightbox } from './lightbox.js';
import { initFlashcards } from './flashcards.js';
import { initMarp } from './marp.js';
import { recordVisit } from './history.js';

initTheme();
initResize();
initToc();
initSidebar();
initCommandPalette();
initHTMXHooks();
initCalendar();
initKeys();
initBookmarks();
initZen();
initLightbox();
initFlashcards();
initMarp();

// Record initial page load if it's a note.
if (location.pathname.startsWith('/notes/')) {
  recordVisit(decodeURIComponent(location.pathname).replace(/^\/notes\//, ''));
}

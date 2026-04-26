import { initTheme } from './theme.js';
import { initResize } from './resize.js';
import { initToc } from './toc.js';
import { initSidebar } from './sidebar.js';
import { initCommandPalette } from './command-palette.js';
import { initHTMXHooks } from './htmx-hooks.js';
import { initCalendar } from './calendar.js';
import { initKeys } from './keys.js';
import { initBookmarks } from './bookmark.js';
import { initShare } from './share.js';
import { initZen } from './zen.js';
import { initLightbox } from './lightbox.js';
import { initFlashcards } from './flashcards.js';
import { initMarp } from './marp.js';
import { initToast } from './toast.js';
import { recordVisit } from './history.js';
import { initPreview } from './preview.js';

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

// Record initial page load if it's a note.
if (location.pathname.startsWith('/notes/')) {
  recordVisit(decodeURIComponent(location.pathname).replace(/^\/notes\//, ''));
}

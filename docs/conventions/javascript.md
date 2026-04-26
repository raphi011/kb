# JS Modules

## Rules

- ES2022+ baseline: `async/await`, `?.`, `??`, private fields (`#`), `AbortController`
- No build step -- native ES modules loaded via `<script type="module">`
- Components self-register with the registry on import
- `api()` for all `/api/*` calls; HTMX handles all HTML endpoints
- Events for cross-component communication; direct imports for utilities

## Directory Structure

```
static/js/
  app.js                  # Entry point: imports, global setup, HTMX lifecycle
  navigation.js           # SPA navigation helpers (navigateTo, fetchContent)
  theme.js, keys.js       # Global one-time setup
  lib/
    registry.js           # Component lifecycle registry
    api.js                # fetch wrapper for /api/* JSON endpoints
    events.js             # Event constants + emit/on helpers
    toast.js              # Toast notification API + HX-Trigger listener
    store.js              # localStorage persistence (get/set)
    manifest.js           # Client-side note manifest cache
  components/
    bookmark.js           # Self-registering feature components
    toc.js, resize.js, lightbox.js, marp.js, ...
    sidebar.js, calendar.js, command-palette.js, preview.js
```

## Patterns

### Registry Lifecycle

The registry (`lib/registry.js`) manages component init/destroy tied to HTMX swaps:

```js
import { registry } from '../lib/registry.js';

// Register: selector determines when init/destroy fire
registry.register('#my-widget', {
    init(root) {
        // root is the swapped container (or document on page load)
        const el = root.querySelector('#my-widget');
        // set up event listeners, state, etc.
    },
    destroy(root) {
        // cleanup before root is swapped out
        // abort controllers, timers, etc.
    }
});
```

Lifecycle flow:
1. Page load: `registry.init(document)` in `app.js`
2. HTMX swap: `registry.destroy(target)` on `htmx:beforeSwap`, then `registry.init(target)` on `htmx:afterSettle`
3. `init` only fires if `root.querySelector(selector)` matches

### Adding a New Component

1. Create `components/myfeature.js`
2. Import `registry`, call `registry.register(selector, { init, destroy })`
3. Export an `initMyFeature()` for one-time global setup (event delegation, etc.)
4. In `app.js`: `import './components/myfeature.js'` (for registry) and call `initMyFeature()`

See `components/bookmark.js` for a complete example: global click delegation in `initBookmarks()`, `api()` calls in async handlers, `registry.register()` for post-swap icon updates.

### `api()` Wrapper

Located in `lib/api.js`. Use for all `/api/*` calls:

```js
import { api, ApiError } from '../lib/api.js';

// Basic usage
const data = await api('GET', '/api/flashcards/stats');

// With body
await api('PUT', `/api/bookmarks/${path}`);

// With AbortController
const ctrl = new AbortController();
await api('GET', '/api/share/note.md', { signal: ctrl.signal });

// Returns null for 204, parsed JSON otherwise
// Redirects to /login on 401
// Throws ApiError on non-ok responses
```

### Event System

`lib/events.js` provides typed event constants and helpers:

```js
import { Events, emit, on } from '../lib/events.js';

// Listen (returns unsubscribe function)
const off = on(Events.MANIFEST_CHANGED, (e) => { /* ... */ });
off(); // cleanup

// Emit
emit(Events.DATE_FILTER_SET, { date: '2024-01-15' });
```

Use events for cross-component communication. Use direct imports for utilities (`api`, `toast`, `store`).

### Store (UI Persistence)

`lib/store.js` persists UI state in localStorage under `zk-ui`:

```js
import { get, set } from '../lib/store.js';

set('panel.toc', false);     // collapse TOC panel
const open = get('panel.toc'); // false
```

### Toast Notifications

```js
import { toast } from '../lib/toast.js';

toast('Saved!');                          // info
toast('Something broke', true);           // error
toast('Undo?', false, [                   // with action button
    { label: 'Undo', onClick: () => undoThing() }
]);
```

Server-triggered toasts arrive via `HX-Trigger` header and are handled automatically by `toast.js`.

## Anti-patterns

- **Don't build HTMX HTML in JS.** Use `htmx.ajax()` for programmatic HTMX requests, or `ContentLink` in templ for links.
- **Don't duplicate server state.** The manifest cache is the exception; everything else comes from the server.
- **Don't use `querySelector` outside of `init()`.** Elements may not exist yet or may be stale after a swap.
- **Don't forget `destroy()`.** Leaking event listeners or timers across swaps causes bugs.
- **Don't add global event listeners in `init()`.** Use `initMyFeature()` for delegation that lives for the page lifetime; use `init()`/`destroy()` only for swap-scoped setup.

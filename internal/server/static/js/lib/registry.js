// internal/server/static/js/lib/registry.js

class ComponentRegistry {
  #components = [];

  /**
   * Register a component with a CSS selector and lifecycle hooks.
   * - init(root): called when selector matches inside root after HTMX swap or page load
   * - destroy(root): called before root is swapped out, for cleanup (abort controllers, timers)
   */
  register(selector, { init, destroy } = {}) {
    this.#components.push({ selector, init, destroy });
  }

  /**
   * Initialize all registered components whose selector matches inside root.
   * Called after HTMX swaps and on initial page load.
   */
  init(root = document) {
    for (const c of this.#components) {
      if (root.querySelector(c.selector) && c.init) {
        c.init(root);
      }
    }
  }

  /**
   * Destroy all registered components whose selector matches inside root.
   * Called before HTMX swaps out content, for cleanup.
   */
  destroy(root = document) {
    for (const c of this.#components) {
      if (root.querySelector(c.selector) && c.destroy) {
        c.destroy(root);
      }
    }
  }
}

export const registry = new ComponentRegistry();

// internal/server/static/js/lib/api.js

export class ApiError extends Error {
  constructor(status, body) {
    super(`API error ${status}`);
    this.status = status;
    this.body = body;
  }
}

/**
 * Fetch wrapper for /api/* calls.
 *
 * - Redirects to /login on 401
 * - Returns parsed JSON (or null for 204)
 * - Throws ApiError on non-ok responses
 * - Supports AbortController via signal option
 *
 * @param {'GET'|'POST'|'PUT'|'DELETE'|'PATCH'} method
 * @param {string} path
 * @param {{ body?: unknown, signal?: AbortSignal }} options
 * @returns {Promise<unknown>}
 */
export async function api(method, path, { body, signal } = {}) {
  const opts = { method, signal };

  if (body !== undefined) {
    opts.headers = { 'Content-Type': 'application/json' };
    opts.body = JSON.stringify(body);
  }

  const res = await fetch(path, opts);

  if (res.status === 401) {
    window.location.href = '/login';
    throw new ApiError(401, 'unauthorized');
  }

  if (!res.ok) {
    throw new ApiError(res.status, await res.text());
  }

  if (res.status === 204) return null;

  return res.json();
}

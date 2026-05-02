package server

import (
	"hash/fnv"
	"sync"

	"github.com/raphi011/kb/internal/markdown"
)

// viewCache holds pre-rendered HTML alongside the note metadata cache.
// Render entries survive across cache refreshes if the content hasn't changed
// (validated by content hash on read). This avoids cold-start re-rendering
// after every git push while still invalidating stale data atomically.
type viewCache struct {
	mu      sync.RWMutex
	entries map[string]renderEntry
}

type renderEntry struct {
	html        string
	headings    []markdown.Heading
	contentHash uint64
}

func newViewCache() *viewCache {
	return &viewCache{entries: make(map[string]renderEntry)}
}

func (vc *viewCache) get(path string, content []byte) (renderEntry, bool) {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	entry, ok := vc.entries[path]
	if !ok {
		return renderEntry{}, false
	}
	if entry.contentHash != hashContent(content) {
		return renderEntry{}, false
	}
	return entry, true
}

func (vc *viewCache) put(path string, content []byte, entry renderEntry) {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	entry.contentHash = hashContent(content)
	vc.entries[path] = entry
}

func (vc *viewCache) headings(path string) []markdown.Heading {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	if entry, ok := vc.entries[path]; ok {
		return entry.headings
	}
	return nil
}

func (vc *viewCache) clear() {
	vc.mu.Lock()
	defer vc.mu.Unlock()

	vc.entries = make(map[string]renderEntry)
}

func hashContent(b []byte) uint64 {
	h := fnv.New64a()
	h.Write(b)
	return h.Sum64()
}

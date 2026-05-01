package server

import (
	"hash/fnv"
	"sync"

	"github.com/raphi011/kb/internal/markdown"
)

type renderCacheEntry struct {
	html        string
	headings    []markdown.Heading
	contentHash uint64
}

type renderCache struct {
	mu      sync.RWMutex
	entries map[string]renderCacheEntry
}

func newRenderCache() *renderCache {
	return &renderCache{entries: make(map[string]renderCacheEntry)}
}

func (rc *renderCache) get(path string, content []byte) (renderCacheEntry, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	entry, ok := rc.entries[path]
	if !ok {
		return renderCacheEntry{}, false
	}
	if entry.contentHash != hashContent(content) {
		return renderCacheEntry{}, false
	}
	return entry, true
}

func (rc *renderCache) put(path string, content []byte, entry renderCacheEntry) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	entry.contentHash = hashContent(content)
	rc.entries[path] = entry
}

func (rc *renderCache) headings(path string) []markdown.Heading {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if entry, ok := rc.entries[path]; ok {
		return entry.headings
	}
	return nil
}

func (rc *renderCache) clear() {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.entries = make(map[string]renderCacheEntry)
}

func hashContent(b []byte) uint64 {
	h := fnv.New64a()
	h.Write(b)
	return h.Sum64()
}

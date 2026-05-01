package server

import (
	"testing"
)

func TestRenderCache_MissOnEmpty(t *testing.T) {
	rc := newRenderCache()
	_, ok := rc.get("notes/foo.md", []byte("content"))
	if ok {
		t.Fatal("expected miss on empty cache")
	}
}

func TestRenderCache_HitAfterPut(t *testing.T) {
	rc := newRenderCache()
	content := []byte("# Hello")
	entry := renderCacheEntry{html: "<h1>Hello</h1>"}
	rc.put("notes/foo.md", content, entry)

	got, ok := rc.get("notes/foo.md", content)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.html != entry.html {
		t.Fatalf("expected html %q, got %q", entry.html, got.html)
	}
}

func TestRenderCache_MissAfterContentChange(t *testing.T) {
	rc := newRenderCache()
	rc.put("notes/foo.md", []byte("original"), renderCacheEntry{html: "<p>original</p>"})

	_, ok := rc.get("notes/foo.md", []byte("changed"))
	if ok {
		t.Fatal("expected miss after content change")
	}
}

func TestRenderCache_ClearRemovesAll(t *testing.T) {
	rc := newRenderCache()
	rc.put("notes/a.md", []byte("a"), renderCacheEntry{html: "<p>a</p>"})
	rc.put("notes/b.md", []byte("b"), renderCacheEntry{html: "<p>b</p>"})

	rc.clear()

	if _, ok := rc.get("notes/a.md", []byte("a")); ok {
		t.Fatal("expected miss after clear for a.md")
	}
	if _, ok := rc.get("notes/b.md", []byte("b")); ok {
		t.Fatal("expected miss after clear for b.md")
	}
}

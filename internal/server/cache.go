package server

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/raphi011/kb/internal/index"
)

type FileNode struct {
	Name     string
	Path     string
	IsDir    bool
	IsActive bool
	IsOpen   bool
	Children []*FileNode
}

type BreadcrumbSegment struct {
	Name       string
	FolderPath string
}

type FolderEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Title string `json:"title,omitempty"`
	IsDir bool   `json:"isDir"`
}

type noteCache struct {
	notes        []index.Note
	tags         []index.Tag
	manifestJSON string
	lookup       map[string]string
	notesByPath  map[string]*index.Note
}

func buildNoteCache(store Store) (*noteCache, error) {
	notes, err := store.AllNotes()
	if err != nil {
		return nil, fmt.Errorf("load notes: %w", err)
	}
	tags, err := store.AllTags()
	if err != nil {
		return nil, fmt.Errorf("load tags: %w", err)
	}

	lookup := make(map[string]string, len(notes)*2)
	byPath := make(map[string]*index.Note, len(notes))
	for i, n := range notes {
		stem := strings.TrimSuffix(n.Path[strings.LastIndex(n.Path, "/")+1:], ".md")
		lookup[stem] = n.Path
		lookup[strings.TrimSuffix(n.Path, ".md")] = n.Path
		byPath[n.Path] = &notes[i]
	}

	return &noteCache{
		notes:        notes,
		tags:         tags,
		manifestJSON: buildManifestJSON(notes),
		lookup:       lookup,
		notesByPath:  byPath,
	}, nil
}

func buildManifestJSON(notes []index.Note) string {
	type entry struct {
		Title string   `json:"title"`
		Path  string   `json:"path"`
		Tags  []string `json:"tags"`
		Mod   int64    `json:"mod"`
	}
	entries := make([]entry, len(notes))
	for i, n := range notes {
		tags := n.Tags
		if tags == nil {
			tags = []string{}
		}
		entries[i] = entry{Title: n.Title, Path: n.Path, Tags: tags, Mod: n.Modified.Unix()}
	}
	b, _ := json.Marshal(entries)
	return string(b)
}

func buildBreadcrumbs(notePath string) []BreadcrumbSegment {
	parts := strings.Split(notePath, "/")
	dirs := parts[:len(parts)-1]
	crumbs := make([]BreadcrumbSegment, len(dirs))
	for i, name := range dirs {
		crumbs[i] = BreadcrumbSegment{
			Name:       name,
			FolderPath: strings.Join(parts[:i+1], "/"),
		}
	}
	return crumbs
}

func buildTree(notes []index.Note, activePath string) []*FileNode {
	type treeEntry struct {
		node     *FileNode
		children map[string]*treeEntry
	}
	root := &treeEntry{children: map[string]*treeEntry{}}

	for _, n := range notes {
		parts := strings.Split(n.Path, "/")
		cur := root
		for i, part := range parts {
			isLast := i == len(parts)-1
			if _, exists := cur.children[part]; !exists {
				var node *FileNode
				if !isLast {
					node = &FileNode{Name: part, IsDir: true}
				} else {
					node = &FileNode{
						Name:     n.Title,
						Path:     n.Path,
						IsActive: n.Path == activePath,
					}
				}
				cur.children[part] = &treeEntry{node: node, children: map[string]*treeEntry{}}
			}
			cur = cur.children[part]
		}
	}

	var flatten func(*treeEntry) ([]*FileNode, bool)
	flatten = func(e *treeEntry) ([]*FileNode, bool) {
		var dirKeys, fileKeys []string
		for k, child := range e.children {
			if child.node.IsDir {
				dirKeys = append(dirKeys, k)
			} else {
				fileKeys = append(fileKeys, k)
			}
		}
		sort.Strings(dirKeys)
		sort.Strings(fileKeys)

		anyActive := false
		nodes := make([]*FileNode, 0, len(e.children))
		for _, k := range dirKeys {
			child := e.children[k]
			child.node.Children, child.node.IsOpen = flatten(child)
			if child.node.IsOpen {
				anyActive = true
			}
			nodes = append(nodes, child.node)
		}
		for _, k := range fileKeys {
			n := e.children[k].node
			if n.IsActive {
				anyActive = true
			}
			nodes = append(nodes, n)
		}
		return nodes, anyActive
	}

	nodes, _ := flatten(root)
	return nodes
}

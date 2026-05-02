package index

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/raphi011/kb/internal/markdown"
)

// Tx wraps a *sql.Tx and provides the same write operations as DB
// but within a single transaction.
type Tx struct {
	tx *sql.Tx
}

func (t *Tx) UpsertNote(n Note) error {
	var metadataJSON []byte
	if n.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(n.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
	}

	_, err := t.tx.Exec(`
		INSERT INTO notes (path, title, body, lead, word_count, is_marp, has_flashcards, created, modified, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			title = excluded.title,
			body = excluded.body,
			lead = excluded.lead,
			word_count = excluded.word_count,
			is_marp = excluded.is_marp,
			has_flashcards = excluded.has_flashcards,
			created = excluded.created,
			modified = excluded.modified,
			metadata = excluded.metadata`,
		n.Path, n.Title, n.Body, n.Lead, n.WordCount, n.IsMarp, n.HasFlashcards,
		formatTime(n.Created), formatTime(n.Modified),
		string(metadataJSON),
	)
	if err != nil {
		return fmt.Errorf("upsert note: %w", mapDBError(err))
	}
	return nil
}

func (t *Tx) DeleteNote(path string) error {
	_, err := t.tx.Exec("DELETE FROM notes WHERE path = ?", path)
	if err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	return nil
}

func (t *Tx) SetTags(path string, tags []string) error {
	if _, err := t.tx.Exec("DELETE FROM tags WHERE path = ?", path); err != nil {
		return fmt.Errorf("delete tags: %w", err)
	}
	for _, tag := range tags {
		if _, err := t.tx.Exec("INSERT INTO tags (name, path) VALUES (?, ?)", tag, path); err != nil {
			return fmt.Errorf("insert tag: %w", mapDBError(err))
		}
	}
	return nil
}

func (t *Tx) SetLinks(path string, links []Link) error {
	if _, err := t.tx.Exec("DELETE FROM links WHERE source_path = ?", path); err != nil {
		return fmt.Errorf("delete links: %w", err)
	}
	for _, l := range links {
		if _, err := t.tx.Exec(
			"INSERT INTO links (source_path, target_path, title, external) VALUES (?, ?, ?, ?)",
			path, l.TargetPath, l.Title, l.External,
		); err != nil {
			return fmt.Errorf("insert link: %w", mapDBError(err))
		}
	}
	return nil
}

func (t *Tx) UpsertFlashcards(notePath string, cards []markdown.ParsedCard) error {
	// Build set of current hashes.
	newHashes := make(map[string]bool, len(cards))
	for _, c := range cards {
		newHashes[c.Hash] = true
	}

	// Delete cards that are no longer present.
	rows, err := t.tx.Query("SELECT card_hash FROM flashcards WHERE note_path = ?", notePath)
	if err != nil {
		return fmt.Errorf("query existing flashcards: %w", err)
	}
	var toDelete []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			rows.Close()
			return fmt.Errorf("scan flashcard hash: %w", err)
		}
		if !newHashes[h] {
			toDelete = append(toDelete, h)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate flashcard hashes: %w", err)
	}

	for _, h := range toDelete {
		if _, err := t.tx.Exec("DELETE FROM flashcards WHERE card_hash = ?", h); err != nil {
			return fmt.Errorf("delete flashcard: %w", err)
		}
	}

	// Upsert current cards.
	for _, c := range cards {
		reversed := 0
		if c.Reversed {
			reversed = 1
		}
		_, err := t.tx.Exec(`
			INSERT INTO flashcards (card_hash, note_path, kind, question, answer, reversed, ord)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(card_hash) DO UPDATE SET
				note_path = excluded.note_path,
				last_seen = CURRENT_TIMESTAMP,
				ord = excluded.ord,
				question = excluded.question,
				answer = excluded.answer`,
			c.Hash, notePath, string(c.Kind), c.Question, c.Answer, reversed, c.Ord,
		)
		if err != nil {
			return fmt.Errorf("upsert flashcard: %w", mapDBError(err))
		}
	}
	return nil
}

func (t *Tx) SetMeta(key, value string) error {
	_, err := t.tx.Exec(
		"INSERT INTO index_meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set meta %q: %w", key, err)
	}
	return nil
}

// ResolveLinks updates target_path for non-external links by matching
// wiki-link stems to actual note paths within the transaction.
func (t *Tx) ResolveLinks() error {
	rows, err := t.tx.Query("SELECT path FROM notes")
	if err != nil {
		return fmt.Errorf("query notes for resolve: %w", err)
	}
	defer rows.Close()

	lookup := make(map[string]string)
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return err
		}
		stem := path
		if idx := strings.LastIndex(path, "/"); idx >= 0 {
			stem = path[idx+1:]
		}
		stem = strings.TrimSuffix(stem, ".md")
		if _, exists := lookup[stem]; !exists {
			lookup[stem] = path
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Find unresolved links.
	linkRows, err := t.tx.Query(`
		SELECT l.source_path, l.target_path
		FROM links l
		LEFT JOIN notes n ON n.path = l.target_path
		WHERE l.external = 0 AND n.path IS NULL`)
	if err != nil {
		return fmt.Errorf("query unresolved links: %w", err)
	}
	defer linkRows.Close()

	type unresolvedLink struct {
		sourcePath string
		targetPath string
	}
	var unresolved []unresolvedLink
	for linkRows.Next() {
		var l unresolvedLink
		if err := linkRows.Scan(&l.sourcePath, &l.targetPath); err != nil {
			return err
		}
		unresolved = append(unresolved, l)
	}
	if err := linkRows.Err(); err != nil {
		return err
	}

	for _, l := range unresolved {
		stem := strings.TrimSuffix(l.targetPath, ".md")
		if fullPath, ok := lookup[stem]; ok {
			if _, err := t.tx.Exec(
				"UPDATE links SET target_path = ? WHERE source_path = ? AND target_path = ?",
				fullPath, l.sourcePath, l.targetPath,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

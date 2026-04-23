package index

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (d *DB) AllNotes() ([]Note, error) {
	rows, err := d.db.Query(`
		SELECT n.path, n.title, n.lead, n.word_count,
		       n.created, n.modified, n.metadata,
		       COALESCE(GROUP_CONCAT(t.name, char(1)), '') AS tags
		FROM notes n
		LEFT JOIN tags t ON t.path = n.path
		GROUP BY n.path
		ORDER BY n.path`)
	if err != nil {
		return nil, fmt.Errorf("query all notes: %w", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		n, err := scanNoteRow(rows)
		if err != nil {
			return nil, err
		}
		notes = append(notes, *n)
	}
	return notes, rows.Err()
}

func (d *DB) AllTags() ([]Tag, error) {
	rows, err := d.db.Query(`
		SELECT name, COUNT(*) AS note_count
		FROM tags
		GROUP BY name
		ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query all tags: %w", err)
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.Name, &t.NoteCount); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (d *DB) OutgoingLinks(path string) ([]Link, error) {
	rows, err := d.db.Query(`
		SELECT source_path, target_path, COALESCE(title, ''), external
		FROM links
		WHERE source_path = ?
		ORDER BY external, title`, path)
	if err != nil {
		return nil, fmt.Errorf("query outgoing links: %w", err)
	}
	defer rows.Close()

	var links []Link
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.SourcePath, &l.TargetPath, &l.Title, &l.External); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

func (d *DB) Backlinks(path string) ([]Link, error) {
	rows, err := d.db.Query(`
		SELECT l.source_path, COALESCE(n.title, ''), l.target_path, COALESCE(l.title, ''), l.external
		FROM links l
		LEFT JOIN notes n ON n.path = l.source_path
		WHERE l.target_path = ? AND l.external = 0
		ORDER BY n.title`, path)
	if err != nil {
		return nil, fmt.Errorf("query backlinks: %w", err)
	}
	defer rows.Close()

	var links []Link
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.SourcePath, &l.SourceTitle, &l.TargetPath, &l.Title, &l.External); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

func (d *DB) ActivityDays(year, month int) (map[int]bool, error) {
	ym := fmt.Sprintf("%04d-%02d", year, month)
	rows, err := d.db.Query(`
		SELECT DISTINCT CAST(strftime('%d', created) AS INTEGER)
		FROM notes WHERE strftime('%Y-%m', created) = ?
		UNION
		SELECT DISTINCT CAST(strftime('%d', modified) AS INTEGER)
		FROM notes WHERE strftime('%Y-%m', modified) = ?`, ym, ym)
	if err != nil {
		return nil, fmt.Errorf("query activity days: %w", err)
	}
	defer rows.Close()

	days := make(map[int]bool)
	for rows.Next() {
		var day int
		if err := rows.Scan(&day); err != nil {
			return nil, err
		}
		days[day] = true
	}
	return days, rows.Err()
}

func (d *DB) NotesByDate(date string) ([]Note, error) {
	rows, err := d.db.Query(`
		SELECT n.path, n.title, n.lead, n.word_count,
		       n.created, n.modified, n.metadata,
		       COALESCE(GROUP_CONCAT(t.name, char(1)), '') AS tags
		FROM notes n
		LEFT JOIN tags t ON t.path = n.path
		WHERE DATE(n.created) = ? OR DATE(n.modified) = ?
		GROUP BY n.path
		ORDER BY n.path`, date, date)
	if err != nil {
		return nil, fmt.Errorf("query notes by date: %w", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		n, err := scanNoteRow(rows)
		if err != nil {
			return nil, err
		}
		notes = append(notes, *n)
	}
	return notes, rows.Err()
}

func scanNoteRow(rows interface{ Scan(...any) error }) (*Note, error) {
	var (
		n           Note
		createdRaw  string
		modifiedRaw string
		metadataRaw string
		tagsRaw     string
	)
	if err := rows.Scan(&n.Path, &n.Title, &n.Lead, &n.WordCount,
		&createdRaw, &modifiedRaw, &metadataRaw, &tagsRaw); err != nil {
		return nil, err
	}
	if createdRaw != "" {
		n.Created, _ = time.Parse(time.RFC3339, createdRaw)
	}
	if modifiedRaw != "" {
		n.Modified, _ = time.Parse(time.RFC3339, modifiedRaw)
	}
	if metadataRaw != "" {
		_ = json.Unmarshal([]byte(metadataRaw), &n.Metadata)
	}
	if tagsRaw != "" {
		n.Tags = strings.Split(tagsRaw, "\x01")
	}
	return &n, nil
}

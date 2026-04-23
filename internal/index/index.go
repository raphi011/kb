package index

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("not found")

type Note struct {
	Path      string
	Title     string
	Body      string
	Lead      string
	WordCount int
	Created   time.Time
	Modified  time.Time
	Metadata  map[string]any
	Tags      []string
}

type Tag struct {
	Name      string
	NoteCount int
}

type Link struct {
	SourcePath  string
	TargetPath  string
	Title       string
	External    bool
	SourceTitle string // populated by backlinks query
}

type DB struct {
	db *sql.DB
}

func Open(dbPath string) (*DB, error) {
	dsn := dbPath
	if dbPath != ":memory:" {
		dsn = fmt.Sprintf("file:%s?_pragma=journal_mode(wal)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(on)", dbPath)
	} else {
		dsn = "file::memory:?_pragma=foreign_keys(on)"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open index db: %w", err)
	}

	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return &DB{db: db}, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) UpsertNote(n Note) error {
	var metadataJSON []byte
	if n.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(n.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
	}

	_, err := d.db.Exec(`
		INSERT INTO notes (path, title, body, lead, word_count, created, modified, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			title = excluded.title,
			body = excluded.body,
			lead = excluded.lead,
			word_count = excluded.word_count,
			created = excluded.created,
			modified = excluded.modified,
			metadata = excluded.metadata`,
		n.Path, n.Title, n.Body, n.Lead, n.WordCount,
		formatTime(n.Created), formatTime(n.Modified),
		string(metadataJSON),
	)
	return err
}

func (d *DB) DeleteNote(path string) error {
	_, err := d.db.Exec("DELETE FROM notes WHERE path = ?", path)
	return err
}

func (d *DB) SetTags(path string, tags []string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM tags WHERE path = ?", path); err != nil {
		return err
	}
	for _, tag := range tags {
		if _, err := tx.Exec("INSERT INTO tags (name, path) VALUES (?, ?)", tag, path); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) SetLinks(path string, links []Link) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM links WHERE source_path = ?", path); err != nil {
		return err
	}
	for _, l := range links {
		if _, err := tx.Exec(
			"INSERT INTO links (source_path, target_path, title, external) VALUES (?, ?, ?, ?)",
			path, l.TargetPath, l.Title, l.External,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) SetMeta(key, value string) error {
	_, err := d.db.Exec(
		"INSERT INTO index_meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	)
	return err
}

func (d *DB) GetMeta(key string) (string, error) {
	var value string
	err := d.db.QueryRow("SELECT value FROM index_meta WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (d *DB) NoteByPath(path string) (*Note, error) {
	row := d.db.QueryRow(`
		SELECT path, title, body, lead, word_count, created, modified, metadata
		FROM notes WHERE path = ?`, path)

	n, err := scanNote(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	tags, err := d.tagsForPath(path)
	if err != nil {
		return nil, err
	}
	n.Tags = tags

	return n, nil
}

func (d *DB) tagsForPath(path string) ([]string, error) {
	rows, err := d.db.Query("SELECT name FROM tags WHERE path = ? ORDER BY name", path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tags = append(tags, name)
	}
	return tags, rows.Err()
}

func scanNote(row *sql.Row) (*Note, error) {
	var (
		n           Note
		createdRaw  sql.NullString
		modifiedRaw sql.NullString
		metadataRaw sql.NullString
	)
	if err := row.Scan(&n.Path, &n.Title, &n.Body, &n.Lead, &n.WordCount,
		&createdRaw, &modifiedRaw, &metadataRaw); err != nil {
		return nil, err
	}
	if createdRaw.Valid {
		var err error
		n.Created, err = time.Parse(time.RFC3339, createdRaw.String)
		if err != nil {
			slog.Warn("invalid created timestamp", "path", n.Path, "raw", createdRaw.String, "error", err)
		}
	}
	if modifiedRaw.Valid {
		var err error
		n.Modified, err = time.Parse(time.RFC3339, modifiedRaw.String)
		if err != nil {
			slog.Warn("invalid modified timestamp", "path", n.Path, "raw", modifiedRaw.String, "error", err)
		}
	}
	if metadataRaw.Valid && metadataRaw.String != "" {
		if err := json.Unmarshal([]byte(metadataRaw.String), &n.Metadata); err != nil {
			slog.Warn("invalid metadata JSON", "path", n.Path, "error", err)
		}
	}
	return &n, nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

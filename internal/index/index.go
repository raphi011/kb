package index

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
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
	IsMarp    bool
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

	// Run migrations — ignore "duplicate column" errors from already-migrated DBs.
	for _, stmt := range strings.Split(migrationsSQL, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			if !strings.Contains(err.Error(), "duplicate column") {
				db.Close()
				return nil, fmt.Errorf("migration: %w", err)
			}
		}
	}

	return &DB{db: db}, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

// WithTx executes fn within a single database transaction.
// If fn returns an error, the transaction is rolled back.
func (d *DB) WithTx(fn func(tx *Tx) error) error {
	sqlTx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer sqlTx.Rollback()

	if err := fn(&Tx{tx: sqlTx}); err != nil {
		return err
	}
	return sqlTx.Commit()
}

func (d *DB) UpsertNote(n Note) error {
	return d.WithTx(func(tx *Tx) error {
		return tx.UpsertNote(n)
	})
}

func (d *DB) DeleteNote(path string) error {
	return d.WithTx(func(tx *Tx) error {
		return tx.DeleteNote(path)
	})
}

func (d *DB) SetTags(path string, tags []string) error {
	return d.WithTx(func(tx *Tx) error {
		return tx.SetTags(path, tags)
	})
}

func (d *DB) SetLinks(path string, links []Link) error {
	return d.WithTx(func(tx *Tx) error {
		return tx.SetLinks(path, links)
	})
}


func (d *DB) SetMeta(key, value string) error {
	_, err := d.db.Exec(
		"INSERT INTO index_meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set meta %q: %w", key, err)
	}
	return nil
}

func (d *DB) GetMeta(key string) (string, error) {
	var value string
	err := d.db.QueryRow("SELECT value FROM index_meta WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// IndexSHA returns the currently indexed commit hash.
func (d *DB) IndexSHA() (string, error) {
	return d.GetMeta("head_commit")
}

func (d *DB) NoteByPath(path string) (*Note, error) {
	row := d.db.QueryRow(`
		SELECT path, title, body, lead, word_count, is_marp, created, modified, metadata
		FROM notes WHERE path = ?`, path)

	n, err := scanNote(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query note %q: %w", path, err)
	}

	tags, err := d.tagsForPath(path)
	if err != nil {
		return nil, fmt.Errorf("tags for note %q: %w", path, err)
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
	if err := row.Scan(&n.Path, &n.Title, &n.Body, &n.Lead, &n.WordCount, &n.IsMarp,
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

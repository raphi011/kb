package index

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
)

func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate share token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ShareNote creates a share link for the given note path.
// If the note is already shared, returns the existing token.
func (d *DB) ShareNote(path string) (string, error) {
	var existing string
	err := d.db.QueryRow("SELECT token FROM shared_notes WHERE note_path = ?", path).Scan(&existing)
	if err == nil {
		return existing, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}

	token, err := generateToken()
	if err != nil {
		return "", err
	}
	_, err = d.db.Exec("INSERT INTO shared_notes (token, note_path) VALUES (?, ?)", token, path)
	if err != nil {
		return "", err
	}
	return token, nil
}

// UnshareNote revokes the share link for the given note path.
func (d *DB) UnshareNote(path string) error {
	_, err := d.db.Exec("DELETE FROM shared_notes WHERE note_path = ?", path)
	return err
}

// ShareTokenForNote returns the share token for a note, or empty string if not shared.
func (d *DB) ShareTokenForNote(path string) (string, error) {
	var token string
	err := d.db.QueryRow("SELECT token FROM shared_notes WHERE note_path = ?", path).Scan(&token)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return token, err
}

// NotePathForShareToken returns the note path for a share token.
func (d *DB) NotePathForShareToken(token string) (string, error) {
	var path string
	err := d.db.QueryRow("SELECT note_path FROM shared_notes WHERE token = ?", token).Scan(&path)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return path, err
}

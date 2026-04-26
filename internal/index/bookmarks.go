package index

import "fmt"

func (d *DB) AddBookmark(path string) error {
	_, err := d.db.Exec(
		"INSERT INTO bookmarks (path) VALUES (?) ON CONFLICT(path) DO NOTHING",
		path,
	)
	if err != nil {
		return fmt.Errorf("add bookmark: %w", mapDBError(err))
	}
	return nil
}

func (d *DB) RemoveBookmark(path string) error {
	_, err := d.db.Exec("DELETE FROM bookmarks WHERE path = ?", path)
	if err != nil {
		return fmt.Errorf("remove bookmark: %w", err)
	}
	return nil
}

func (d *DB) BookmarkedPaths() ([]string, error) {
	rows, err := d.db.Query("SELECT path FROM bookmarks ORDER BY created DESC")
	if err != nil {
		return nil, fmt.Errorf("query bookmarks: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scan bookmark: %w", err)
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

func (d *DB) IsBookmarked(path string) (bool, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM bookmarks WHERE path = ?", path).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check bookmark: %w", err)
	}
	return count > 0, nil
}

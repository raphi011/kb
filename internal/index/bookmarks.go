package index

func (d *DB) AddBookmark(path string) error {
	_, err := d.db.Exec(
		"INSERT INTO bookmarks (path) VALUES (?) ON CONFLICT(path) DO NOTHING",
		path,
	)
	return err
}

func (d *DB) RemoveBookmark(path string) error {
	_, err := d.db.Exec("DELETE FROM bookmarks WHERE path = ?", path)
	return err
}

func (d *DB) BookmarkedPaths() ([]string, error) {
	rows, err := d.db.Query("SELECT path FROM bookmarks ORDER BY created DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

func (d *DB) IsBookmarked(path string) (bool, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM bookmarks WHERE path = ?", path).Scan(&count)
	return count > 0, err
}

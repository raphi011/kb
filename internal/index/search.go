package index

import (
	"fmt"
	"strings"

	"github.com/raphi011/kb/internal/markdown"
)

func (d *DB) Search(q string, tags []string) ([]Note, error) {
	if q == "" && len(tags) == 0 {
		return nil, nil
	}
	if q != "" {
		return d.searchFTS(q, tags)
	}
	return d.searchByTags(tags)
}

func (d *DB) searchFTS(q string, tags []string) ([]Note, error) {
	fts := markdown.ConvertQuery(q)
	if fts == "" {
		return nil, nil
	}

	// bm25() is only valid in a query where the FTS table appears directly in
	// FROM without being mixed with non-FTS JOINs. We compute ranked rowids in
	// a subquery, then join to notes/tags for the full row data.
	var tagClauses []string
	var args []any

	args = append(args, fts) // for the inner subquery MATCH

	for _, tag := range tags {
		tagClauses = append(tagClauses, `n.path IN (SELECT path FROM tags WHERE name = ?)`)
		args = append(args, tag)
	}

	outerWhere := ""
	if len(tagClauses) > 0 {
		outerWhere = "WHERE " + strings.Join(tagClauses, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT n.path, n.title, n.lead, n.word_count,
		       n.created, n.modified, n.metadata,
		       COALESCE(GROUP_CONCAT(t.name, char(1)), '') AS tags
		FROM (
			SELECT rowid, bm25(notes_fts, 10.0, 1.0, 5.0) AS rank -- weights: title=10, body=1, path=5
			FROM notes_fts
			WHERE notes_fts MATCH ?
			ORDER BY rank
			LIMIT 200
		) AS ranked
		JOIN notes n ON n.rowid = ranked.rowid
		LEFT JOIN tags t ON t.path = n.path
		%s
		GROUP BY n.path
		ORDER BY ranked.rank`, outerWhere)

	return d.execSearch(query, args)
}

func (d *DB) searchByTags(tags []string) ([]Note, error) {
	var clauses []string
	var args []any

	for _, tag := range tags {
		clauses = append(clauses, `n.path IN (SELECT path FROM tags WHERE name = ?)`)
		args = append(args, tag)
	}

	query := fmt.Sprintf(`
		SELECT n.path, n.title, n.lead, n.word_count,
		       n.created, n.modified, n.metadata,
		       COALESCE(GROUP_CONCAT(t.name, char(1)), '') AS tags
		FROM notes n
		LEFT JOIN tags t ON t.path = n.path
		WHERE %s
		GROUP BY n.path
		ORDER BY n.path
		LIMIT 200`, strings.Join(clauses, " AND "))

	return d.execSearch(query, args)
}

func (d *DB) execSearch(query string, args []any) ([]Note, error) {
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
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

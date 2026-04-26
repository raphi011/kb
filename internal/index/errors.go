package index

import (
	"errors"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// mapDBError translates SQLite constraint errors to domain errors.
// FK violations → ErrNotFound (the referenced row doesn't exist).
func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	if sqlErr, ok := errors.AsType[*sqlite.Error](err); ok {
		if sqlErr.Code() == sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY {
			return ErrNotFound
		}
	}
	return err
}

package index

import (
	"errors"
	"fmt"
	"testing"
)

func TestMapDBError_Nil(t *testing.T) {
	if err := mapDBError(nil); err != nil {
		t.Errorf("mapDBError(nil) = %v, want nil", err)
	}
}

func TestMapDBError_PassThrough(t *testing.T) {
	orig := fmt.Errorf("some other error")
	err := mapDBError(orig)
	if err != orig {
		t.Errorf("mapDBError should pass through non-sqlite errors, got %v", err)
	}
}

func TestMapDBError_WrappedPassThrough(t *testing.T) {
	wrapped := fmt.Errorf("exec: %w", fmt.Errorf("not a sqlite error"))
	err := mapDBError(wrapped)
	if err != wrapped {
		t.Errorf("wrapped non-sqlite error should pass through")
	}
}

func TestAddBookmark_NonexistentNote_ReturnsErrNotFound(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = db.AddBookmark("nonexistent/note.md")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("AddBookmark(nonexistent) = %v, want ErrNotFound", err)
	}
}

func TestShareNote_NonexistentNote_ReturnsErrNotFound(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.ShareNote("nonexistent/note.md")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("ShareNote(nonexistent) = %v, want ErrNotFound", err)
	}
}

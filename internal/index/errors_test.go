package index

import (
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

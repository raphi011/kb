package index

const schemaSQL = `
CREATE TABLE IF NOT EXISTS notes (
    path        TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    body        TEXT NOT NULL,
    lead        TEXT,
    word_count  INTEGER NOT NULL,
    is_marp     BOOLEAN NOT NULL DEFAULT 0,
    created     DATETIME,
    modified    DATETIME,
    metadata    TEXT
);

CREATE VIRTUAL TABLE IF NOT EXISTS notes_fts USING fts5(
    title, body, path,
    content='notes',
    content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS notes_ai AFTER INSERT ON notes BEGIN
    INSERT INTO notes_fts(rowid, title, body, path) VALUES (new.rowid, new.title, new.body, new.path);
END;

CREATE TRIGGER IF NOT EXISTS notes_ad AFTER DELETE ON notes BEGIN
    INSERT INTO notes_fts(notes_fts, rowid, title, body, path) VALUES ('delete', old.rowid, old.title, old.body, old.path);
END;

CREATE TRIGGER IF NOT EXISTS notes_au AFTER UPDATE ON notes BEGIN
    INSERT INTO notes_fts(notes_fts, rowid, title, body, path) VALUES ('delete', old.rowid, old.title, old.body, old.path);
    INSERT INTO notes_fts(rowid, title, body, path) VALUES (new.rowid, new.title, new.body, new.path);
END;

CREATE TABLE IF NOT EXISTS tags (
    name    TEXT NOT NULL,
    path    TEXT NOT NULL REFERENCES notes(path) ON DELETE CASCADE,
    PRIMARY KEY (name, path)
);

CREATE TABLE IF NOT EXISTS links (
    source_path TEXT NOT NULL REFERENCES notes(path) ON DELETE CASCADE,
    target_path TEXT NOT NULL,
    title       TEXT,
    external    BOOLEAN NOT NULL DEFAULT 0,
    PRIMARY KEY (source_path, target_path)
);

CREATE TABLE IF NOT EXISTS index_meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS bookmarks (
    path    TEXT PRIMARY KEY REFERENCES notes(path) ON DELETE CASCADE,
    created DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS flashcards (
    card_hash       TEXT PRIMARY KEY,
    note_path       TEXT NOT NULL REFERENCES notes(path) ON DELETE CASCADE,
    kind            TEXT NOT NULL,
    question        TEXT NOT NULL,
    answer          TEXT NOT NULL,
    reversed        INTEGER NOT NULL DEFAULT 0,
    ord             INTEGER NOT NULL,
    first_seen      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS flashcards_by_note ON flashcards(note_path);

CREATE TABLE IF NOT EXISTS flashcard_state (
    card_hash       TEXT PRIMARY KEY REFERENCES flashcards(card_hash) ON DELETE CASCADE,
    due             DATETIME NOT NULL,
    stability       REAL NOT NULL,
    difficulty      REAL NOT NULL,
    elapsed_days    REAL NOT NULL,
    scheduled_days  REAL NOT NULL,
    reps            INTEGER NOT NULL,
    lapses          INTEGER NOT NULL,
    state           INTEGER NOT NULL,
    last_review     DATETIME
);
CREATE INDEX IF NOT EXISTS flashcard_state_due ON flashcard_state(due);

CREATE TABLE IF NOT EXISTS flashcard_reviews (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    card_hash       TEXT NOT NULL REFERENCES flashcards(card_hash) ON DELETE CASCADE,
    reviewed_at     DATETIME NOT NULL,
    rating          INTEGER NOT NULL,
    elapsed_days    REAL NOT NULL,
    scheduled_days  REAL NOT NULL,
    state_before    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS flashcard_reviews_by_card ON flashcard_reviews(card_hash);
CREATE INDEX IF NOT EXISTS flashcard_reviews_by_date ON flashcard_reviews(reviewed_at);
`

package index

const schemaSQL = `
CREATE TABLE IF NOT EXISTS notes (
    path        TEXT PRIMARY KEY,
    title       TEXT NOT NULL,
    body        TEXT NOT NULL,
    lead        TEXT,
    word_count  INTEGER NOT NULL,
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
`

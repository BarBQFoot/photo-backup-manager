-- SQLite schema for Photo Backup Manager.
-- Keep this file aligned with docs/02-DATABASE-SPEC.md.

CREATE TABLE IF NOT EXISTS backup_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL DEFAULT '',
    destination TEXT NOT NULL CHECK (trim(destination) <> ''),
    started_at DATETIME NOT NULL,
    completed_at DATETIME,
    total_files INTEGER NOT NULL DEFAULT 0 CHECK (total_files >= 0),
    success_count INTEGER NOT NULL DEFAULT 0 CHECK (success_count >= 0),
    failed_count INTEGER NOT NULL DEFAULT 0 CHECK (failed_count >= 0),
    skipped_count INTEGER NOT NULL DEFAULT 0 CHECK (skipped_count >= 0),
    duration_ms INTEGER NOT NULL DEFAULT 0 CHECK (duration_ms >= 0),
    status TEXT NOT NULL CHECK (status IN ('running', 'completed', 'partial', 'failed', 'interrupted'))
);

CREATE TABLE IF NOT EXISTS file_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_name TEXT NOT NULL CHECK (trim(file_name) <> ''),
    path TEXT NOT NULL CHECK (trim(path) <> ''),
    size_bytes INTEGER NOT NULL CHECK (size_bytes >= 0),
    mime_type TEXT,
    modified_at DATETIME,
    backup_job_id INTEGER NULL REFERENCES backup_jobs(id) ON DELETE SET NULL,
    description TEXT,
    ai_status TEXT NOT NULL DEFAULT 'unanalyzed'
        CHECK (ai_status IN ('unanalyzed', 'analyzing', 'analyzed', 'failed')),
    status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'missing', 'deleted'))
);

CREATE INDEX IF NOT EXISTS idx_file_records_path ON file_records(path);
CREATE INDEX IF NOT EXISTS idx_file_records_status ON file_records(status);
CREATE INDEX IF NOT EXISTS idx_backup_jobs_destination_started
    ON backup_jobs(destination, started_at);

CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO schema_migrations(version) VALUES (1);

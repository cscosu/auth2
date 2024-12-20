PRAGMA journal_mode=WAL;

BEGIN;

CREATE TABLE IF NOT EXISTS users (
    -- https://webauth.service.ohio-state.edu/~shibboleth/user-attribute-reference.html?article=idm-id
    -- Identity Management-assigned serial number, guarunteed to be unique and not change over time, unlike buck_id (employee id).
    idm_id TEXT PRIMARY KEY,
    discord_id INTEGER,
    buck_id INTEGER,
    name_num TEXT NOT NULL,
    display_name TEXT NOT NULL,
    last_seen_timestamp INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    last_attended_timestamp INTEGER,
    
    added_to_mailinglist INTEGER NOT NULL DEFAULT (FALSE),

    -- 0 or 1 depending on if the user has the affiliation
    student INTEGER NOT NULL,
    alum INTEGER NOT NULL,
    employee INTEGER NOT NULL,
    faculty INTEGER NOT NULL
) WITHOUT ROWID;

CREATE INDEX IF NOT EXISTS users_discord_id ON users (discord_id);
CREATE INDEX IF NOT EXISTS users_buck_id ON users (buck_id);
CREATE INDEX IF NOT EXISTS users_student ON users (student);

CREATE TABLE IF NOT EXISTS attendance_records (
    user_id INTEGER NOT NULL,
    timestamp INTEGER NOT NULL,
    -- 0 for in person, 1 for online
    kind INTEGER NOT NULL,
    PRIMARY KEY (user_id, timestamp),
    FOREIGN KEY (user_id) REFERENCES users(idm_id)
) WITHOUT ROWID;

COMMIT;

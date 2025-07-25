/*
   SNGCNOTIFIERbot - Social Network's Group Comments notifier bot
   Copyright (C) 2025  Unbewohnte (Kasyanov Nikolay Alexeevich)

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package db

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func NewDB(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path+"?_journal=WAL&_timeout=5000&_fk=true")
	if err != nil {
		return nil, err
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS monitored_groups (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		network TEXT NOT NULL,
		group_id TEXT NOT NULL,
		group_name TEXT NOT NULL,
		last_check INTEGER DEFAULT 0,
		last_notified INTEGER DEFAULT 0,
		extra_data TEXT DEFAULT '{}'
	);
	
    CREATE TABLE IF NOT EXISTS comments (
        id TEXT PRIMARY KEY,
        group_id INTEGER NOT NULL,
        network TEXT NOT NULL,
        comment_id TEXT NOT NULL,
        author TEXT NOT NULL,
        text TEXT NOT NULL,
        timestamp INTEGER NOT NULL,
        post_url TEXT NOT NULL,
        is_pending BOOLEAN DEFAULT FALSE,
        received_at INTEGER DEFAULT 0,
        FOREIGN KEY(group_id) REFERENCES monitored_groups(id) ON DELETE CASCADE
    );
    
    CREATE INDEX IF NOT EXISTS idx_comments_group ON comments(group_id);
    CREATE INDEX IF NOT EXISTS idx_comments_pending ON comments(is_pending);
`)
	if err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

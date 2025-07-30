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
	"time"
)

func (db *DB) AddGroup(group *MonitoredGroup) (int64, error) {
	result, err := db.Exec(`
		INSERT INTO monitored_groups (network, group_id, group_name, last_check)
		VALUES (?, ?, ?, ?)
	`, group.Network, group.GroupID, group.GroupName, group.LastCheck)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

func (db *DB) RemoveGroup(network, groupID string) error {
	_, err := db.Exec(`
		DELETE FROM monitored_groups
		WHERE network = ? AND group_id = ?
	`, network, groupID)
	return err
}

func (db *DB) GetGroups() ([]MonitoredGroup, error) {
	rows, err := db.Query(`
		SELECT id, created_at, network, group_id, group_name, last_check
		FROM monitored_groups
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []MonitoredGroup
	for rows.Next() {
		var group MonitoredGroup
		var createdAt string
		err := rows.Scan(
			&group.ID,
			&createdAt,
			&group.Network,
			&group.GroupID,
			&group.GroupName,
			&group.LastCheck,
		)
		if err != nil {
			return nil, err
		}

		group.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, err
		}

		groups = append(groups, group)
	}

	return groups, nil
}

func (db *DB) UpdateLastCheck(groupID int64, timestamp int64) error {
	_, err := db.Exec(`
		UPDATE monitored_groups
		SET last_check = ?
		WHERE id = ?
	`, timestamp, groupID)
	return err
}

func (db *DB) GetGroupsByNetwork(network string) ([]MonitoredGroup, error) {
	rows, err := db.Query(`
		SELECT id, created_at, network, group_id, group_name, last_check
		FROM monitored_groups
		WHERE network = ?
	`, network)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []MonitoredGroup
	for rows.Next() {
		var group MonitoredGroup
		var createdAt string
		err := rows.Scan(
			&group.ID,
			&createdAt,
			&group.Network,
			&group.GroupID,
			&group.GroupName,
			&group.LastCheck,
		)
		if err != nil {
			return nil, err
		}

		group.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, err
		}

		groups = append(groups, group)
	}

	return groups, nil
}

func (db *DB) GetGroupByNetworkAndID(network, groupID string) (*MonitoredGroup, error) {
	var group MonitoredGroup
	var createdAt string

	err := db.QueryRow(`
        SELECT id, created_at, network, group_id, group_name, last_check, extra_data
        FROM monitored_groups
        WHERE network = ? AND group_id = ?
    `, network, groupID).Scan(
		&group.ID,
		&createdAt,
		&group.Network,
		&group.GroupID,
		&group.GroupName,
		&group.LastCheck,
		&group.ExtraData,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	group.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, err
	}

	return &group, nil
}

func (db *DB) UpdateLastNotified(groupID int64, timestamp int64) error {
	_, err := db.Exec(`
        UPDATE monitored_groups
        SET last_notified = ?
        WHERE id = ?
    `, timestamp, groupID)
	return err
}

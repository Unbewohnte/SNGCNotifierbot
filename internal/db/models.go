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

import "time"

// Модель отслеживаемой группы
type MonitoredGroup struct {
	ID        int64     `db:"id"`
	CreatedAt time.Time `db:"created_at"`
	Network   string    `db:"network"`    // "vk", "ok", "tg"
	GroupID   string    `db:"group_id"`   // ID группы в соцсети
	GroupName string    `db:"group_name"` // Название группы
	LastCheck int64     `db:"last_check"` // Время последней проверки (unix timestamp)
	ExtraData string    `db:"extra_data"`
}

// Модель комментария
type Comment struct {
	ID        string `db:"id"`
	GroupID   int64  `db:"group_id"`   // Ссылка на группу
	Network   string `db:"network"`    // Соцсеть
	CommentID string `db:"comment_id"` // ID комментария в соцсети
	Author    string `db:"author"`
	Text      string `db:"text"`
	Timestamp int64  `db:"timestamp"` // Unix timestamp
	PostURL   string `db:"post_url"`
}

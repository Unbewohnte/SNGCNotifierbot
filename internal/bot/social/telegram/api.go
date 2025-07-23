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

package telegram

import (
	"Unbewohnte/SNGCNOTIFIERbot/internal/bot/social"
	"Unbewohnte/SNGCNOTIFIERbot/internal/db"
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Client struct {
	bot *tgbotapi.BotAPI
}

func NewClient(bot *tgbotapi.BotAPI) *Client {
	return &Client{
		bot: bot,
	}
}

// Для Telegram нам не нужны эти методы, но реализуем их для интерфейса
func (c *Client) GetGroupName(ctx context.Context, groupID string) (string, error) {
	// В Telegram мы получаем название группы из обновлений
	return groupID, nil
}

func (c *Client) GetComments(ctx context.Context, groupID string, lastCheck int64) ([]db.Comment, error) {
	// Для Telegram мы обрабатываем комментарии в реальном времени
	// Этот метод не используется
	return nil, nil
}

func (c *Client) GetGroupInfo(ctx context.Context, groupIdentifier string) (*social.GroupInfo, error) {
	// В Telegram мы получаем информацию о группе из обновлений
	return &social.GroupInfo{
		ID:         groupIdentifier,
		Name:       groupIdentifier,
		ScreenName: groupIdentifier,
	}, nil
}

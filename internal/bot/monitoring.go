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

package bot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"Unbewohnte/SNGCNOTIFIERbot/internal/db"

	"github.com/mymmrac/telego"
)

func (bot *Bot) StartMonitoring(intervalMins int) {
	go func(intervalMins int) {
		ticker := time.NewTicker(time.Duration(intervalMins) * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			groups, err := bot.conf.GetDB().GetGroups()
			if err != nil {
				log.Printf("Ошибка получения групп: %v", err)
				continue
			}

			for _, group := range groups {
				comments, err := bot.checkGroupComments(group)
				if err != nil {
					log.Printf("Ошибка проверки группы %s (%s): %v",
						group.GroupName, group.Network, err)
					continue
				}

				if len(comments) > 0 {
					if bot.isNotificationAllowed() {
						bot.notifyNewComments(group, comments)
					} else {
						bot.cacheComments(group, comments)
					}
				}

				// Всегда обновляем время последней проверки
				bot.conf.GetDB().UpdateLastCheck(group.ID, time.Now().Unix())
				time.Sleep(time.Second * 3)
			}

			// Проверяем кэш при каждой итерации
			bot.processPendingComments()
		}
	}(intervalMins)
}

func (bot *Bot) checkGroupComments(group db.MonitoredGroup) ([]db.Comment, error) {
	switch group.Network {
	case "vk":
		return bot.social.VKClient.GetComments(context.Background(), group.GroupID, group.LastCheck)
	case "ok":
		return bot.social.OKClient.GetComments(context.Background(), group.GroupID, group.LastCheck)
	default:
		return nil, nil
	}
}

// processCommentText обрабатывает текст комментария, заменяя специальные теги на понятные сообщения
func processCommentText(text string) string {
	if text == "" {
		return "((Пустой текст, возможно файл или стикер))"
	}

	// Проверяем наличие тегов вложения
	if strings.HasPrefix(text, "#ud") {
		// Пример: #ud6f8934c00#192:192s#
		return "((Сообщение содержит стикер или вложенный файл))"
	}

	if strings.Contains(text, "Сообщение содержит прикрепленные файлы") {
		return "((Сообщение содержит прикрепленные файлы))"
	}

	if strings.Contains(text, "media_url") {
		return "((Сообщение содержит медиа))"
	}

	return text
}

func (bot *Bot) notifyNewComments(group db.MonitoredGroup, comments []db.Comment) {
	for _, comment := range comments {
		processedText := processCommentText(comment.Text)

		status := "Только что"
		if comment.IsPending {
			status = fmt.Sprintf("Отправлено с задержкой: %s (комментарий получен в нерабочее время)",
				time.Unix(comment.Timestamp, 0).Format("2006-01-02 15:04"))
		}

		msgText := fmt.Sprintf(
			"💬 *Новый комментарий в %s (%s)*:\n\n"+
				"👤 *Автор*: %s\n"+
				"📝 *Текст*: %s\n"+
				"🔗 *Ссылка*: [Перейти к посту](%s)\n"+
				"⏰ *Время*: %s"+
				"📌 *Статус оповещения*: %s",
			group.GroupName,
			group.Network,
			comment.Author,
			processedText,
			comment.PostURL,
			time.Unix(comment.Timestamp, 0).Format("2006-01-02 15:04"),
			status,
		)

		params := &telego.SendMessageParams{
			ChatID:    telego.ChatID{ID: bot.conf.Telegram.MonitoringChannelID},
			Text:      msgText,
			ParseMode: "Markdown",
		}

		// Указываем ID топика, если он установлен
		if bot.conf.Telegram.MonitoringThreadID != 0 {
			params.MessageThreadID = int(bot.conf.Telegram.MonitoringThreadID)
		}

		if _, err := bot.api.SendMessage(context.Background(), params); err != nil {
			log.Printf("Ошибка отправки уведомления: %v", err)
		}
	}
}

func (bot *Bot) handleTelegramComment(msg *telego.Message) {
	// Пропускаем служебные сообщения и сообщения от самого бота
	if msg.From != nil && msg.From.ID == bot.api.ID() {
		return
	}

	// Формируем комментарий
	comment := db.Comment{
		ID:         fmt.Sprintf("%d", msg.MessageID),
		CommentID:  fmt.Sprintf("%d", msg.MessageID),
		Author:     formatUserName(msg.From),
		Text:       msg.Text,
		Timestamp:  int64(msg.Date),
		PostURL:    generateTelegramLink(msg),
		IsPending:  false,
		ReceivedAt: time.Now().Unix(),
	}

	group, err := bot.db.GetGroupByNetworkAndID("tg", fmt.Sprintf("%d", msg.Chat.ID))
	if err != nil {
		log.Printf("Failed to get tg group by ID: %s", err)
		return
	}

	// Обрабатываем комментарий
	if bot.isNotificationAllowed() {
		bot.notifyNewComments(*group, []db.Comment{comment})
	} else {
		comment.IsPending = true
		bot.cacheComments(*group, []db.Comment{comment})
	}
}

func (bot *Bot) cacheComments(group db.MonitoredGroup, comments []db.Comment) error {
	for i := range comments {
		_, err := bot.db.Exec(`
            INSERT OR REPLACE INTO comments 
            (id, group_id, network, comment_id, author, text, timestamp, post_url, is_pending, received_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			comments[i].ID,
			group.ID,
			group.Network,
			comments[i].CommentID,
			comments[i].Author,
			comments[i].Text,
			comments[i].Timestamp,
			comments[i].PostURL,
			true,              // is_pending = true
			time.Now().Unix(), // received_at
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (bot *Bot) processPendingComments() error {
	if !bot.isNotificationAllowed() {
		return nil
	}

	// Получаем все отложенные комментарии
	rows, err := bot.db.Query(`
        SELECT * FROM comments 
        WHERE is_pending = TRUE 
        AND received_at > ?`,
		time.Now().Add(-7*24*time.Hour).Unix(),
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	var comments []db.Comment
	for rows.Next() {
		var c db.Comment
		err := rows.Scan(
			&c.ID, &c.GroupID, &c.Network, &c.CommentID,
			&c.Author, &c.Text, &c.Timestamp, &c.PostURL,
			&c.IsPending, &c.ReceivedAt,
		)
		if err != nil {
			return err
		}
		comments = append(comments, c)
	}

	// Отправляем и помечаем как отправленные
	for _, c := range comments {
		group, err := bot.db.GetGroupByNetworkAndID(c.Network, fmt.Sprintf("%d", c.GroupID))
		if err != nil {
			continue
		}

		bot.notifyNewComments(*group, []db.Comment{c})

		// Обновляем статус в БД
		_, err = bot.db.Exec(`
            UPDATE comments SET is_pending = FALSE 
            WHERE id = ?`, c.ID)
		if err != nil {
			log.Printf("Ошибка обновления комментария: %v", err)
		}
	}

	return nil
}

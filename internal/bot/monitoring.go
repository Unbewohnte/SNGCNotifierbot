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

func (bot *Bot) StartMonitoring() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute) // Проверяем каждые N минут
		defer ticker.Stop()

		for range ticker.C {
			groups, err := bot.conf.GetDB().GetGroups()
			if err != nil {
				log.Printf("Ошибка получения групп: %v", err)
				continue
			}

			for _, group := range groups {
				if bot.conf.Debug {
					log.Printf("Смотрим группу %+v...", group)
				}

				comments, err := bot.checkGroupComments(group)
				if err != nil {
					log.Printf("Ошибка проверки группы %s (%s): %v",
						group.GroupName, group.Network, err)
					continue
				}

				if bot.conf.Debug {
					log.Printf("Комментарии: %+v", comments)
				}

				if len(comments) > 0 {
					bot.notifyNewComments(group, comments)
					bot.conf.GetDB().UpdateLastCheck(group.ID, time.Now().Unix())
				}

				// Даем время
				time.Sleep(time.Second * 3)
			}
		}
	}()
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

		msgText := fmt.Sprintf(
			"💬 *Новый комментарий в %s (%s)*:\n\n"+
				"👤 *Автор*: %s\n"+
				"📝 *Текст*: %s\n"+
				"🔗 *Ссылка*: [Перейти к посту](%s)\n"+
				"⏰ *Время*: %s",
			group.GroupName,
			group.Network,
			comment.Author,
			processedText,
			comment.PostURL,
			time.Unix(comment.Timestamp, 0).Format("2006-01-02 15:04"),
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

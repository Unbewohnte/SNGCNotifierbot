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
	"strconv"
	"strings"
	"time"

	"Unbewohnte/SNGCNOTIFIERbot/internal/db"

	"github.com/mymmrac/telego"
)

func (bot *Bot) StartMonitoring(intervalMins int) {
	log.Printf("Запускаем мониторинг с интервалом %d минут", intervalMins)

	go func(intervalMins int) {
		ticker := time.NewTicker(time.Duration(intervalMins) * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			groups, err := bot.conf.GetDB().GetGroups()
			if err != nil {
				log.Printf("Ошибка получения групп: %v", err)
				continue
			}

			// Проверяем кэш при каждой итерации
			err = bot.processPendingComments()
			if err != nil {
				log.Printf("Ошибка обработки кэшированных сообщений: %s", err)
			}

			log.Printf("Проверка %d групп...", len(groups))

			for _, group := range groups {
				if group.Network == "tg" {
					continue
				}

				time.Sleep(time.Second * 5)

				comments, err := bot.checkGroupComments(group)
				if err != nil {
					log.Printf("Ошибка проверки группы %s (%s): %v. Дополнительно ждем...",
						group.GroupName, group.Network, err,
					)

					time.Sleep(time.Second * 15)
					comments, err = bot.checkGroupComments(group)
					if err != nil {
						log.Printf("Ошибка дополнительной проверки %s: %s. Комментарии не проверены.", group.GroupName, err)
						continue
					}
				}

				if len(comments) > 0 {
					log.Printf("Найдено %d новых комментариев в %s (%s)",
						len(comments),
						group.GroupName,
						group.Network,
					)

					if bot.isNotificationAllowed() {
						log.Printf("Оповещения разрешены, оповещение...")
						bot.notifyNewComments(group, comments)
					} else {
						log.Printf("Оповещения запрещены, добавление в кэш...")
						err = bot.cacheComments(group, comments)
						if err != nil {
							log.Printf("Ошибка добавления новых комментариев в кэш: %s. Не обновляем время последней проверки.", err)
							continue
						}
					}
				}

				// Обновляем время последней проверки
				bot.conf.GetDB().UpdateLastCheck(group.ID, time.Now().Unix())
			}
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

func escapeMarkdown(text string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"`", "\\`",
		"[", "\\[",
	)

	return replacer.Replace(text)
}

func formatTimeAgo(timestamp int64) string {
	ago := time.Since(time.Unix(timestamp, 0))

	switch {
	case ago.Seconds() < 10:
		return "только что"
	case ago.Minutes() < 1:
		return fmt.Sprintf("%d сек назад", int(ago.Seconds()))
	case ago.Hours() < 1:
		return fmt.Sprintf("%d мин назад", int(ago.Minutes()))
	case ago.Hours() < 24:
		return fmt.Sprintf("%d ч назад", int(ago.Hours()))
	default:
		return "давно"
	}
}

func (bot *Bot) constructNotificationMessage(group db.MonitoredGroup, comment db.Comment) string {
	safeAuthor := escapeMarkdown(comment.Author)
	safeGroupName := escapeMarkdown(group.GroupName)
	if len([]rune(comment.Text)) > 500 {
		comment.Text = string([]rune(comment.Text)[:500]) + "\n\n⚠️ Сообщение было обрезано."
	}
	safeText := escapeMarkdown(processCommentText(comment.Text))

	status := "Только что"
	if comment.IsPending {
		status = "Отправлено с задержкой: (комментарий получен в нерабочее время)"
	}

	// Форматируем время в человекочитаемый вид
	commentTime := time.Unix(comment.Timestamp, 0)
	var timeStr string
	if commentTime.Day() == time.Now().Day() && commentTime.Month() == time.Now().Month() && commentTime.Year() == time.Now().Year() {
		timeStr = commentTime.Format("сегодня в 15:04")
	} else {
		timeStr = commentTime.Format("02.01.2006 в 15:04")
	}

	var msgText string
	switch bot.conf.NotificationMessageType {
	case NOTIFICATION_FULL:
		msgText = fmt.Sprintf(
			"💬 *Новый комментарий в \"%s\" (%s)*:\n\n"+
				"📝 *Текст*: %s\n\n"+
				"👤 *Автор*: %s\n"+
				"🔗 *Ссылка*: [Перейти к посту](%s)\n"+
				"⏰ *Время публикации комментария*: %s\n"+
				"📌 *Статус оповещения*: %s",
			safeGroupName,
			group.Network,
			safeText,
			safeAuthor,
			comment.PostURL,
			timeStr,
			status,
		)
	case NOTIFICATION_MINIMALISTIC:
		ago := formatTimeAgo(comment.Timestamp)

		// Обрезаем текст комментария для минималистичного вида
		msgText = fmt.Sprintf(
			"🌐 (%s) *%s*\n"+
				"💬 %s\n"+
				"⏰ %s | (статус: %s)\n"+
				"👤 *%s*\n"+
				"🔗 [Перейти к посту](%s) • %s",
			group.Network,
			group.GroupName,
			safeText,
			timeStr,
			status,
			safeAuthor,
			comment.PostURL,
			ago,
		)

	case NOTIFICATION_SPACED:
		// Добавляем визуальные разделители и отступы
		divider := strings.Repeat("•", 35) + "\n"

		msgText = fmt.Sprintf(
			"*💬 НОВЫЙ КОММЕНТАРИЙ*\n"+
				"*Группа:* _%s_ (%s)\n"+
				divider+
				"*📝 Текст комментария:*\n%s\n"+
				divider+
				"*👤 Автор:* %s\n"+
				"*⏰ Время:* %s\n"+
				"*📌 Статус:* %s\n"+
				"*🔗 Ссылка:* [Перейти к посту](%s)",
			safeGroupName,
			group.Network,
			safeText,
			safeAuthor,
			timeStr,
			status,
			comment.PostURL,
		)

	default:
		msgText = fmt.Sprintf(
			"💬 *Новый комментарий в \"%s\" (%s)*:\n\n"+
				"👤 *Автор*: %s\n"+
				"📝 *Текст*: %s\n"+
				"🔗 *Ссылка*: [Перейти к посту](%s)\n"+
				"⏰ *Время публикации комментария*: %s\n"+
				"📌 *Статус оповещения*: %s",
			safeGroupName,
			group.Network,
			safeAuthor,
			safeText,
			comment.PostURL,
			time.Unix(comment.Timestamp, 0).Format("2006-01-02 15:04"),
			status,
		)
	}

	return msgText
}

func (bot *Bot) notifyNewComments(group db.MonitoredGroup, comments []db.Comment) {
	for _, comment := range comments {
		msgText := bot.constructNotificationMessage(group, comment)

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
	if msg.From == nil {
		return
	}

	if msg.From.ID == bot.api.ID() {
		return
	}

	// Пропускаем сообщения от "группы" (пользователя Telegram)
	if msg.From.ID == 777000 { // 777000 — официальный ID Telegram-групп
		return
	}

	// Формируем комментарий
	if msg.Text == "" && msg.Caption != "" {
		msg.Text = msg.Caption
	}

	comment := db.Comment{
		ID:         fmt.Sprintf("tg-%d", msg.MessageID),
		CommentID:  fmt.Sprintf("%d", msg.MessageID),
		Author:     formatUserName(msg.From),
		Text:       msg.Text,
		Timestamp:  int64(msg.Date),
		PostURL:    bot.generateTelegramLink(msg),
		IsPending:  false,
		ReceivedAt: time.Now().Unix(),
	}

	group, err := bot.db.GetGroupByNetworkAndID("tg", strconv.FormatInt(msg.Chat.ID, 10))
	if err != nil {
		log.Printf("Failed to get tg group by ID: %s", err)
		return
	}

	log.Printf("Новый комментарий в телеграм от %d в %s (%s).",
		msg.From.ID,
		group.GroupName,
		group.Network,
	)

	// Обрабатываем комментарий
	if bot.isNotificationAllowed() {
		log.Printf("Оповещение о телеграм комментарии...")
		bot.notifyNewComments(*group, []db.Comment{comment})
	} else {
		log.Printf("Добавление телеграм комментария в кэш...")
		comment.IsPending = true
		err = bot.cacheComments(*group, []db.Comment{comment})
		if err != nil {
			log.Printf("Не удалось сохранить телеграм комментарий в кэш: %s. Потеря комментария.", err)
		}
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
		group, err := bot.db.GetGroupByInternalID(fmt.Sprintf("%d", c.GroupID))
		if err != nil {
			continue
		}

		if group == nil {
			log.Printf("Группа ID %d не найдена, удаляю комментарии", c.GroupID)
			bot.db.Exec(`DELETE FROM comments WHERE group_id = ?`, c.GroupID)
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

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
	"fmt"
	"sort"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Левенштейн
func minDistance(a, b string) int {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
		dp[i][0] = i
	}
	for j := range dp[0] {
		dp[0][j] = j
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1]
			} else {
				dp[i][j] = 1 + min(dp[i-1][j], dp[i][j-1], dp[i-1][j-1])
			}
		}
	}
	return dp[m][n]
}

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

func (bot *Bot) findSimilarCommands(input string) []string {
	type cmdDistance struct {
		name     string
		distance int
	}

	var distances []cmdDistance
	for _, cmd := range bot.commands {
		dist := minDistance(input, cmd.Name)
		distances = append(distances, cmdDistance{cmd.Name, dist})
	}

	sort.Slice(distances, func(i, j int) bool {
		return distances[i].distance < distances[j].distance
	})

	var suggestions []string
	for i := 0; i < 3 && i < len(distances); i++ {
		suggestions = append(suggestions, distances[i].name)
	}

	return suggestions
}

func (bot *Bot) sendError(chatID int64, text string, replyTo int) {
	msg := tgbotapi.NewMessage(chatID, "❌ "+text)
	msg.ReplyToMessageID = replyTo
	bot.api.Send(msg)
}

func (bot *Bot) sendSuccess(chatID int64, text string, replyTo int) {
	msg := tgbotapi.NewMessage(chatID, "✅ "+text)
	msg.ReplyToMessageID = replyTo
	bot.api.Send(msg)
}

func (bot *Bot) sendCommandSuggestions(chatID int64, input string) {
	suggestions := bot.findSimilarCommands(input)
	if len(suggestions) == 0 {
		return
	}

	message := "Неизвестная команда. Возможно, имеется в виду одна из этих команд:\n"
	for _, cmd := range suggestions {
		command := bot.CommandByName(cmd)
		if command != nil {
			message += fmt.Sprintf("`/%s` - %s\n", command.Name, command.Description)
		}
	}
	message += "\nДля справки используйте `help [команда](опционально)`"

	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = "Markdown"
	bot.api.Send(msg)
}

// Проверяем, является ли чат мониторируемой группой Telegram
func (bot *Bot) isMonitoredTelegramGroup(chatID int64) bool {
	groups, err := bot.conf.GetDB().GetGroupsByNetwork("tg")
	if err != nil {
		return false
	}

	for _, group := range groups {
		groupID, err := strconv.ParseInt(group.GroupID, 10, 64)
		if err != nil {
			continue
		}
		if groupID == chatID {
			return true
		}
	}
	return false
}

// Обработчик комментариев в Telegram группах
func (bot *Bot) handleTelegramComment(message *tgbotapi.Message) {
	// Пропускаем служебные сообщения и сообщения от самого бота
	if message.From.ID == bot.api.Self.ID {
		return
	}

	// Формируем информацию о комментарии
	authorName := message.From.FirstName
	if message.From.LastName != "" {
		authorName += " " + message.From.LastName
	}

	// Формируем ссылку на сообщение
	var link string
	if message.Chat.UserName != "" {
		link = fmt.Sprintf("https://t.me/%s/%d", message.Chat.UserName, message.MessageID)
	} else {
		link = fmt.Sprintf("chat_id: %d, message_id: %d", message.Chat.ID, message.MessageID)
	}

	// Создаем уведомление
	msgText := fmt.Sprintf(
		"💬 *Новый комментарий в %s (Telegram)*:\n\n"+
			"👤 *Автор*: %s\n"+
			"📝 *Текст*: %s\n"+
			"🔗 *Ссылка*: [Перейти к комментарию](%s)\n"+
			"⏰ *Время*: %s",
		message.Chat.Title,
		authorName,
		message.Text,
		link,
		time.Unix(int64(message.Date), 0).Format("2006-01-02 15:04"),
	)

	// Отправляем уведомление в мониторинговый канал
	msg := tgbotapi.NewMessage(bot.conf.Telegram.MonitoringChannelID, msgText)
	msg.ParseMode = "Markdown"
	msg.DisableWebPagePreview = true
	bot.api.Send(msg)
}

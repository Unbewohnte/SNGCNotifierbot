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
	"sort"
	"strconv"
	"time"

	"github.com/mymmrac/telego"
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

func (bot *Bot) sendMessage(chatID int64, text string) {
	params := &telego.SendMessageParams{
		ChatID: telego.ChatID{
			ID: chatID,
		},
		Text:      text,
		ParseMode: "Markdown",
	}

	bot.api.SendMessage(context.Background(), params)
}

func (bot *Bot) answerBack(message *telego.Message, text string, reply bool) {
	params := &telego.SendMessageParams{
		ChatID: telego.ChatID{
			ID: message.Chat.ID,
		},
		Text:      text,
		ParseMode: "Markdown",
	}

	if message.MessageThreadID != 0 {
		params.MessageThreadID = message.MessageThreadID
	}

	if reply {
		params.ReplyParameters = &telego.ReplyParameters{
			MessageID: message.MessageID,
		}
	}

	bot.api.SendMessage(context.Background(), params)
}

func (bot *Bot) sendError(message *telego.Message, text string) {
	bot.answerBack(message, "❌ "+text, true)
}

func (bot *Bot) sendSuccess(message *telego.Message, text string) {
	bot.answerBack(message, "✅ "+text, true)
}

func (bot *Bot) sendCommandSuggestions(msg *telego.Message, input string) {
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

	bot.answerBack(msg, message, true)
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

func (bot *Bot) handleTelegramComment(msg *telego.Message) {
	// Пропускаем служебные сообщения и сообщения от самого бота
	if msg.From != nil && msg.From.ID == bot.api.ID() {
		return
	}

	// Формируем информацию о комментарии
	authorName := msg.From.FirstName
	if msg.From.LastName != "" {
		authorName += " " + msg.From.LastName
	}

	// Формируем ссылку на сообщение
	var link string
	if msg.Chat.Username != "" {
		link = fmt.Sprintf("https://t.me/%s/%d", msg.Chat.Username, msg.MessageID)
	} else {
		link = fmt.Sprintf("chat_id: %d, message_id: %d", msg.Chat.ID, msg.MessageID)
	}

	// Создаем уведомление
	msgText := fmt.Sprintf(
		"💬 *Новый комментарий в %s (Telegram)*:\n"+
			"👤 *Автор*: %s\n"+
			"📝 *Текст*: %s\n"+
			"🔗 *Ссылка*: [Перейти к комментарию](%s)\n"+
			"⏰ *Время*: %s",
		msg.Chat.Title,
		authorName,
		msg.Text,
		link,
		time.Unix(int64(msg.Date), 0).Format("2006-01-02 15:04"),
	)

	// Отправляем уведомление в мониторинговый канал
	params := &telego.SendMessageParams{
		ChatID:    telego.ChatID{ID: bot.conf.Telegram.MonitoringChannelID},
		Text:      msgText,
		ParseMode: "Markdown",
	}

	if bot.conf.Telegram.MonitoringThreadID != 0 {
		params.MessageThreadID = int(bot.conf.Telegram.MonitoringThreadID)
	}

	bot.api.SendMessage(context.Background(), params)
}

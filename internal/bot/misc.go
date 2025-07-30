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
	"sort"
	"strconv"
	"strings"
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

func (bot *Bot) sendMessage(chatID int64, threadID int, text string) {
	params := &telego.SendMessageParams{
		ChatID: telego.ChatID{
			ID: chatID,
		},
		Text:      text,
		ParseMode: "Markdown",
	}

	if threadID != 0 {
		params.MessageThreadID = threadID
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

	log.Printf("Проверяем chatID %d в мониторируемых группах: %+v", chatID, groups)

	// Преобразуем chatID в строку
	chatIDStr := strconv.FormatInt(chatID, 10)

	for _, group := range groups {
		log.Printf("Сравниваем: GroupID='%s' с chatID='%d'", group.GroupID, chatID)

		if group.GroupID == chatIDStr {
			return true
		}
	}

	return false
}

// Проверяет, разрешено ли сейчас отправлять уведомления
func (bot *Bot) isNotificationAllowed() bool {
	schedule := bot.conf.Schedule

	// Если расписание отключено - разрешаем всегда
	if !schedule.Enabled {
		return true
	}

	// Определяем текущее время в нужном часовом поясе
	loc, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		log.Printf("Ошибка загрузки часового пояса: %v", err)
		return true // Разрешаем по умолчанию
	}

	now := time.Now().In(loc)
	currentDay := strings.ToLower(now.Weekday().String()[:3])
	currentTime := now.Format("15:04")

	// Проверяем день недели
	dayAllowed := false
	for _, day := range schedule.DaysOfWeek {
		if strings.ToLower(day) == currentDay {
			dayAllowed = true
			break
		}
	}

	if !dayAllowed {
		return false
	}

	// Проверяем временной интервал
	return currentTime >= schedule.StartTime && currentTime <= schedule.EndTime
}

// Форматирует имя пользователя Telegram
func formatUserName(user *telego.User) string {
	if user == nil {
		return "Неизвестный пользователь"
	}
	name := user.FirstName
	if user.LastName != "" {
		name += " " + user.LastName
	}
	if user.Username != "" {
		name += " (@" + user.Username + ")"
	}
	return name
}

// Генерирует ссылку на сообщение в Telegram
func generateTelegramLink(msg *telego.Message) string {
	if msg.Chat.Username != "" {
		return fmt.Sprintf("https://t.me/%s/%d", msg.Chat.Username, msg.MessageID)
	}

	// Для чатов без username используем формат с ID
	return fmt.Sprintf("https://t.me/c/%d/%d", msg.Chat.ID, msg.MessageID)
}

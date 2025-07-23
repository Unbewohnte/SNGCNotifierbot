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
	"Unbewohnte/SNGCNOTIFIERbot/internal/db"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Command struct {
	Name        string
	Description string
	Example     string
	Group       string
	Call        func(*tgbotapi.Message)
}

func (bot *Bot) NewCommand(cmd Command) {
	bot.commands = append(bot.commands, cmd)
}

func (bot *Bot) CommandByName(name string) *Command {
	for i := range bot.commands {
		if bot.commands[i].Name == name {
			return &bot.commands[i]
		}
	}

	return nil
}

func constructCommandHelpMessage(command Command) string {
	commandHelp := ""
	commandHelp += fmt.Sprintf("\n*Команда:* \"/%s\"\n*Описание:* %s\n", command.Name, command.Description)
	if command.Example != "" {
		commandHelp += fmt.Sprintf("*Пример:* `%s`\n", command.Example)
	}

	return commandHelp
}

func (bot *Bot) Help(message *tgbotapi.Message) {
	parts := strings.Split(message.Text, " ")
	if len(parts) >= 2 {
		// Ответить лишь по конкретной команде
		command := bot.CommandByName(parts[1])
		if command != nil {
			helpMessage := constructCommandHelpMessage(*command)
			msg := tgbotapi.NewMessage(
				message.Chat.ID,
				helpMessage,
			)
			msg.ParseMode = "Markdown"
			bot.api.Send(msg)
			return
		}
	}

	var helpMessage string

	commandsByGroup := make(map[string][]Command)
	for _, command := range bot.commands {
		commandsByGroup[command.Group] = append(commandsByGroup[command.Group], command)
	}

	groups := []string{}
	for g := range commandsByGroup {
		groups = append(groups, g)
	}
	sort.Strings(groups)

	for _, group := range groups {
		helpMessage += fmt.Sprintf("\n\n*[%s]*\n", group)
		for _, command := range commandsByGroup[group] {
			helpMessage += constructCommandHelpMessage(command)
		}
	}

	msg := tgbotapi.NewMessage(
		message.Chat.ID,
		helpMessage,
	)
	msg.ParseMode = "Markdown"
	bot.api.Send(msg)
}

func (bot *Bot) About(message *tgbotapi.Message) {
	msg := tgbotapi.NewMessage(
		message.Chat.ID,
		`SNGCNOTIFIER bot - Телеграм бот для оповещения о новых комментариях под постами групп в ВКонтакте, Одноклассники и Телеграм.

Source: https://github.com/Unbewohnte/SNGCNotifierbot
Лицензия: GPLv3`,
	)

	bot.api.Send(msg)
}

func (bot *Bot) AddUser(message *tgbotapi.Message) {
	parts := strings.Split(strings.TrimSpace(message.Text), " ")
	if len(parts) < 2 {
		msg := tgbotapi.NewMessage(
			message.Chat.ID,
			"ID пользователя не указан",
		)
		msg.ReplyToMessageID = message.MessageID
		bot.api.Send(msg)
		return
	}

	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		msg := tgbotapi.NewMessage(
			message.Chat.ID,
			"Неверный ID пользователя",
		)
		msg.ReplyToMessageID = message.MessageID
		bot.api.Send(msg)
		return
	}

	for _, allowedID := range bot.conf.Telegram.AllowedUserIDs {
		if id == allowedID {
			msg := tgbotapi.NewMessage(
				message.Chat.ID,
				"Этот пользователь уже есть в списке разрешенных.",
			)
			msg.ReplyToMessageID = message.MessageID
			bot.api.Send(msg)
			return
		}
	}

	bot.conf.Telegram.AllowedUserIDs = append(bot.conf.Telegram.AllowedUserIDs, id)

	// Сохраним в файл
	bot.conf.Update()

	msg := tgbotapi.NewMessage(
		message.Chat.ID,
		"Пользователь успешно добавлен!",
	)
	msg.ReplyToMessageID = message.MessageID
	bot.api.Send(msg)
}

func (bot *Bot) TogglePublicity(message *tgbotapi.Message) {
	if bot.conf.Telegram.Public {
		bot.conf.Telegram.Public = false
		bot.api.Send(
			tgbotapi.NewMessage(message.Chat.ID, "Доступ к боту теперь только у избранных."),
		)
	} else {
		bot.conf.Telegram.Public = true
		bot.api.Send(
			tgbotapi.NewMessage(message.Chat.ID, "Доступ к боту теперь у всех."),
		)
	}

	// Обновляем конфигурационный файл
	bot.conf.Update()
}

func (bot *Bot) RemoveUser(message *tgbotapi.Message) {
	parts := strings.Split(strings.TrimSpace(message.Text), " ")
	if len(parts) < 2 {
		msg := tgbotapi.NewMessage(
			message.Chat.ID,
			"ID пользователя не указан",
		)
		msg.ReplyToMessageID = message.MessageID
		bot.api.Send(msg)
		return
	}

	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		msg := tgbotapi.NewMessage(
			message.Chat.ID,
			"Неверный ID пользователя",
		)
		msg.ReplyToMessageID = message.MessageID
		bot.api.Send(msg)
		return
	}

	tmp := bot.conf.Telegram.AllowedUserIDs
	bot.conf.Telegram.AllowedUserIDs = []int64{}
	for _, allowedID := range tmp {
		if allowedID == id {
			continue
		}

		bot.conf.Telegram.AllowedUserIDs = append(bot.conf.Telegram.AllowedUserIDs, allowedID)
	}

	// Сохраним в файл
	bot.conf.Update()

	msg := tgbotapi.NewMessage(
		message.Chat.ID,
		"Пользователь успешно удален!",
	)
	msg.ReplyToMessageID = message.MessageID
	bot.api.Send(msg)
}

func (bot *Bot) PrintConfig(message *tgbotapi.Message) {
	var response string = ""

	response += "*Нынешняя конфигурация*: \n"
	response += "\n*[ОБЩЕЕ]*:\n"
	response += fmt.Sprintf("*Общедоступный?*: `%v`\n", bot.conf.Telegram.Public)
	response += fmt.Sprintf("*Разрешенные пользователи*: `%+v`\n", bot.conf.Telegram.AllowedUserIDs)
	response += fmt.Sprintf("*ID мониторинговый чат*: `%+v`\n", bot.conf.Telegram.MonitoringChannelID)

	response += "\n*[СОЦИАЛЬНЫЕ СЕТИ]*:\n"
	if bot.conf.Social.OK.Token != "" {
		response += "*OK*: Токен имеется\n"
	} else {
		response += "*OK*: Токен отсутствует\n"
	}
	if bot.conf.Social.VK.Token != "" {
		response += "*VK*: Токен имеется\n"
	} else {
		response += "*VK*: Токен отсутствует\n"
	}
	if bot.conf.Social.Telegram.Token != "" {
		response += "*TG*: Токен имеется\n"
	} else {
		response += "*TG*: Токен отсутствует\n"
	}

	msg := tgbotapi.NewMessage(
		message.Chat.ID,
		response,
	)
	msg.ParseMode = "Markdown"
	msg.ReplyToMessageID = message.MessageID
	bot.api.Send(msg)
}

// Вспомогательная функция для нормализации ID группы ВК
func normalizeVKGroupID(input string) (string, error) {
	input = strings.TrimSpace(input)

	// Извлекаем последнюю часть из URL (если это ссылка)
	if strings.HasPrefix(input, "https://") ||
		strings.HasPrefix(input, "http://") ||
		strings.HasPrefix(input, "vk.com") {

		// Разбиваем URL на части
		parts := strings.Split(input, "/")
		lastPart := parts[len(parts)-1]

		// Удаляем параметры запроса (если есть)
		lastPart = strings.Split(lastPart, "?")[0]
		input = lastPart
	}

	// Удаляем префиксы "club" и "public"
	input = strings.TrimPrefix(input, "club")
	input = strings.TrimPrefix(input, "public")

	// Проверяем, содержит ли только допустимые символы
	validChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_."
	for _, char := range input {
		if !strings.ContainsRune(validChars, char) {
			return "", fmt.Errorf("недопустимые символы в идентификаторе группы")
		}
	}

	if input == "" {
		return "", fmt.Errorf("не удалось извлечь идентификатор группы")
	}

	return input, nil
}

func (bot *Bot) AddGroup(message *tgbotapi.Message) {
	parts := strings.Split(strings.TrimSpace(message.Text), " ")
	if len(parts) < 3 {
		bot.sendError(message.Chat.ID, "Неверный формат. Используйте: /addgroup <сеть> <ID группы>", message.MessageID)
		return
	}

	network := strings.ToLower(parts[1])
	groupID := strings.Join(parts[2:], " ") // Объединяем оставшиеся части на случай пробелов в ID

	// Сначала проверяем, существует ли уже такая группа
	existingGroup, err := bot.conf.GetDB().GetGroupByNetworkAndID(network, groupID)
	if err == nil && existingGroup != nil {
		bot.sendError(message.Chat.ID,
			fmt.Sprintf("Эта группа уже добавлена:\nНазвание: %s\nID: %s\nДобавлена: %s",
				existingGroup.GroupName,
				existingGroup.GroupID,
				existingGroup.CreatedAt.Local().Format("2006-01-02 15:04")),
			message.MessageID)
		return
	}

	var group db.MonitoredGroup
	switch network {
	case "vk":
		// Нормализуем идентификатор группы ВК
		normalizedID, err := normalizeVKGroupID(groupID)
		if err != nil {
			bot.sendError(message.Chat.ID, "Неверный ID группы ВК: "+err.Error(), message.MessageID)
			return
		}

		// Получаем информацию о группе
		info, err := bot.social.VKClient.GetGroupInfo(context.Background(), normalizedID)
		if err != nil {
			bot.sendError(message.Chat.ID, fmt.Sprintf("Ошибка проверки группы: %v", err), message.MessageID)
			return
		}

		// Преобразуем ExtraData в JSON
		extraData := map[string]string{
			"screen_name": info.ScreenName,
		}
		extraDataJSON, err := json.Marshal(extraData)
		if err != nil {
			bot.sendError(message.Chat.ID, "Ошибка формирования данных группы: "+err.Error(), message.MessageID)
			return
		}

		group = db.MonitoredGroup{
			Network:   network,
			GroupID:   info.ID, // Сохраняем числовой ID
			GroupName: info.Name,
			LastCheck: time.Now().Unix(),
			ExtraData: string(extraDataJSON),
		}
	case "ok":
		// Извлекаем короткое имя из URL, если это ссылка
		if strings.HasPrefix(groupID, "http") || strings.HasPrefix(groupID, "ok.ru") {
			groupID = extractOKGroupID(groupID)
		}

		// Получаем информацию о группе
		info, err := bot.social.OKClient.GetGroupInfo(context.Background(), groupID)
		if err != nil {
			bot.sendError(message.Chat.ID, fmt.Sprintf("Ошибка проверки группы: %v", err), message.MessageID)
			return
		}

		// Преобразуем ExtraData в JSON
		extraData := map[string]string{
			"screen_name": info.ScreenName,
		}
		extraDataJSON, err := json.Marshal(extraData)
		if err != nil {
			bot.sendError(message.Chat.ID, "Ошибка формирования данных группы: "+err.Error(), message.MessageID)
			return
		}

		group = db.MonitoredGroup{
			Network:   network,
			GroupID:   info.ID, // Используем числовой ID
			GroupName: info.Name,
			LastCheck: time.Now().Unix(),
			ExtraData: string(extraDataJSON),
		}
	case "tg":
		// Для Telegram используем ID чата из текущего сообщения
		if message.Chat.ID == 0 {
			bot.sendError(message.Chat.ID, "Не удалось определить ID группы", message.MessageID)
			return
		}

		group = db.MonitoredGroup{
			Network:   network,
			GroupID:   strconv.FormatInt(message.Chat.ID, 10),
			GroupName: message.Chat.Title,
			LastCheck: time.Now().Unix(),
			ExtraData: "{}",
		}
	default:
		bot.sendError(message.Chat.ID, "Неподдерживаемая социальная сеть", message.MessageID)
		return
	}

	id, err := bot.conf.GetDB().AddGroup(&group)
	if err != nil {
		bot.sendError(message.Chat.ID, "Ошибка добавления группы: "+err.Error(), message.MessageID)
		return
	}

	bot.sendSuccess(message.Chat.ID, fmt.Sprintf(
		"Группа добавлена:\nНазвание: %s\nID: %s\nID в базе: %d",
		group.GroupName, group.GroupID, id,
	), message.MessageID)
}

// Извлекаем ID группы из URL Одноклассников
func extractOKGroupID(url string) string {
	// Примеры URL:
	// https://ok.ru/group/apiok
	// https://ok.ru/group/123456789012
	parts := strings.Split(url, "/")
	for i, part := range parts {
		if part == "group" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return url
}

func (bot *Bot) RemoveGroup(message *tgbotapi.Message) {
	parts := strings.Split(strings.TrimSpace(message.Text), " ")
	if len(parts) < 3 {
		bot.sendError(message.Chat.ID, "Неверный формат. Используйте: /rmgroup <сеть> <ID группы>", message.MessageID)
		return
	}

	network := strings.ToLower(parts[1])
	groupID := parts[2]

	// Сначала проверяем, существует ли такая группа
	existingGroup, err := bot.conf.GetDB().GetGroupByNetworkAndID(network, groupID)
	if err != nil || existingGroup == nil {
		bot.sendError(message.Chat.ID,
			fmt.Sprintf("Группа с ID %s не найдена в %s", groupID, network),
			message.MessageID)
		return
	}

	err = bot.conf.GetDB().RemoveGroup(network, groupID)
	if err != nil {
		bot.sendError(message.Chat.ID, "Ошибка удаления группы: "+err.Error(), message.MessageID)
		return
	}

	bot.sendSuccess(message.Chat.ID, "Группа успешно удалена", message.MessageID)
}

func (bot *Bot) ListGroups(message *tgbotapi.Message) {
	groups, err := bot.conf.GetDB().GetGroups()
	if err != nil {
		bot.sendError(message.Chat.ID, "Ошибка получения групп: "+err.Error(), message.MessageID)
		return
	}

	if len(groups) == 0 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Нет отслеживаемых групп")
		bot.api.Send(msg)
		return
	}

	var response strings.Builder
	response.WriteString("📋 Отслеживаемые группы:\n\n")

	for _, group := range groups {
		response.WriteString(
			fmt.Sprintf("🔹 *%s* ([%s])\nID: `%s`\nПоследняя проверка: %s\n\n",
				group.GroupName,
				strings.ToUpper(group.Network),
				group.GroupID,
				time.Unix(group.LastCheck, 0).Format("2006-01-02 15:04"),
			),
		)
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, response.String())
	msg.ParseMode = "Markdown"
	bot.api.Send(msg)
}

func (bot *Bot) ChatID(message *tgbotapi.Message) {
	msg := tgbotapi.NewMessage(
		message.Chat.ID,
		fmt.Sprintf(
			"ID Чата: `%d`",
			message.Chat.ID,
		),
	)
	msg.ParseMode = "Markdown"
	bot.api.Send(msg)
}

func (bot *Bot) SetChatID(message *tgbotapi.Message) {
	parts := strings.Split(strings.TrimSpace(message.Text), " ")
	if len(parts) < 2 {
		bot.sendError(
			message.Chat.ID,
			"Неверный формат.",
			message.MessageID,
		)
		return
	}

	newIDStr := parts[1]
	newID, err := strconv.ParseInt(newIDStr, 10, 64)
	if err != nil {
		bot.sendError(
			message.Chat.ID,
			"Указан неверный ID",
			message.MessageID,
		)
		return
	}

	bot.conf.Telegram.MonitoringChannelID = newID
	bot.conf.Update()

	msg := tgbotapi.NewMessage(
		message.Chat.ID,
		"ID Чата изменено",
	)
	bot.api.Send(msg)
}

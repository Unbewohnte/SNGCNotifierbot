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
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mymmrac/telego"
)

type Command struct {
	Name        string
	Description string
	Example     string
	Group       string
	Call        func(*telego.Message)
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

func (bot *Bot) Help(message *telego.Message) {
	parts := strings.Split(message.Text, " ")
	if len(parts) >= 2 {
		// Ответить лишь по конкретной команде
		command := bot.CommandByName(parts[1])
		if command != nil {
			helpMessage := constructCommandHelpMessage(*command)
			bot.sendMessage(message.Chat.ID, helpMessage)
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

	bot.sendMessage(message.Chat.ID, helpMessage)
}

func (bot *Bot) About(message *telego.Message) {
	txt := `SNGCNOTIFIER bot - Телеграм бот для оповещения о новых комментариях под постами групп в ВКонтакте, Одноклассники и Телеграм.

Source: https://github.com/Unbewohnte/SNGCNotifierbot
Лицензия: GPLv3`

	bot.answerBack(message, txt, true)
}

func (bot *Bot) AddUser(message *telego.Message) {
	parts := strings.Split(strings.TrimSpace(message.Text), " ")
	if len(parts) < 2 {
		bot.answerBack(message, "ID пользователя не указан", true)
		return
	}

	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		bot.answerBack(message, "Неверный ID пользователя", true)
		return
	}

	for _, allowedID := range bot.conf.Telegram.AllowedUserIDs {
		if id == allowedID {
			bot.answerBack(message, "Этот пользователь уже есть в списке разрешенных.", true)
			return
		}
	}

	bot.conf.Telegram.AllowedUserIDs = append(bot.conf.Telegram.AllowedUserIDs, id)

	// Сохраним в файл
	bot.conf.Update()

	bot.answerBack(message, "Пользователь успешно добавлен!", true)
}

func (bot *Bot) TogglePublicity(message *telego.Message) {
	if bot.conf.Telegram.Public {
		bot.conf.Telegram.Public = false
		bot.answerBack(message, "Доступ к боту теперь только у избранных.", true)
	} else {
		bot.conf.Telegram.Public = true
		bot.answerBack(message, "Доступ к боту теперь у всех.", true)
	}

	// Обновляем конфигурационный файл
	bot.conf.Update()
}

func (bot *Bot) RemoveUser(message *telego.Message) {
	parts := strings.Split(strings.TrimSpace(message.Text), " ")
	if len(parts) < 2 {
		bot.answerBack(message, "ID пользователя не указан", true)
		return
	}

	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		bot.answerBack(message, "Неверный ID пользователя", true)
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
	bot.answerBack(message, "Пользователь успешно удален!", true)
}

func (bot *Bot) PrintConfig(message *telego.Message) {
	var response string = ""

	response += "*Нынешняя конфигурация*: \n"
	response += "\n*[ОБЩЕЕ]*:\n"
	response += fmt.Sprintf("*Общедоступный?*: `%v`\n", bot.conf.Telegram.Public)
	response += fmt.Sprintf("*Разрешенные пользователи*: `%+v`\n", bot.conf.Telegram.AllowedUserIDs)
	response += fmt.Sprintf("*Мониторинговый чат*: `%+v`\n", bot.conf.Telegram.MonitoringChannelID)
	response += fmt.Sprintf("*Раздел:*: `%+v`\n", bot.conf.Telegram.MonitoringThreadID)
	response += fmt.Sprintf("*Пустые комментарии разрешены?:*: `%+v`\n", bot.conf.AllowEmptyComments)

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
	if bot.conf.Social.TG.Token != "" {
		response += "*TG*: Токен имеется\n"
	} else {
		response += "*TG*: Токен отсутствует\n"
	}

	bot.answerBack(message, response, true)
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

func (bot *Bot) AddGroup(message *telego.Message) {
	parts := strings.Split(strings.TrimSpace(message.Text), " ")
	if len(parts) < 3 {
		bot.sendError(message, "Неверный формат. Используйте: /addgroup <сеть> <ID группы>")
		return
	}

	network := strings.ToLower(parts[1])
	groupID := strings.Join(parts[2:], " ") // Объединяем оставшиеся части на случай пробелов в ID

	// Сначала проверяем, существует ли уже такая группа
	existingGroup, err := bot.conf.GetDB().GetGroupByNetworkAndID(network, groupID)
	if err == nil && existingGroup != nil {
		bot.sendError(message,
			fmt.Sprintf("Эта группа уже добавлена:\nНазвание: %s\nID: %s\nДобавлена: %s",
				existingGroup.GroupName,
				existingGroup.GroupID,
				existingGroup.CreatedAt.Local().Format("2006-01-02 15:04")),
		)
		return
	}

	var group db.MonitoredGroup
	switch network {
	case "vk":
		// Нормализуем идентификатор группы ВК
		normalizedID, err := normalizeVKGroupID(groupID)
		if err != nil {
			bot.sendError(message, "Неверный ID группы ВК: "+err.Error())
			return
		}

		// Получаем информацию о группе
		info, err := bot.social.VKClient.GetGroupInfo(context.Background(), normalizedID)
		if err != nil {
			bot.sendError(message, fmt.Sprintf("Ошибка проверки группы: %v", err))
			return
		}

		// Преобразуем ExtraData в JSON
		extraData := map[string]string{
			"screen_name": info.ScreenName,
		}
		extraDataJSON, err := json.Marshal(extraData)
		if err != nil {
			bot.sendError(message, "Ошибка формирования данных группы: "+err.Error())
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
			bot.sendError(message, fmt.Sprintf("Ошибка проверки группы: %v", err))
			return
		}

		// Преобразуем ExtraData в JSON
		extraData := map[string]string{
			"screen_name": info.ScreenName,
		}
		extraDataJSON, err := json.Marshal(extraData)
		if err != nil {
			bot.sendError(message, "Ошибка формирования данных группы: "+err.Error())
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
			bot.sendError(message, "Не удалось определить ID группы")
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
		bot.sendError(message, "Неподдерживаемая социальная сеть")
		return
	}

	id, err := bot.conf.GetDB().AddGroup(&group)
	if err != nil {
		bot.sendError(message, "Ошибка добавления группы: "+err.Error())
		return
	}

	bot.sendSuccess(message, fmt.Sprintf(
		"Группа добавлена:\nНазвание: %s\nID: %s\nID в базе: %d",
		group.GroupName, group.GroupID, id,
	))
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

func (bot *Bot) RemoveGroup(message *telego.Message) {
	parts := strings.Split(strings.TrimSpace(message.Text), " ")
	if len(parts) < 3 {
		bot.sendError(message, "Неверный формат. Используйте: /rmgroup <сеть> <ID группы>")
		return
	}

	network := strings.ToLower(parts[1])
	groupID := parts[2]

	// Сначала проверяем, существует ли такая группа
	existingGroup, err := bot.conf.GetDB().GetGroupByNetworkAndID(network, groupID)
	if err != nil || existingGroup == nil {
		bot.sendError(message,
			fmt.Sprintf("Группа с ID %s не найдена в %s", groupID, network))
		return
	}

	err = bot.conf.GetDB().RemoveGroup(network, groupID)
	if err != nil {
		bot.sendError(message, "Ошибка удаления группы: "+err.Error())
		return
	}

	bot.sendSuccess(message, "Группа успешно удалена")
}

func (bot *Bot) ListGroups(message *telego.Message) {
	groups, err := bot.conf.GetDB().GetGroups()
	if err != nil {
		bot.sendError(message, "Ошибка получения групп: "+err.Error())
		return
	}

	if len(groups) == 0 {
		bot.answerBack(message, "Нет отслеживаемых групп", true)
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

	bot.answerBack(message, response.String(), true)
}

func (bot *Bot) ChatID(message *telego.Message) {
	bot.answerBack(message,
		fmt.Sprintf(
			"ID Чата: `%d`\nID раздела: `%d`",
			message.Chat.ID,
			message.MessageThreadID,
		),
		true,
	)
}

func (bot *Bot) SetChatID(message *telego.Message) {
	parts := strings.Split(strings.TrimSpace(message.Text), " ")
	if len(parts) < 2 {
		bot.sendError(
			message,
			"Неверный формат.",
		)
		return
	}

	newIDStr := parts[1]
	newID, err := strconv.ParseInt(newIDStr, 10, 64)
	if err != nil {
		bot.sendError(
			message,
			"Указан неверный ID",
		)
		return
	}

	bot.conf.Telegram.MonitoringChannelID = newID
	bot.conf.Update()

	bot.answerBack(message, "ID Чата изменено", true)
}

func (bot *Bot) SetThreadID(message *telego.Message) {
	parts := strings.Split(strings.TrimSpace(message.Text), " ")
	if len(parts) < 2 {
		bot.sendError(
			message,
			"Неверный формат.",
		)
		return
	}

	newIDStr := parts[1]
	newID, err := strconv.ParseInt(newIDStr, 10, 64)
	if err != nil {
		bot.sendError(
			message,
			"Указан неверный ID",
		)
		return
	}

	bot.conf.Telegram.MonitoringThreadID = newID
	bot.conf.Update()

	bot.answerBack(message, "ID Раздела изменено", true)
}

func (bot *Bot) ToggleAllowEmptyComments(message *telego.Message) {
	if bot.conf.AllowEmptyComments {
		bot.conf.AllowEmptyComments = false
		bot.answerBack(message, "Не оповещаем о пустых комментариях.", true)
	} else {
		bot.conf.AllowEmptyComments = true
		bot.answerBack(message, "Оповещаем о пустых комментариях.", true)
	}

	// Обновляем конфигурационный файл
	bot.conf.Update()
}

func (bot *Bot) ShowSchedule(message *telego.Message) {
	schedule := bot.conf.Schedule

	status := "❌ отключено"
	if schedule.Enabled {
		status = "✅ включено"
	}

	response := fmt.Sprintf(
		"*Расписание уведомлений*:\n\n"+
			"Статус: %s\n"+
			"Дни недели: %s\n"+
			"Время: %s - %s\n"+
			"Часовой пояс: %s",
		status,
		strings.Join(schedule.DaysOfWeek, ", "),
		schedule.StartTime,
		schedule.EndTime,
		schedule.Timezone,
	)

	response += "\n\n```\n"
	days := []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}
	for _, day := range days {
		active := " "
		for _, activeDay := range schedule.DaysOfWeek {
			if day == activeDay {
				active = "✓"
				break
			}
		}
		response += fmt.Sprintf("%s: [%s] %s - %s\n",
			day,
			active,
			schedule.StartTime,
			schedule.EndTime,
		)
	}
	response += "```"

	bot.sendMessage(message.Chat.ID, response)
}

func (bot *Bot) SetSchedule(message *telego.Message) {
	parts := strings.Split(strings.TrimSpace(message.Text), " ")
	if len(parts) < 6 {
		bot.sendError(message,
			"Неверный формат. Используйте: /setschedule <enabled|disabled> <дни> <начало> <конец> <часовой пояс>\nПример: /setschedule enabled mon,tue,wed,thu,fri 08:00 18:00 Europe/Moscow",
		)
		return
	}

	// Парсим статус
	status := strings.ToLower(parts[1])
	if status != "enabled" && status != "disabled" {
		bot.sendError(
			message,
			"Статус должен быть 'enabled' или 'disabled'",
		)
		return
	}

	// Парсим дни недели
	days := strings.Split(parts[2], ",")
	validDays := map[string]bool{"mon": true, "tue": true, "wed": true, "thu": true, "fri": true, "sat": true, "sun": true}
	for _, day := range days {
		if !validDays[strings.ToLower(day)] {
			bot.sendError(
				message,
				fmt.Sprintf("Недопустимый день недели: %s. Допустимые: mon,tue,wed,thu,fri,sat,sun", day),
			)
			return
		}
	}

	// Парсим время
	startTime := parts[3]
	endTime := parts[4]
	if !isValidTime(startTime) || !isValidTime(endTime) {
		bot.sendError(
			message,
			"Неверный формат времени. Используйте HH:MM",
		)
		return
	}

	// Проверяем, что начало раньше конца
	if startTime > endTime {
		bot.sendError(
			message,
			"Время начала должно быть раньше времени окончания",
		)
		return
	}

	// Часовой пояс
	timezone := parts[5]
	if timezone == "auto" {
		// Определяем часовой пояс сервера
		zone, offset := time.Now().Zone()
		timezone = fmt.Sprintf("Etc/GMT%+d", -offset/3600)
		log.Printf("Автоматический часовой пояс: %s (%s)", timezone, zone)
	} else if _, err := time.LoadLocation(timezone); err != nil {
		bot.sendError(message,
			fmt.Sprintf("Неверный часовой пояс: %s. Пример: Europe/Moscow", timezone),
		)
		return
	}

	// Обновляем конфиг
	bot.conf.Schedule.Enabled = (status == "enabled")
	bot.conf.Schedule.DaysOfWeek = days
	bot.conf.Schedule.StartTime = startTime
	bot.conf.Schedule.EndTime = endTime
	bot.conf.Schedule.Timezone = timezone

	// Сохраняем
	if err := bot.conf.Update(); err != nil {
		bot.sendError(
			message,
			"Ошибка сохранения расписания: "+err.Error(),
		)
		return
	}

	bot.sendSuccess(message, "Расписание успешно обновлено!")
}

// Вспомогательная функция для проверки формата времени
func isValidTime(t string) bool {
	parts := strings.Split(t, ":")
	if len(parts) != 2 {
		return false
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return false
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return false
	}

	return true
}

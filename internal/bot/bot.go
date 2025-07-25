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
	"Unbewohnte/SNGCNOTIFIERbot/internal/bot/social"
	"Unbewohnte/SNGCNOTIFIERbot/internal/bot/social/ok"
	"Unbewohnte/SNGCNOTIFIERbot/internal/bot/social/tg"
	"Unbewohnte/SNGCNOTIFIERbot/internal/bot/social/vk"
	"Unbewohnte/SNGCNOTIFIERbot/internal/db"
	"context"
	"log"
	"strings"
	"time"

	"github.com/mymmrac/telego"
)

type PendingComment struct {
	Group      db.MonitoredGroup
	Comment    db.Comment
	ReceivedAt time.Time
}

type Bot struct {
	api      *telego.Bot
	conf     *Config
	commands []Command
	social   *social.SocialManager
	db       *db.DB
}

func NewBot(config *Config) (*Bot, error) {
	api, err := telego.NewBot(config.Telegram.ApiToken, telego.WithDefaultLogger(false, false))
	if err != nil {
		return nil, err
	}

	// Инициализируем менеджер соцсетей
	socialManager := &social.SocialManager{
		VKClient: vk.NewClient(config.Social.VK.Token),
		OKClient: ok.NewClient(
			config.Social.OK.Token,
			config.Social.OK.PublicKey,
			config.Social.OK.SecretKey,
			config.Social.OK.AppID,
		),
		TGClient: tg.NewClient(api),
	}

	dbase, err := config.OpenDB()
	if err != nil {
		return nil, err
	}

	return &Bot{
		api:    api,
		conf:   config,
		social: socialManager,
		db:     dbase,
	}, nil
}

func (bot *Bot) Init() {
	bot.NewCommand(Command{
		Name:        "help",
		Description: "Напечатать вспомогательное сообщение",
		Group:       "Общее",
		Call:        bot.Help,
	})

	bot.NewCommand(Command{
		Name:        "about",
		Description: "Напечатать информацию о боте",
		Group:       "Общее",
		Call:        bot.About,
	})

	bot.NewCommand(Command{
		Name:        "togglepublic",
		Description: "Включить или выключить публичный/приватный доступ к боту",
		Group:       "Телеграм",
		Call:        bot.TogglePublicity,
	})

	bot.NewCommand(Command{
		Name:        "adduser",
		Description: "Добавить доступ к боту определенному пользователю по ID (напишите боту @userinfobot для получения своего ID)",
		Example:     "/adduser 5293210034",
		Group:       "Телеграм",
		Call:        bot.AddUser,
	})

	bot.NewCommand(Command{
		Name:        "rmuser",
		Description: "Убрать доступ к боту определенному пользователю по ID",
		Example:     "/rmuser 5293210034",
		Group:       "Телеграм",
		Call:        bot.RemoveUser,
	})

	bot.NewCommand(Command{
		Name:        "conf",
		Description: "Написать текущую конфигурацию",
		Group:       "Общее",
		Call:        bot.PrintConfig,
	})

	bot.NewCommand(Command{
		Name:        "addgroup",
		Description: "Добавить группу для мониторинга",
		Example:     "/addgroup vk club123",
		Group:       "Мониторинг",
		Call:        bot.AddGroup,
	})

	bot.NewCommand(Command{
		Name:        "rmgroup",
		Description: "Удалить группу из мониторинга",
		Example:     "/rmgroup vk 123",
		Group:       "Мониторинг",
		Call:        bot.RemoveGroup,
	})

	bot.NewCommand(Command{
		Name:        "listgroups",
		Description: "Показать все отслеживаемые группы",
		Group:       "Мониторинг",
		Call:        bot.ListGroups,
	})

	bot.NewCommand(Command{
		Name:        "chatid",
		Description: "Показать ID канала",
		Group:       "Общее",
		Call:        bot.ChatID,
	})

	bot.NewCommand(Command{
		Name:        "setchatid",
		Description: "Сменить ID чата для отправки сообщений о комментариях",
		Group:       "Общее",
		Call:        bot.SetChatID,
	})

	bot.NewCommand(Command{
		Name:        "setthreadid",
		Description: "Сменить ID раздела для отправки сообщений о комментариях",
		Group:       "Общее",
		Call:        bot.SetThreadID,
	})

	bot.NewCommand(Command{
		Name:        "toggleempty",
		Description: "Включить или выключить оповещения для пустых комментариев",
		Group:       "Общее",
		Call:        bot.ToggleAllowEmptyComments,
	})

	bot.NewCommand(Command{
		Name:        "schedule",
		Description: "Показать текущее расписание уведомлений",
		Group:       "Расписание",
		Call:        bot.ShowSchedule,
	})

	bot.NewCommand(Command{
		Name:        "setschedule",
		Description: "Установить расписание уведомлений",
		Example:     "setschedule enabled mon,tue,wed,thu,fri 08:00 18:00 Europe/Moscow",
		Group:       "Расписание",
		Call:        bot.SetSchedule,
	})
}

func (bot *Bot) Start() error {
	bot.Init()

	log.Printf("Бот авторизован как %s", bot.api.Username())

	bot.StartMonitoring(bot.conf.CheckIntervalMinutes)

	startTime := time.Now()
	retryDelay := 5 * time.Second

	for {
		// Получаем обновления через long polling
		updates, err := bot.api.UpdatesViaLongPolling(
			context.Background(),
			&telego.GetUpdatesParams{
				Timeout: 60,
			},
		)
		if err != nil {
			log.Printf("Ошибка получения обновлений: %v. Переподключение через %v...", err, retryDelay)
			time.Sleep(retryDelay)
			if retryDelay < 300*time.Second {
				retryDelay *= 2
			}
			continue
		}

		// Сбрасываем задержку переподключения при успешном соединении
		retryDelay = 5 * time.Second

		for update := range updates {
			if update.Message == nil {
				continue
			}

			go func(message *telego.Message) {
				// Пропускаем сообщения, пришедшие до старта бота
				if time.Unix(int64(message.Date), 0).Before(startTime) {
					return
				}

				// Обработка комментариев в Telegram группах
				if bot.isMonitoredTelegramGroup(message.Chat.ID) {
					bot.handleTelegramComment(message)
				}

				// Команда ли это
				if !strings.HasPrefix(message.Text, "/") {
					return // Пропускаем
				}

				// Проверка доступа
				if !bot.conf.Telegram.Public {
					var allowed bool = false
					for _, allowedID := range bot.conf.Telegram.AllowedUserIDs {
						if message.From.ID == allowedID {
							allowed = true
							break
						}
					}

					if !allowed {
						bot.answerBack(message, "Вам не разрешено пользоваться этим ботом!", true)
						if bot.conf.Debug {
							log.Printf("Не допустили к общению пользователя %v", message.From.ID)
						}
						return
					}
				}

				log.Printf("[%s] %s", message.From.Username, message.Text)

				// Обработка команд
				message.Text = strings.TrimSpace(message.Text)
				for _, command := range bot.commands {
					if strings.HasPrefix(strings.ToLower(message.Text), "/"+command.Name) {
						go command.Call(message)
						return
					}
				}

				// Неверная команда
				bot.sendCommandSuggestions(message, strings.ToLower(message.Text))
			}(update.Message)
		}

		log.Println("Соединение с Telegram потеряно. Переподключение...")
	}
}

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
	"encoding/json"
	"errors"
	"io"
	"os"
)

var CONFIG_PATH string = ""

type TelegramConf struct {
	ApiToken            string  `json:"api_token"`
	Public              bool    `json:"is_public"`
	AllowedUserIDs      []int64 `json:"allowed_user_ids"`
	MonitoringChannelID int64   `json:"monitoring_channel_id"`
	MonitoringThreadID  int64   `json:"monitoring_thread_id"`
}

type DBConf struct {
	File string `json:"file"`
	db   *db.DB
}

type VKConf struct {
	Token string `json:"token"`
}

type OKConf struct {
	Token     string `json:"token"`
	PublicKey string `json:"public_key"`
	SecretKey string `json:"secret_key"`
	AppID     string `json:"app_id"`
}

type TGConf struct {
	Token string `json:"token"`
}

type SocialConfig struct {
	VK VKConf `json:"vk"`
	OK OKConf `json:"ok"`
	TG TGConf `json:"telegram"`
}

type Config struct {
	Telegram           TelegramConf `json:"telegram"`
	Debug              bool         `json:"debug"`
	DB                 DBConf       `json:"database"`
	Social             SocialConfig `json:"socials"`
	AllowEmptyComments bool         `json:"allow_empty_comments"`
}

func (c *Config) OpenDB() (*db.DB, error) {
	var err error
	c.DB.db, err = db.NewDB(c.DB.File)
	if err != nil {
		return nil, err
	}

	return c.DB.db, nil
}

func (c *Config) GetDB() *db.DB {
	return c.DB.db
}

func DefaultConfig() *Config {
	return &Config{
		Telegram: TelegramConf{
			ApiToken: "tg_token",
			Public:   true,
		},
		DB: DBConf{
			File: "DB.sqlite3",
		},
		Social: SocialConfig{
			VK: VKConf{
				Token: "vk_user_token",
			},
			OK: OKConf{
				Token:     "token",
				PublicKey: "pub_key",
				SecretKey: "secret_key",
				AppID:     "app_id",
			},
			TG: TGConf{
				Token: "token",
			},
		},
		Debug:              false,
		AllowEmptyComments: true,
	}
}

func (conf *Config) Save(filepath string) error {
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return err
	}
	defer file.Close()

	jsonBytes, err := json.MarshalIndent(&conf, "", "\t")
	if err != nil {
		return err
	}

	_, err = file.Write(jsonBytes)

	// Запоминаем, куда сохранили
	CONFIG_PATH = filepath

	return err
}

func ConfigFrom(filepath string) (*Config, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	contents, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var conf Config
	err = json.Unmarshal(contents, &conf)
	if err != nil {
		return nil, err
	}

	// Запоминаем, откуда взяли
	CONFIG_PATH = filepath

	return &conf, nil
}

// Обновляет конфигурационный файл
func (conf *Config) Update() error {
	if CONFIG_PATH == "" {
		return errors.New("неизвестен путь к конфигурационному файлу")
	}

	return conf.Save(CONFIG_PATH)
}

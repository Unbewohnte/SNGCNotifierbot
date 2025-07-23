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

package ok

import (
	"Unbewohnte/SNGCNOTIFIERbot/internal/bot/social"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	apiURL = "https://api.ok.ru/fb.do"
)

type Client struct {
	accessToken string
	publicKey   string
	secretKey   string
	appID       string
	http        *http.Client
}

func NewClient(accessToken, publicKey, secretKey, appID string) *Client {
	return &Client{
		accessToken: accessToken,
		publicKey:   publicKey,
		secretKey:   secretKey,
		appID:       appID,
		http:        &http.Client{Timeout: 30 * time.Second},
	}
}

// signRequest создает подпись для запроса по спецификации OK API
func (c *Client) signRequest(params url.Values) string {
	// 1. Собираем все параметры в строку вида key1=value1key2=value2...
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(params.Get(k))
	}

	// 2. Добавляем секретный ключ
	sb.WriteString(c.secretKey)

	// 3. Вычисляем MD5
	hash := md5.Sum([]byte(sb.String()))
	return strings.ToLower(hex.EncodeToString(hash[:]))
}

func (c *Client) callMethod(ctx context.Context, method string, params url.Values) (json.RawMessage, error) {
	// Устанавливаем общие параметры
	params.Set("application_key", c.publicKey)
	params.Set("format", "json")
	params.Set("method", method)

	// Добавляем access_token, если он есть
	if c.accessToken != "" {
		params.Set("access_token", c.accessToken)
	}

	// Создаем подпись
	sig := c.signRequest(params)
	params.Set("sig", sig)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Читаем весь ответ как JSON.RawMessage
	var rawResponse json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&rawResponse); err != nil {
		return nil, fmt.Errorf("failed to decode raw response: %w", err)
	}

	return rawResponse, nil
}

// GetGroupName возвращает название группы
func (c *Client) GetGroupName(ctx context.Context, groupID string) (string, error) {
	info, err := c.GetGroupInfo(ctx, groupID)
	if err != nil {
		return "", err
	}
	return info.Name, nil
}

func isValidOKGroupID(groupID string) bool {
	// ID группы в OK может быть числовым или строковым
	// Проверяем, что не пустой и не содержит запрещенных символов
	return len(groupID) > 0 && !strings.ContainsAny(groupID, " \t\n\r")
}

// GetGroupInfo возвращает информацию о группе
func (c *Client) GetGroupInfo(ctx context.Context, groupIdentifier string) (*social.GroupInfo, error) {
	// Сначала пробуем получить информацию напрямую
	info, err := c.tryGetGroupInfo(ctx, groupIdentifier)
	if err == nil {
		return info, nil
	}

	// Если не получилось, пробуем извлечь ID из короткого имени
	if !isNumeric(groupIdentifier) {
		groupID, err := c.resolveGroupID(ctx, groupIdentifier)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve group ID: %w", err)
		}
		return c.tryGetGroupInfo(ctx, groupID)
	}

	return nil, fmt.Errorf("failed to get group info: %s", err)
}
func (c *Client) tryGetGroupInfo(ctx context.Context, groupID string) (*social.GroupInfo, error) {
	params := url.Values{}
	params.Set("uids", groupID)
	params.Set("fields", "name,description,members_count")

	response, err := c.callMethod(ctx, "group.getInfo", params)
	if err != nil {
		return nil, err
	}

	var result []struct {
		UID         string `json:"uid"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Members     int    `json:"members_count"`
	}

	if err := json.Unmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal group info: %w", err)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("group not found")
	}

	group := result[0]

	return &social.GroupInfo{
		ID:         groupID,
		Name:       group.Name,
		ScreenName: group.UID,
	}, nil
}

func isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func (c *Client) resolveGroupID(ctx context.Context, shortName string) (string, error) {
	urlToCheck := fmt.Sprintf("https://ok.ru/group/%s ", shortName)

	params := url.Values{}
	params.Set("url", urlToCheck)

	response, err := c.callMethod(ctx, "url.getInfo", params)
	if err != nil {
		return "", fmt.Errorf("failed to resolve group ID: %w", err)
	}

	var result struct {
		ID int64 `json:"objectId"`
	}

	if err := json.Unmarshal(response, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal url info: %w", err)
	}

	return fmt.Sprintf("%d", result.ID), nil
}

// Получаем информацию о пользователе по ID
func (c *Client) getUserName(ctx context.Context, userID string) (string, error) {
	params := url.Values{}
	params.Set("uids", userID)
	params.Set("fields", "name") // Запрашиваем только имя пользователя

	response, err := c.callMethod(ctx, "users.getInfo", params)
	if err != nil {
		return "", err
	}

	// Парсим ответ (массив пользователей)
	var users []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(response, &users); err != nil {
		return "", err
	}

	// Возвращаем имя, если пользователь найден
	if len(users) > 0 && users[0].Name != "" {
		return users[0].Name, nil
	}

	return "", fmt.Errorf("user not found")
}

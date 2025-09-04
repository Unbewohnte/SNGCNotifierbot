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

package vk

import (
	"Unbewohnte/SNGCNOTIFIERbot/internal/bot/social"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	apiVersion = "5.131"
	apiURL     = "https://api.vk.ru/method/"
)

type Client struct {
	token      string
	http       *http.Client
	usersCache map[int]UserInfo
	cacheMutex sync.Mutex
}

func NewClient(token string) *Client {
	return &Client{
		token:      token,
		http:       &http.Client{Timeout: 30 * time.Second},
		usersCache: make(map[int]UserInfo),
		cacheMutex: sync.Mutex{},
	}
}

func (c *Client) callMethod(ctx context.Context, method string, params url.Values) (json.RawMessage, error) {
	params.Set("access_token", c.token)
	params.Set("v", apiVersion)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL+method, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.URL.RawQuery = params.Encode()

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Response json.RawMessage `json:"response"`
		Error    *VKError        `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error != nil {
		return nil, result.Error
	}

	return result.Response, nil
}

type VKError struct {
	Code    int    `json:"error_code"`
	Message string `json:"error_msg"`
}

func (e *VKError) Error() string {
	return fmt.Sprintf("VK API error %d: %s", e.Code, e.Message)
}

func (c *Client) normalizeGroupIdentifier(input string) (string, bool) {
	// Если это числовой ID (12345, club12345)
	cleaned := strings.TrimPrefix(input, "club")
	if _, err := strconv.Atoi(cleaned); err == nil {
		return cleaned, true // is_numeric = true
	}

	// Если это короткое имя (ustdon)
	if strings.TrimSpace(input) != "" {
		return input, false // is_numeric = false
	}

	return "", false
}

type vkGroupResponse struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	ScreenName string `json:"screen_name"`
}

// GetGroupInfo возвращает информацию о группе
func (c *Client) GetGroupInfo(ctx context.Context, groupIdentifier string) (*social.GroupInfo, error) {
	normalized, isNumeric := c.normalizeGroupIdentifier(groupIdentifier)
	if normalized == "" {
		return nil, fmt.Errorf("invalid group identifier")
	}

	params := url.Values{}
	if isNumeric {
		params.Set("group_id", normalized)
	} else {
		params.Set("group_ids", normalized)
	}
	params.Set("fields", "name,description,members_count")

	response, err := c.callMethod(ctx, "groups.getById", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get group info: %w", err)
	}

	// Используем промежуточную структуру для разбора
	var vkGroups []vkGroupResponse
	if err := json.Unmarshal(response, &vkGroups); err != nil {
		return nil, fmt.Errorf("failed to unmarshal group info: %w", err)
	}

	if len(vkGroups) == 0 {
		return nil, fmt.Errorf("group not found")
	}

	// Конвертируем в целевую структуру
	vkGroup := vkGroups[0]
	return &social.GroupInfo{
		ID:         strconv.Itoa(vkGroup.ID), // Конвертируем int в string
		Name:       vkGroup.Name,
		ScreenName: vkGroup.ScreenName,
	}, nil
}

// GetGroupName возвращает название группы
func (c *Client) GetGroupName(ctx context.Context, groupID string) (string, error) {
	info, err := c.GetGroupInfo(ctx, groupID)
	if err != nil {
		return "", err
	}
	return info.Name, nil
}
func (c *Client) getUsersInfo(ctx context.Context, userIDs []int) (map[int]UserInfo, error) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	// Проверяем кэш
	result := make(map[int]UserInfo)
	var missingIDs []int

	for _, id := range userIDs {
		if user, exists := c.usersCache[id]; exists {
			result[id] = user
		} else {
			missingIDs = append(missingIDs, id)
		}
	}

	if len(missingIDs) == 0 {
		return result, nil
	}

	// Запрашиваем только недостающие ID
	users, err := c.fetchUsersInfo(ctx, missingIDs)
	if err != nil {
		return nil, err
	}

	// Обновляем кэш и результат
	for id, user := range users {
		c.usersCache[id] = user
		result[id] = user
	}

	return result, nil
}

type UserInfo struct {
	ID        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func (c *Client) fetchUsersInfo(ctx context.Context, userIDs []int) (map[int]UserInfo, error) {
	// Разбиваем на группы по 1000 пользователей
	batchSize := 1000
	result := make(map[int]UserInfo)

	for i := 0; i < len(userIDs); i += batchSize {
		end := i + batchSize
		if end > len(userIDs) {
			end = len(userIDs)
		}

		batch := userIDs[i:end]
		users, err := c.makeUsersRequest(ctx, batch)
		if err != nil {
			return nil, err
		}

		for id, user := range users {
			result[id] = user
		}
	}

	return result, nil
}

func (c *Client) makeUsersRequest(ctx context.Context, userIDs []int) (map[int]UserInfo, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}

	// Преобразуем IDs в строку
	ids := make([]string, len(userIDs))
	for i, id := range userIDs {
		ids[i] = strconv.Itoa(id)
	}
	idsStr := strings.Join(ids, ",")

	params := url.Values{}
	params.Set("user_ids", idsStr)
	params.Set("fields", "first_name,last_name")

	response, err := c.callMethod(ctx, "users.get", params)
	if err != nil {
		return nil, err
	}

	var users []UserInfo
	if err := json.Unmarshal(response, &users); err != nil {
		return nil, err
	}

	result := make(map[int]UserInfo)
	for _, user := range users {
		result[user.ID] = user
	}

	return result, nil
}

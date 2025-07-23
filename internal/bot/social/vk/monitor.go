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
	"Unbewohnte/SNGCNOTIFIERbot/internal/db"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

func (c *Client) GetComments(ctx context.Context, groupID string, lastCheck int64) ([]db.Comment, error) {
	// Нормализуем ID группы
	normalizedID, isNumeric := c.normalizeGroupIdentifier(groupID)
	if normalizedID == "" {
		return nil, fmt.Errorf("invalid group identifier")
	}

	// Если это не числовой ID, получаем числовой ID через API
	if !isNumeric {
		info, err := c.GetGroupInfo(ctx, normalizedID)
		if err != nil {
			return nil, fmt.Errorf("failed to get group info: %w", err)
		}
		normalizedID = info.ID
	}

	// Получаем посты
	posts, err := c.getWallPosts(ctx, normalizedID)
	if err != nil {
		return nil, fmt.Errorf("failed to get posts: %w", err)
	}

	// log.Printf("Посты (%v): %+v", groupID, posts)

	var comments []db.Comment

	// Получаем комментарии для каждого поста
	for _, post := range posts {
		postComments, err := c.getNewPostComments(ctx, normalizedID, post.ID, lastCheck)
		if err != nil {
			continue // Пропускаем посты с ошибками
		}
		comments = append(comments, postComments...)
	}

	return comments, nil
}

func (c *Client) getWallPosts(ctx context.Context, groupIdentifier string) ([]WallPost, error) {
	normalized, isNumeric := c.normalizeGroupIdentifier(groupIdentifier)
	if normalized == "" {
		return nil, fmt.Errorf("invalid group identifier")
	}

	params := url.Values{}
	if isNumeric {
		params.Set("owner_id", "-"+normalized) // Для числовых ID
	} else {
		params.Set("domain", normalized) // Для коротких имен
	}
	params.Set("count", "50")     // Получаем N последних постов
	params.Set("filter", "owner") // Только посты владельца группы

	response, err := c.callMethod(ctx, "wall.get", params)
	if err != nil {
		return nil, err
	}

	var result struct {
		Items []WallPost `json:"items"`
	}
	if err := json.Unmarshal(response, &result); err != nil {
		return nil, err
	}

	return result.Items, nil
}

type WallPost struct {
	ID    int    `json:"id"`
	Text  string `json:"text"`
	Date  int64  `json:"date"`
	Likes struct {
		Count int `json:"count"`
	} `json:"likes"`
	Reposts struct {
		Count int `json:"count"`
	} `json:"reposts"`
}

func (c *Client) getNewPostComments(ctx context.Context, groupID string, postID int, lastCheck int64) ([]db.Comment, error) {
	params := url.Values{}
	params.Set("owner_id", "-"+groupID)
	params.Set("post_id", strconv.Itoa(postID))
	params.Set("need_likes", "1")
	params.Set("count", "100") // Максимальное количество комментариев
	params.Set("sort", "desc") // Сначала новые

	response, err := c.callMethod(ctx, "wall.getComments", params)
	if err != nil {
		return nil, err
	}

	// log.Printf("RESPONSE КОММЕНТАРИИ: %s", response)

	var result struct {
		Items []VKComment `json:"items"`
	}
	if err := json.Unmarshal(response, &result); err != nil {
		return nil, err
	}

	// Собираем ID пользователей для запроса информации
	userIDs := make([]int, 0, len(result.Items))
	for _, comment := range result.Items {
		if comment.FromID > 0 { // Игнорируем отрицательные ID (группы)
			userIDs = append(userIDs, comment.FromID)
		}
	}

	// Получаем информацию о пользователях
	usersInfo, err := c.getUsersInfo(ctx, userIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get users info: %w", err)
	}

	var comments []db.Comment
	postURL := fmt.Sprintf("https://vk.com/wall-%s_%d", groupID, postID)

	for _, comment := range result.Items {
		if comment.Date <= lastCheck {
			continue
		}

		// Формируем имя автора
		var authorName string
		if user, exists := usersInfo[comment.FromID]; exists {
			authorName = fmt.Sprintf("%s %s", user.FirstName, user.LastName)
		} else {
			authorName = fmt.Sprintf("Пользователь #%d", comment.FromID)
		}

		comments = append(comments, db.Comment{
			ID:        strconv.Itoa(comment.ID),
			Author:    authorName,
			Text:      comment.Text,
			Timestamp: comment.Date,
			PostURL:   postURL,
		})
	}

	return comments, nil
}

type VKComment struct {
	ID      int    `json:"id"`
	FromID  int    `json:"from_id"`
	Text    string `json:"text"`
	Date    int64  `json:"date"`
	PostID  int    `json:"post_id"`
	OwnerID int    `json:"owner_id"`
	Likes   struct {
		Count int `json:"count"`
	} `json:"likes"`
}

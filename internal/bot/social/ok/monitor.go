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
	"Unbewohnte/SNGCNOTIFIERbot/internal/db"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

func (c *Client) GetComments(ctx context.Context, groupID string, lastCheck int64) ([]db.Comment, error) {
	if !isValidOKGroupID(groupID) {
		return nil, fmt.Errorf("invalid group ID")
	}

	// 1. Получаем последние посты
	posts, err := c.getGroupFeed(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get posts: %w", err)
	}

	var comments []db.Comment

	// 2. Для каждого поста получаем комментарии
	for _, post := range posts {
		if post.Type != "GROUP_THEME" {
			continue
		}

		postComments, err := c.getPostComments(ctx, post.ID, groupID, lastCheck)
		if err != nil {
			continue
		}

		comments = append(comments, postComments...)
	}

	return comments, nil
}

func (c *Client) getGroupFeed(ctx context.Context, groupID string) ([]OKPost, error) {
	params := url.Values{}
	params.Set("gid", groupID)
	params.Set("count", "50") // N последних постов

	response, err := c.callMethod(ctx, "mediatopic.getTopics", params)
	if err != nil {
		return nil, err
	}

	var feed struct {
		MediaTopics []OKTopic `json:"media_topics"`
	}

	if err := json.Unmarshal(response, &feed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal feed: %w", err)
	}

	// Преобразуем OKTopic в OKPost
	var posts []OKPost
	for _, topic := range feed.MediaTopics {
		// log.Printf("TOPIC: %+v", topic)

		// Извлекаем текст из media
		var text string
		for _, media := range topic.Media {
			if media.Type == "text" && media.Text != "" {
				text = media.Text
				break
			}
		}

		// Формируем автора (группа)
		authorID := extractOKGroupIDFromRef(topic.AuthorRef) // Например: "group:55348644610059" → "55348644610059"
		posts = append(posts, OKPost{
			ID:      topic.ID,
			Type:    "GROUP_THEME",
			Created: topic.Created,
			Author: struct {
				ID   string `json:"uid"`
				Name string `json:"name"`
			}{
				ID:   authorID,
				Name: "",
			},
			Text: text,
		})
	}

	return posts, nil
}

func extractOKGroupIDFromRef(ref string) string {
	parts := strings.Split(ref, ":")
	if len(parts) == 2 && parts[0] == "group" {
		return parts[1]
	}
	return ""
}

type OKTopic struct {
	ID        string `json:"id"`
	Created   int64  `json:"created_ms"`
	AuthorRef string `json:"author_ref"` // Например: "group:55348644610059"
	OwnerRef  string `json:"owner_ref"`  // Например: "group:55348644610059"
	Media     []struct {
		Type       string `json:"type"`
		Text       string `json:"text"` // Текст поста
		TextTokens []struct {
			Text string `json:"text"`
		} `json:"text_tokens"`
	} `json:"media"`
}

type OKPost struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Created int64  `json:"created_ms"`
	Author  struct {
		ID   string `json:"uid"`
		Name string `json:"name"`
	} `json:"author"`
	Text string `json:"text"`
}

func (c *Client) getPostComments(ctx context.Context, postID, groupID string, lastCheck int64) ([]db.Comment, error) {
	params := url.Values{}
	params.Set("discussionId", postID)
	params.Set("discussionType", "GROUP_TOPIC")
	params.Set("count", "100")

	response, err := c.callMethod(ctx, "discussions.getComments", params)
	if err != nil {
		return nil, err
	}

	var result OKCommentResponse
	if err := json.Unmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal comments: %w", err)
	}

	// Создаем мапу для быстрого поиска имени автора
	userMap := make(map[string]string)
	for _, user := range result.Entities.Users {
		userMap[user.ID] = user.Name
	}

	var comments []db.Comment
	postURL := fmt.Sprintf("https://ok.ru/group/%s/topic/%s ", groupID, postID)

	for _, comment := range result.Comments {
		commentTime, err := time.ParseInLocation("2006-01-02 15:04:05", comment.Date, time.Local)
		if err != nil {
			continue
		}

		// Конвертируем в UTC для хранения и сравнения
		commentTimeUTC := commentTime.UTC()

		// log.Printf("LOCAL TIME: %v | UTC: %v | UNIX (UTC: %v - lastCheck: %v)",
		// 	commentTime, commentTimeUTC, commentTimeUTC.Unix(), lastCheck)

		if commentTimeUTC.Unix() <= lastCheck {
			continue
		}

		authorName := userMap[comment.AuthorID]
		if authorName == "" {
			// Делаем дополнительный запрос к users.getInfo
			name, err := c.getUserName(ctx, comment.AuthorID)
			if err != nil {
				name = "Unknown"
			}
			authorName = name
		}

		comments = append(comments, db.Comment{
			ID:        comment.ID,
			Author:    authorName,
			Text:      comment.Text,
			Timestamp: commentTimeUTC.Unix(),
			PostURL:   postURL,
		})
	}

	return comments, nil
}

type OKCommentResponse struct {
	Comments []OKComment `json:"comments"`
	Entities struct {
		Users []OKAuthor `json:"users"`
	} `json:"entities,omitempty"`
}

type OKComment struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	Date       string `json:"date"`
	AuthorID   string `json:"author_id"`
	AuthorName string `json:"-"`
}

type OKAuthor struct {
	ID   string `json:"uid"`
	Name string `json:"name"`
}

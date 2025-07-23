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

package social

import (
	"Unbewohnte/SNGCNOTIFIERbot/internal/db"
	"context"
	"fmt"
)

type GroupInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ScreenName string `json:"screen_name"`
}

type APIClient interface {
	GetGroupName(ctx context.Context, groupID string) (string, error)
	GetComments(ctx context.Context, groupID string, lastCheck int64) ([]db.Comment, error)
	GetGroupInfo(ctx context.Context, groupIdentifier string) (*GroupInfo, error)
}

type SocialManager struct {
	VKClient APIClient
	OKClient APIClient
	TGClient APIClient
}

func (sm *SocialManager) GetGroupName(network, groupID string) (string, error) {
	ctx := context.Background()

	switch network {
	case "vk":
		return sm.VKClient.GetGroupName(ctx, groupID)
	case "ok":
		return sm.OKClient.GetGroupName(ctx, groupID)
	case "tg":
		return sm.TGClient.GetGroupName(ctx, groupID)
	default:
		return "", fmt.Errorf("unsupported network: %s", network)
	}
}

type Monitor interface {
	CheckNewComments(groupID int64, lastCheck int64) ([]db.Comment, error)
}

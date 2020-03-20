// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package commands

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost-server/v5/app"
	"github.com/mattermost/mattermost-server/v5/model"
)

const CHANNEL_ARG_SEPARATOR = ":"

func getChannelsFromChannelArgs(a *app.App, channelArgs []string) []*model.Channel {
	channels := make([]*model.Channel, 0, len(channelArgs))
	for _, channelArg := range channelArgs {
		channel := getChannelFromChannelArg(a, channelArg)
		channels = append(channels, channel)
	}
	return channels
}

func parseChannelArg(channelArg string) (string, string) {
	result := strings.SplitN(channelArg, CHANNEL_ARG_SEPARATOR, 2)
	if len(result) == 1 {
		return "", channelArg
	}
	return result[0], result[1]
}

func getChannelFromChannelArg(a *app.App, channelArg string) *model.Channel {
	teamArg, channelPart := parseChannelArg(channelArg)
	if teamArg == "" && channelPart == "" {
		return nil
	}

	var channel *model.Channel
	if teamArg != "" {
		team := getTeamFromTeamArg(a, teamArg)
		if team == nil {
			return nil
		}

		if result, err := a.Srv().Store.Channel().GetByNameIncludeDeleted(team.Id, channelPart, true); err == nil {
			channel = result
		} else {
			fmt.Println(err.Error())
		}
	}

	if channel == nil {
		if ch, errCh := a.Srv().Store.Channel().Get(channelPart, true); errCh == nil {
			channel = ch
		}
	}

	return channel
}

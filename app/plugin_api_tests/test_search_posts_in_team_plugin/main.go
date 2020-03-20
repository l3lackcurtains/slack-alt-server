// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"fmt"

	"github.com/mattermost/mattermost-server/v5/app/plugin_api_tests"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
)

type MyPlugin struct {
	plugin.MattermostPlugin
	configuration plugin_api_tests.BasicConfig
}

func (p *MyPlugin) OnConfigurationChange() error {
	if err := p.API.LoadPluginConfiguration(&p.configuration); err != nil {
		return err
	}
	return nil
}

func (p *MyPlugin) MessageWillBePosted(c *plugin.Context, post *model.Post) (*model.Post, string) {
	testCases := []struct {
		description      string
		teamID           string
		params           []*model.SearchParams
		expectedPostsLen int
	}{
		{
			"nil params",
			p.configuration.BasicTeamId,
			nil,
			0,
		},
		{
			"empty params",
			p.configuration.BasicTeamId,
			[]*model.SearchParams{},
			0,
		},
		{
			"doesn't match any posts",
			p.configuration.BasicTeamId,
			model.ParseSearchParams("bad message", 0),
			0,
		},
		{
			"matched posts",
			p.configuration.BasicTeamId,
			model.ParseSearchParams(p.configuration.BasicPostMessage, 0),
			1,
		},
	}

	for _, testCase := range testCases {
		posts, err := p.API.SearchPostsInTeam(testCase.teamID, testCase.params)
		if err != nil {
			return nil, fmt.Sprintf("%v: %v", testCase.description, err.Error())
		}
		if testCase.expectedPostsLen != len(posts) {
			return nil, fmt.Sprintf("%v: invalid number of posts: %v != %v", testCase.description, testCase.expectedPostsLen, len(posts))
		}
	}
	return nil, "OK"
}

func main() {
	plugin.ClientMain(&MyPlugin{})
}

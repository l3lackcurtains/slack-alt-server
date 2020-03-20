// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/utils"
)

func TestSendNotifications(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.AddUserToChannel(th.BasicUser2, th.BasicChannel)

	post1, appErr := th.App.CreatePostMissingChannel(&model.Post{
		UserId:    th.BasicUser.Id,
		ChannelId: th.BasicChannel.Id,
		Message:   "@" + th.BasicUser2.Username,
		Type:      model.POST_ADD_TO_CHANNEL,
		Props:     map[string]interface{}{model.POST_PROPS_ADDED_USER_ID: "junk"},
	}, true)
	require.Nil(t, appErr)

	mentions, err := th.App.SendNotifications(post1, th.BasicTeam, th.BasicChannel, th.BasicUser, nil)
	require.NoError(t, err)
	require.NotNil(t, mentions)
	require.True(t, utils.StringInSlice(th.BasicUser2.Id, mentions), "mentions", mentions)

	dm, appErr := th.App.GetOrCreateDirectChannel(th.BasicUser.Id, th.BasicUser2.Id)
	require.Nil(t, appErr)

	post2, appErr := th.App.CreatePostMissingChannel(&model.Post{
		UserId:    th.BasicUser.Id,
		ChannelId: dm.Id,
		Message:   "dm message",
	}, true)
	require.Nil(t, appErr)

	mentions, err = th.App.SendNotifications(post2, th.BasicTeam, dm, th.BasicUser, nil)
	require.NoError(t, err)
	require.NotNil(t, mentions)

	_, appErr = th.App.UpdateActive(th.BasicUser2, false)
	require.Nil(t, appErr)
	appErr = th.App.InvalidateAllCaches()
	require.Nil(t, appErr)

	post3, appErr := th.App.CreatePostMissingChannel(&model.Post{
		UserId:    th.BasicUser.Id,
		ChannelId: dm.Id,
		Message:   "dm message",
	}, true)
	require.Nil(t, appErr)

	mentions, err = th.App.SendNotifications(post3, th.BasicTeam, dm, th.BasicUser, nil)
	require.NoError(t, err)
	require.NotNil(t, mentions)

	th.BasicChannel.DeleteAt = 1
	mentions, err = th.App.SendNotifications(post1, th.BasicTeam, th.BasicChannel, th.BasicUser, nil)
	require.NoError(t, err)
	require.Empty(t, mentions)
}

func TestSendNotificationsWithManyUsers(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	users := []*model.User{}
	for i := 0; i < 10; i++ {
		user := th.CreateUser()
		th.LinkUserToTeam(user, th.BasicTeam)
		th.App.AddUserToChannel(user, th.BasicChannel)
		users = append(users, user)
	}

	_, appErr1 := th.App.CreatePostMissingChannel(&model.Post{
		UserId:    th.BasicUser.Id,
		ChannelId: th.BasicChannel.Id,
		Message:   "@channel",
		Type:      model.POST_ADD_TO_CHANNEL,
		Props:     map[string]interface{}{model.POST_PROPS_ADDED_USER_ID: "junk"},
	}, true)
	require.Nil(t, appErr1)

	// Each user should have a mention count of exactly 1 in the DB at this point.
	t.Run("1-mention", func(t *testing.T) {
		for i, user := range users {
			t.Run(fmt.Sprintf("user-%d", i+1), func(t *testing.T) {
				channelUnread, appErr2 := th.Server.Store.Channel().GetChannelUnread(th.BasicChannel.Id, user.Id)
				require.Nil(t, appErr2)
				assert.Equal(t, int64(1), channelUnread.MentionCount)
			})
		}
	})

	_, appErr1 = th.App.CreatePostMissingChannel(&model.Post{
		UserId:    th.BasicUser.Id,
		ChannelId: th.BasicChannel.Id,
		Message:   "@channel",
		Type:      model.POST_ADD_TO_CHANNEL,
		Props:     map[string]interface{}{model.POST_PROPS_ADDED_USER_ID: "junk"},
	}, true)
	require.Nil(t, appErr1)

	// Now each user should have a mention count of exactly 2 in the DB.
	t.Run("2-mentions", func(t *testing.T) {
		for i, user := range users {
			t.Run(fmt.Sprintf("user-%d", i+1), func(t *testing.T) {
				channelUnread, appErr2 := th.Server.Store.Channel().GetChannelUnread(th.BasicChannel.Id, user.Id)
				require.Nil(t, appErr2)
				assert.Equal(t, int64(2), channelUnread.MentionCount)
			})
		}
	})
}

func TestSendOutOfChannelMentions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channel := th.BasicChannel

	user1 := th.BasicUser
	user2 := th.BasicUser2

	t.Run("should send ephemeral post when there is an out of channel mention", func(t *testing.T) {
		post := &model.Post{}
		potentialMentions := []string{user2.Username}

		sent, err := th.App.sendOutOfChannelMentions(user1, post, channel, potentialMentions)

		assert.Nil(t, err)
		assert.True(t, sent)
	})

	t.Run("should not send ephemeral post when there are no out of channel mentions", func(t *testing.T) {
		post := &model.Post{}
		potentialMentions := []string{"not a user"}

		sent, err := th.App.sendOutOfChannelMentions(user1, post, channel, potentialMentions)

		assert.Nil(t, err)
		assert.False(t, sent)
	})
}

func TestFilterOutOfChannelMentions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channel := th.BasicChannel

	user1 := th.BasicUser
	user2 := th.BasicUser2
	user3 := th.CreateUser()
	th.LinkUserToTeam(user3, th.BasicTeam)

	t.Run("should return users not in the channel", func(t *testing.T) {
		post := &model.Post{}
		potentialMentions := []string{user2.Username, user3.Username}

		outOfChannelUsers, outOfGroupUsers, err := th.App.filterOutOfChannelMentions(user1, post, channel, potentialMentions)

		assert.Nil(t, err)
		assert.Len(t, outOfChannelUsers, 2)
		assert.True(t, (outOfChannelUsers[0].Id == user2.Id || outOfChannelUsers[1].Id == user2.Id))
		assert.True(t, (outOfChannelUsers[0].Id == user3.Id || outOfChannelUsers[1].Id == user3.Id))
		assert.Nil(t, outOfGroupUsers)
	})

	t.Run("should not return results for a system message", func(t *testing.T) {
		post := &model.Post{
			Type: model.POST_ADD_REMOVE,
		}
		potentialMentions := []string{user2.Username, user3.Username}

		outOfChannelUsers, outOfGroupUsers, err := th.App.filterOutOfChannelMentions(user1, post, channel, potentialMentions)

		assert.Nil(t, err)
		assert.Nil(t, outOfChannelUsers)
		assert.Nil(t, outOfGroupUsers)
	})

	t.Run("should not return results for a direct message", func(t *testing.T) {
		post := &model.Post{}
		directChannel := &model.Channel{
			Type: model.CHANNEL_DIRECT,
		}
		potentialMentions := []string{user2.Username, user3.Username}

		outOfChannelUsers, outOfGroupUsers, err := th.App.filterOutOfChannelMentions(user1, post, directChannel, potentialMentions)

		assert.Nil(t, err)
		assert.Nil(t, outOfChannelUsers)
		assert.Nil(t, outOfGroupUsers)
	})

	t.Run("should not return results for a group message", func(t *testing.T) {
		post := &model.Post{}
		groupChannel := &model.Channel{
			Type: model.CHANNEL_GROUP,
		}
		potentialMentions := []string{user2.Username, user3.Username}

		outOfChannelUsers, outOfGroupUsers, err := th.App.filterOutOfChannelMentions(user1, post, groupChannel, potentialMentions)

		assert.Nil(t, err)
		assert.Nil(t, outOfChannelUsers)
		assert.Nil(t, outOfGroupUsers)
	})

	t.Run("should not return inactive users", func(t *testing.T) {
		inactiveUser := th.CreateUser()
		inactiveUser, appErr := th.App.UpdateActive(inactiveUser, false)
		require.Nil(t, appErr)

		post := &model.Post{}
		potentialMentions := []string{inactiveUser.Username}

		outOfChannelUsers, outOfGroupUsers, err := th.App.filterOutOfChannelMentions(user1, post, channel, potentialMentions)

		assert.Nil(t, err)
		assert.Nil(t, outOfChannelUsers)
		assert.Nil(t, outOfGroupUsers)
	})

	t.Run("should not return bot users", func(t *testing.T) {
		botUser := th.CreateUser()
		botUser.IsBot = true

		post := &model.Post{}
		potentialMentions := []string{botUser.Username}

		outOfChannelUsers, outOfGroupUsers, err := th.App.filterOutOfChannelMentions(user1, post, channel, potentialMentions)

		assert.Nil(t, err)
		assert.Nil(t, outOfChannelUsers)
		assert.Nil(t, outOfGroupUsers)
	})

	t.Run("should not return results for non-existent users", func(t *testing.T) {
		post := &model.Post{}
		potentialMentions := []string{"foo", "bar"}

		outOfChannelUsers, outOfGroupUsers, err := th.App.filterOutOfChannelMentions(user1, post, channel, potentialMentions)

		assert.Nil(t, err)
		assert.Nil(t, outOfChannelUsers)
		assert.Nil(t, outOfGroupUsers)
	})

	t.Run("should separate users not in the channel from users not in the group", func(t *testing.T) {
		nonChannelMember := th.CreateUser()
		th.LinkUserToTeam(nonChannelMember, th.BasicTeam)
		nonGroupMember := th.CreateUser()
		th.LinkUserToTeam(nonGroupMember, th.BasicTeam)

		group := th.CreateGroup()
		_, appErr := th.App.UpsertGroupMember(group.Id, th.BasicUser.Id)
		require.Nil(t, appErr)
		_, appErr = th.App.UpsertGroupMember(group.Id, nonChannelMember.Id)
		require.Nil(t, appErr)

		constrainedChannel := th.CreateChannel(th.BasicTeam)
		constrainedChannel.GroupConstrained = model.NewBool(true)
		constrainedChannel, appErr = th.App.UpdateChannel(constrainedChannel)
		require.Nil(t, appErr)

		_, appErr = th.App.UpsertGroupSyncable(&model.GroupSyncable{
			GroupId:    group.Id,
			Type:       model.GroupSyncableTypeChannel,
			SyncableId: constrainedChannel.Id,
		})
		require.Nil(t, appErr)

		post := &model.Post{}
		potentialMentions := []string{nonChannelMember.Username, nonGroupMember.Username}

		outOfChannelUsers, outOfGroupUsers, err := th.App.filterOutOfChannelMentions(user1, post, constrainedChannel, potentialMentions)

		assert.Nil(t, err)
		assert.Len(t, outOfChannelUsers, 1)
		assert.Equal(t, nonChannelMember.Id, outOfChannelUsers[0].Id)
		assert.Len(t, outOfGroupUsers, 1)
		assert.Equal(t, nonGroupMember.Id, outOfGroupUsers[0].Id)
	})
}

func TestGetExplicitMentions(t *testing.T) {
	id1 := model.NewId()
	id2 := model.NewId()
	id3 := model.NewId()

	for name, tc := range map[string]struct {
		Message     string
		Attachments []*model.SlackAttachment
		Keywords    map[string][]string
		Expected    *ExplicitMentions
	}{
		"Nobody": {
			Message:  "this is a message",
			Keywords: map[string][]string{},
			Expected: &ExplicitMentions{},
		},
		"NonexistentUser": {
			Message: "this is a message for @user",
			Expected: &ExplicitMentions{
				OtherPotentialMentions: []string{"user"},
			},
		},
		"OnePerson": {
			Message:  "this is a message for @user",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"OnePersonWithPeriodAtEndOfUsername": {
			Message:  "this is a message for @user.name.",
			Keywords: map[string][]string{"@user.name.": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"OnePersonWithPeriodAtEndOfUsernameButNotSimilarName": {
			Message:  "this is a message for @user.name.",
			Keywords: map[string][]string{"@user.name.": {id1}, "@user.name": {id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"OnePersonAtEndOfSentence": {
			Message:  "this is a message for @user.",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"OnePersonWithoutAtMention": {
			Message:  "this is a message for @user",
			Keywords: map[string][]string{"this": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
				OtherPotentialMentions: []string{"user"},
			},
		},
		"OnePersonWithPeriodAfter": {
			Message:  "this is a message for @user.",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"OnePersonWithPeriodBefore": {
			Message:  "this is a message for .@user",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"OnePersonWithColonAfter": {
			Message:  "this is a message for @user:",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"OnePersonWithColonBefore": {
			Message:  "this is a message for :@user",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"OnePersonWithHyphenAfter": {
			Message:  "this is a message for @user.",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"OnePersonWithHyphenBefore": {
			Message:  "this is a message for -@user",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"MultiplePeopleWithOneWord": {
			Message:  "this is a message for @user",
			Keywords: map[string][]string{"@user": {id1, id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
					id2: KeywordMention,
				},
			},
		},
		"OneOfMultiplePeople": {
			Message:  "this is a message for @user",
			Keywords: map[string][]string{"@user": {id1}, "@mention": {id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"MultiplePeopleWithMultipleWords": {
			Message:  "this is an @mention for @user",
			Keywords: map[string][]string{"@user": {id1}, "@mention": {id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
					id2: KeywordMention,
				},
			},
		},
		"Channel": {
			Message:  "this is an message for @channel",
			Keywords: map[string][]string{"@channel": {id1, id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: ChannelMention,
					id2: ChannelMention,
				},
				ChannelMentioned: true,
			},
		},

		"ChannelWithColonAtEnd": {
			Message:  "this is a message for @channel:",
			Keywords: map[string][]string{"@channel": {id1, id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: ChannelMention,
					id2: ChannelMention,
				},
				ChannelMentioned: true,
			},
		},
		"CapitalizedChannel": {
			Message:  "this is an message for @cHaNNeL",
			Keywords: map[string][]string{"@channel": {id1, id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: ChannelMention,
					id2: ChannelMention,
				},
				ChannelMentioned: true,
			},
		},
		"All": {
			Message:  "this is an message for @all",
			Keywords: map[string][]string{"@all": {id1, id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: ChannelMention,
					id2: ChannelMention,
				},
				AllMentioned: true,
			},
		},
		"AllWithColonAtEnd": {
			Message:  "this is a message for @all:",
			Keywords: map[string][]string{"@all": {id1, id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: ChannelMention,
					id2: ChannelMention,
				},
				AllMentioned: true,
			},
		},
		"CapitalizedAll": {
			Message:  "this is an message for @ALL",
			Keywords: map[string][]string{"@all": {id1, id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: ChannelMention,
					id2: ChannelMention,
				},
				AllMentioned: true,
			},
		},
		"UserWithPeriod": {
			Message:  "user.period doesn't complicate things at all by including periods in their username",
			Keywords: map[string][]string{"user.period": {id1}, "user": {id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"AtUserWithColonAtEnd": {
			Message:  "this is a message for @user:",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"AtUserWithPeriodAtEndOfSentence": {
			Message:  "this is a message for @user.period.",
			Keywords: map[string][]string{"@user.period": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"UserWithPeriodAtEndOfSentence": {
			Message:  "this is a message for user.period.",
			Keywords: map[string][]string{"user.period": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"UserWithColonAtEnd": {
			Message:  "this is a message for user:",
			Keywords: map[string][]string{"user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"PotentialOutOfChannelUser": {
			Message:  "this is an message for @potential and @user",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
				OtherPotentialMentions: []string{"potential"},
			},
		},
		"PotentialOutOfChannelUserWithPeriod": {
			Message: "this is an message for @potential.user",
			Expected: &ExplicitMentions{
				OtherPotentialMentions: []string{"potential.user"},
			},
		},
		"InlineCode": {
			Message:  "`this shouldn't mention @channel at all`",
			Keywords: map[string][]string{},
			Expected: &ExplicitMentions{},
		},
		"FencedCodeBlock": {
			Message:  "```\nthis shouldn't mention @channel at all\n```",
			Keywords: map[string][]string{},
			Expected: &ExplicitMentions{},
		},
		"Emphasis": {
			Message:  "*@aaa @bbb @ccc*",
			Keywords: map[string][]string{"@aaa": {id1}, "@bbb": {id2}, "@ccc": {id3}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
					id2: KeywordMention,
					id3: KeywordMention,
				},
			},
		},
		"StrongEmphasis": {
			Message:  "**@aaa @bbb @ccc**",
			Keywords: map[string][]string{"@aaa": {id1}, "@bbb": {id2}, "@ccc": {id3}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
					id2: KeywordMention,
					id3: KeywordMention,
				},
			},
		},
		"Strikethrough": {
			Message:  "~~@aaa @bbb @ccc~~",
			Keywords: map[string][]string{"@aaa": {id1}, "@bbb": {id2}, "@ccc": {id3}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
					id2: KeywordMention,
					id3: KeywordMention,
				},
			},
		},
		"Heading": {
			Message:  "### @aaa",
			Keywords: map[string][]string{"@aaa": {id1}, "@bbb": {id2}, "@ccc": {id3}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"BlockQuote": {
			Message:  "> @aaa",
			Keywords: map[string][]string{"@aaa": {id1}, "@bbb": {id2}, "@ccc": {id3}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"Emoji": {
			Message:  ":smile:",
			Keywords: map[string][]string{"smile": {id1}, "smiley": {id2}, "smiley_cat": {id3}},
			Expected: &ExplicitMentions{},
		},
		"NotEmoji": {
			Message:  "smile",
			Keywords: map[string][]string{"smile": {id1}, "smiley": {id2}, "smiley_cat": {id3}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"UnclosedEmoji": {
			Message:  ":smile",
			Keywords: map[string][]string{"smile": {id1}, "smiley": {id2}, "smiley_cat": {id3}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"UnopenedEmoji": {
			Message:  "smile:",
			Keywords: map[string][]string{"smile": {id1}, "smiley": {id2}, "smiley_cat": {id3}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"IndentedCodeBlock": {
			Message:  "    this shouldn't mention @channel at all",
			Keywords: map[string][]string{},
			Expected: &ExplicitMentions{},
		},
		"LinkTitle": {
			Message:  `[foo](this "shouldn't mention @channel at all")`,
			Keywords: map[string][]string{},
			Expected: &ExplicitMentions{},
		},
		"MalformedInlineCode": {
			Message:  "`this should mention @channel``",
			Keywords: map[string][]string{},
			Expected: &ExplicitMentions{
				ChannelMentioned: true,
			},
		},
		"MultibyteCharacter": {
			Message:  "My name is 萌",
			Keywords: map[string][]string{"萌": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"MultibyteCharacterAtBeginningOfSentence": {
			Message:  "이메일을 보내다.",
			Keywords: map[string][]string{"이메일": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"MultibyteCharacterInPartOfSentence": {
			Message:  "我爱吃番茄炒饭",
			Keywords: map[string][]string{"番茄": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"MultibyteCharacterAtEndOfSentence": {
			Message:  "こんにちは、世界",
			Keywords: map[string][]string{"世界": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"MultibyteCharacterTwiceInSentence": {
			Message:  "石橋さんが石橋を渡る",
			Keywords: map[string][]string{"石橋": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},

		// The following tests cover cases where the message mentions @user.name, so we shouldn't assume that
		// the user might be intending to mention some @user that isn't in the channel.
		"Don't include potential mention that's part of an actual mention (without trailing period)": {
			Message:  "this is an message for @user.name",
			Keywords: map[string][]string{"@user.name": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"Don't include potential mention that's part of an actual mention (with trailing period)": {
			Message:  "this is an message for @user.name.",
			Keywords: map[string][]string{"@user.name": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"Don't include potential mention that's part of an actual mention (with multiple trailing periods)": {
			Message:  "this is an message for @user.name...",
			Keywords: map[string][]string{"@user.name": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"Don't include potential mention that's part of an actual mention (containing and followed by multiple periods)": {
			Message:  "this is an message for @user...name...",
			Keywords: map[string][]string{"@user...name": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"should include the mentions from attachment text and preText": {
			Message: "this is an message for @user1",
			Attachments: []*model.SlackAttachment{
				{
					Text:    "this is a message For @user2",
					Pretext: "this is a message for @here",
				},
			},
			Keywords: map[string][]string{"@user1": {id1}, "@user2": {id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
					id2: KeywordMention,
				},
				HereMentioned: true,
			},
		},
		"Name on keywords is a prefix of a mention": {
			Message:  "@other @test-two",
			Keywords: map[string][]string{"@test": {model.NewId()}},
			Expected: &ExplicitMentions{
				OtherPotentialMentions: []string{"other", "test-two"},
			},
		},
		"Name on mentions is a prefix of other mention": {
			Message:  "@other-one @other @other-two",
			Keywords: nil,
			Expected: &ExplicitMentions{
				OtherPotentialMentions: []string{"other-one", "other", "other-two"},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			post := &model.Post{
				Message: tc.Message,
				Props: model.StringInterface{
					"attachments": tc.Attachments,
				},
			}

			m := getExplicitMentions(post, tc.Keywords)

			assert.EqualValues(t, tc.Expected, m)
		})
	}
}

func TestGetExplicitMentionsAtHere(t *testing.T) {
	// test all the boundary cases that we know can break up terms (and those that we know won't)
	cases := map[string]bool{
		"":          false,
		"here":      false,
		"@here":     true,
		" @here ":   true,
		"\n@here\n": true,
		"!@here!":   true,
		"#@here#":   true,
		"$@here$":   true,
		"%@here%":   true,
		"^@here^":   true,
		"&@here&":   true,
		"*@here*":   true,
		"(@here(":   true,
		")@here)":   true,
		"-@here-":   true,
		"_@here_":   true,
		"=@here=":   true,
		"+@here+":   true,
		"[@here[":   true,
		"{@here{":   true,
		"]@here]":   true,
		"}@here}":   true,
		"\\@here\\": true,
		"|@here|":   true,
		";@here;":   true,
		"@here:":    true,
		":@here:":   false, // This case shouldn't trigger a mention since it follows the format of reactions e.g. :word:
		"'@here'":   true,
		"\"@here\"": true,
		",@here,":   true,
		"<@here<":   true,
		".@here.":   true,
		">@here>":   true,
		"/@here/":   true,
		"?@here?":   true,
		"`@here`":   false, // This case shouldn't mention since it's a code block
		"~@here~":   true,
		"@HERE":     true,
		"@hERe":     true,
	}

	for message, shouldMention := range cases {
		post := &model.Post{Message: message}
		m := getExplicitMentions(post, nil)
		require.False(t, m.HereMentioned && !shouldMention, "shouldn't have mentioned @here with \"%v\"")
		require.False(t, !m.HereMentioned && shouldMention, "should've mentioned @here with \"%v\"")
	}

	// mentioning @here and someone
	id := model.NewId()
	m := getExplicitMentions(&model.Post{Message: "@here @user @potential"}, map[string][]string{"@user": {id}})
	require.True(t, m.HereMentioned, "should've mentioned @here with \"@here @user\"")
	require.Len(t, m.Mentions, 1)
	require.Equal(t, KeywordMention, m.Mentions[id], "should've mentioned @user with \"@here @user\"")
	require.LessOrEqual(t, len(m.OtherPotentialMentions), 1, "should've potential mentions for @potential")
}

func TestAllowChannelMentions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	post := &model.Post{ChannelId: th.BasicChannel.Id, UserId: th.BasicUser.Id}

	t.Run("should return true for a regular post with few channel members", func(t *testing.T) {
		allowChannelMentions := th.App.allowChannelMentions(post, 5)
		assert.True(t, allowChannelMentions)
	})

	t.Run("should return false for a channel header post", func(t *testing.T) {
		headerChangePost := &model.Post{ChannelId: th.BasicChannel.Id, UserId: th.BasicUser.Id, Type: model.POST_HEADER_CHANGE}
		allowChannelMentions := th.App.allowChannelMentions(headerChangePost, 5)
		assert.False(t, allowChannelMentions)
	})

	t.Run("should return false for a channel purpose post", func(t *testing.T) {
		purposeChangePost := &model.Post{ChannelId: th.BasicChannel.Id, UserId: th.BasicUser.Id, Type: model.POST_PURPOSE_CHANGE}
		allowChannelMentions := th.App.allowChannelMentions(purposeChangePost, 5)
		assert.False(t, allowChannelMentions)
	})

	t.Run("should return false for a regular post with many channel members", func(t *testing.T) {
		allowChannelMentions := th.App.allowChannelMentions(post, int(*th.App.Config().TeamSettings.MaxNotificationsPerChannel)+1)
		assert.False(t, allowChannelMentions)
	})

	t.Run("should return false for a post where the post user does not have USE_CHANNEL_MENTIONS permission", func(t *testing.T) {
		defer th.AddPermissionToRole(model.PERMISSION_USE_CHANNEL_MENTIONS.Id, model.CHANNEL_USER_ROLE_ID)
		th.RemovePermissionFromRole(model.PERMISSION_USE_CHANNEL_MENTIONS.Id, model.CHANNEL_USER_ROLE_ID)
		allowChannelMentions := th.App.allowChannelMentions(post, 5)
		assert.False(t, allowChannelMentions)
	})
}

func TestGetMentionKeywords(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	// user with username or custom mentions enabled
	user1 := &model.User{
		Id:        model.NewId(),
		FirstName: "First",
		Username:  "User",
		NotifyProps: map[string]string{
			"mention_keys": "User,@User,MENTION",
		},
	}

	channelMemberNotifyPropsMap1Off := map[string]model.StringMap{
		user1.Id: {
			"ignore_channel_mentions": model.IGNORE_CHANNEL_MENTIONS_OFF,
		},
	}

	profiles := map[string]*model.User{user1.Id: user1}
	mentions := th.App.getMentionKeywordsInChannel(profiles, true, channelMemberNotifyPropsMap1Off)
	require.Len(t, mentions, 3, "should've returned three mention keywords")

	ids, ok := mentions["user"]
	require.True(t, ok)
	require.Equal(t, user1.Id, ids[0], "should've returned mention key of user")
	ids, ok = mentions["@user"]
	require.True(t, ok)
	require.Equal(t, user1.Id, ids[0], "should've returned mention key of @user")
	ids, ok = mentions["mention"]
	require.True(t, ok)
	require.Equal(t, user1.Id, ids[0], "should've returned mention key of mention")

	// user with first name mention enabled
	user2 := &model.User{
		Id:        model.NewId(),
		FirstName: "First",
		Username:  "User",
		NotifyProps: map[string]string{
			"first_name": "true",
		},
	}

	channelMemberNotifyPropsMap2Off := map[string]model.StringMap{
		user2.Id: {
			"ignore_channel_mentions": model.IGNORE_CHANNEL_MENTIONS_OFF,
		},
	}

	profiles = map[string]*model.User{user2.Id: user2}
	mentions = th.App.getMentionKeywordsInChannel(profiles, true, channelMemberNotifyPropsMap2Off)
	require.Len(t, mentions, 2, "should've returned two mention keyword")

	ids, ok = mentions["First"]
	require.True(t, ok)
	require.Equal(t, user2.Id, ids[0], "should've returned mention key of First")

	// user with @channel/@all mentions enabled
	user3 := &model.User{
		Id:        model.NewId(),
		FirstName: "First",
		Username:  "User",
		NotifyProps: map[string]string{
			"channel": "true",
		},
	}

	// Channel-wide mentions are not ignored on channel level
	channelMemberNotifyPropsMap3Off := map[string]model.StringMap{
		user3.Id: {
			"ignore_channel_mentions": model.IGNORE_CHANNEL_MENTIONS_OFF,
		},
	}
	profiles = map[string]*model.User{user3.Id: user3}
	mentions = th.App.getMentionKeywordsInChannel(profiles, true, channelMemberNotifyPropsMap3Off)
	require.Len(t, mentions, 3, "should've returned three mention keywords")
	ids, ok = mentions["@channel"]
	require.True(t, ok)
	require.Equal(t, user3.Id, ids[0], "should've returned mention key of @channel")
	ids, ok = mentions["@all"]
	require.True(t, ok)
	require.Equal(t, user3.Id, ids[0], "should've returned mention key of @all")

	// Channel member notify props is set to default
	channelMemberNotifyPropsMapDefault := map[string]model.StringMap{
		user3.Id: {
			"ignore_channel_mentions": model.IGNORE_CHANNEL_MENTIONS_DEFAULT,
		},
	}
	profiles = map[string]*model.User{user3.Id: user3}
	mentions = th.App.getMentionKeywordsInChannel(profiles, true, channelMemberNotifyPropsMapDefault)
	require.Len(t, mentions, 3, "should've returned three mention keywords")
	ids, ok = mentions["@channel"]
	require.True(t, ok)
	require.Equal(t, user3.Id, ids[0], "should've returned mention key of @channel")
	ids, ok = mentions["@all"]
	require.True(t, ok)
	require.Equal(t, user3.Id, ids[0], "should've returned mention key of @all")

	// Channel member notify props is empty
	channelMemberNotifyPropsMapEmpty := map[string]model.StringMap{}
	profiles = map[string]*model.User{user3.Id: user3}
	mentions = th.App.getMentionKeywordsInChannel(profiles, true, channelMemberNotifyPropsMapEmpty)
	require.Len(t, mentions, 3, "should've returned three mention keywords")
	ids, ok = mentions["@channel"]
	require.True(t, ok)
	require.Equal(t, user3.Id, ids[0], "should've returned mention key of @channel")
	ids, ok = mentions["@all"]
	require.True(t, ok)
	require.Equal(t, user3.Id, ids[0], "should've returned mention key of @all")

	// Channel-wide mentions are ignored channel level
	channelMemberNotifyPropsMap3On := map[string]model.StringMap{
		user3.Id: {
			"ignore_channel_mentions": model.IGNORE_CHANNEL_MENTIONS_ON,
		},
	}
	mentions = th.App.getMentionKeywordsInChannel(profiles, true, channelMemberNotifyPropsMap3On)
	require.NotEmpty(t, mentions, "should've not returned any keywords")

	// user with all types of mentions enabled
	user4 := &model.User{
		Id:        model.NewId(),
		FirstName: "First",
		Username:  "User",
		NotifyProps: map[string]string{
			"mention_keys": "User,@User,MENTION",
			"first_name":   "true",
			"channel":      "true",
		},
	}

	// Channel-wide mentions are not ignored on channel level
	channelMemberNotifyPropsMap4Off := map[string]model.StringMap{
		user4.Id: {
			"ignore_channel_mentions": model.IGNORE_CHANNEL_MENTIONS_OFF,
		},
	}

	profiles = map[string]*model.User{user4.Id: user4}
	mentions = th.App.getMentionKeywordsInChannel(profiles, true, channelMemberNotifyPropsMap4Off)
	require.Len(t, mentions, 6, "should've returned six mention keywords")
	ids, ok = mentions["user"]
	require.True(t, ok)
	require.Equal(t, user4.Id, ids[0], "should've returned mention key of user")
	ids, ok = mentions["@user"]
	require.True(t, ok)
	require.Equal(t, user4.Id, ids[0], "should've returned mention key of @user")
	ids, ok = mentions["mention"]
	require.True(t, ok)
	require.Equal(t, user4.Id, ids[0], "should've returned mention key of mention")
	ids, ok = mentions["First"]
	require.True(t, ok)
	require.Equal(t, user4.Id, ids[0], "should've returned mention key of First")
	ids, ok = mentions["@channel"]
	require.True(t, ok)
	require.Equal(t, user4.Id, ids[0], "should've returned mention key of @channel")
	ids, ok = mentions["@all"]
	require.True(t, ok)
	require.Equal(t, user4.Id, ids[0], "should've returned mention key of @all")

	// Channel-wide mentions are ignored on channel level
	channelMemberNotifyPropsMap4On := map[string]model.StringMap{
		user4.Id: {
			"ignore_channel_mentions": model.IGNORE_CHANNEL_MENTIONS_ON,
		},
	}
	mentions = th.App.getMentionKeywordsInChannel(profiles, true, channelMemberNotifyPropsMap4On)
	require.Len(t, mentions, 4, "should've returned four mention keywords")
	ids, ok = mentions["user"]
	require.True(t, ok)
	require.Equal(t, user4.Id, ids[0], "should've returned mention key of user")
	ids, ok = mentions["@user"]
	require.True(t, ok)
	require.Equal(t, user4.Id, ids[0], "should've returned mention key of @user")
	ids, ok = mentions["mention"]
	require.True(t, ok)
	require.Equal(t, user4.Id, ids[0], "should've returned mention key of mention")
	ids, ok = mentions["First"]
	require.True(t, ok)
	require.Equal(t, user4.Id, ids[0], "should've returned mention key of First")
	dup_count := func(list []string) map[string]int {

		duplicate_frequency := make(map[string]int)

		for _, item := range list {
			// check if the item/element exist in the duplicate_frequency map

			_, exist := duplicate_frequency[item]

			if exist {
				duplicate_frequency[item] += 1 // increase counter by 1 if already in the map
			} else {
				duplicate_frequency[item] = 1 // else start counting from 1
			}
		}
		return duplicate_frequency
	}

	// multiple users but no more than MaxNotificationsPerChannel
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.MaxNotificationsPerChannel = 4 })
	profiles = map[string]*model.User{
		user1.Id: user1,
		user2.Id: user2,
		user3.Id: user3,
		user4.Id: user4,
	}
	// Channel-wide mentions are not ignored on channel level for all users
	channelMemberNotifyPropsMap5Off := map[string]model.StringMap{
		user1.Id: {
			"ignore_channel_mentions": model.IGNORE_CHANNEL_MENTIONS_OFF,
		},
		user2.Id: {
			"ignore_channel_mentions": model.IGNORE_CHANNEL_MENTIONS_OFF,
		},
		user3.Id: {
			"ignore_channel_mentions": model.IGNORE_CHANNEL_MENTIONS_OFF,
		},
		user4.Id: {
			"ignore_channel_mentions": model.IGNORE_CHANNEL_MENTIONS_OFF,
		},
	}
	mentions = th.App.getMentionKeywordsInChannel(profiles, true, channelMemberNotifyPropsMap5Off)
	require.Len(t, mentions, 6, "should've returned six mention keywords")
	ids, ok = mentions["user"]
	require.True(t, ok)
	require.Len(t, ids, 2)
	require.False(t, ids[0] != user1.Id && ids[1] != user1.Id, "should've mentioned user1  with user")
	require.False(t, ids[0] != user4.Id && ids[1] != user4.Id, "should've mentioned user4  with user")
	idsMap := dup_count(mentions["@user"])
	require.True(t, ok)
	require.Len(t, idsMap, 4)
	require.Equal(t, idsMap[user1.Id], 2, "should've mentioned user1 with @user")
	require.Equal(t, idsMap[user4.Id], 2, "should've mentioned user4 with @user")

	ids, ok = mentions["mention"]
	require.True(t, ok)
	require.Len(t, ids, 2)
	require.False(t, ids[0] != user1.Id && ids[1] != user1.Id, "should've mentioned user1 with mention")
	require.False(t, ids[0] != user4.Id && ids[1] != user4.Id, "should've mentioned user4 with mention")
	ids, ok = mentions["First"]
	require.True(t, ok)
	require.Len(t, ids, 2)
	require.False(t, ids[0] != user2.Id && ids[1] != user2.Id, "should've mentioned user2 with First")
	require.False(t, ids[0] != user4.Id && ids[1] != user4.Id, "should've mentioned user4 with First")
	ids, ok = mentions["@channel"]
	require.True(t, ok)
	require.Len(t, ids, 2)
	require.False(t, ids[0] != user3.Id && ids[1] != user3.Id, "should've mentioned user3 with @channel")
	require.False(t, ids[0] != user4.Id && ids[1] != user4.Id, "should've mentioned user4 with @channel")
	ids, ok = mentions["@all"]
	require.True(t, ok)
	require.Len(t, ids, 2)
	require.False(t, ids[0] != user3.Id && ids[1] != user3.Id, "should've mentioned user3 with @all")
	require.False(t, ids[0] != user4.Id && ids[1] != user4.Id, "should've mentioned user4 with @all")

	// multiple users and more than MaxNotificationsPerChannel
	mentions = th.App.getMentionKeywordsInChannel(profiles, false, channelMemberNotifyPropsMap4Off)
	require.Len(t, mentions, 4, "should've returned four mention keywords")
	_, ok = mentions["@channel"]
	require.False(t, ok, "should not have mentioned any user with @channel")
	_, ok = mentions["@all"]
	require.False(t, ok, "should not have mentioned any user with @all")
	_, ok = mentions["@here"]
	require.False(t, ok, "should not have mentioned any user with @here")
	// no special mentions
	profiles = map[string]*model.User{
		user1.Id: user1,
	}
	mentions = th.App.getMentionKeywordsInChannel(profiles, false, channelMemberNotifyPropsMap4Off)
	require.Len(t, mentions, 3, "should've returned three mention keywords")
	ids, ok = mentions["user"]
	require.True(t, ok)
	require.Len(t, ids, 1)
	require.Equal(t, user1.Id, ids[0], "should've mentioned user1 with user")
	ids, ok = mentions["@user"]

	require.True(t, ok)
	require.Len(t, ids, 2)
	require.Equal(t, user1.Id, ids[0], "should've mentioned user1 twice with @user")
	require.Equal(t, user1.Id, ids[1], "should've mentioned user1 twice with @user")

	ids, ok = mentions["mention"]
	require.True(t, ok)
	require.Len(t, ids, 1)
	require.Equal(t, user1.Id, ids[0], "should've mentioned user1 with user")

	_, ok = mentions["First"]
	require.False(t, ok, "should not have mentioned user1 with First")
	_, ok = mentions["@channel"]
	require.False(t, ok, "should not have mentioned any user with @channel")
	_, ok = mentions["@all"]
	require.False(t, ok, "should not have mentioned any user with @all")
	_, ok = mentions["@here"]
	require.False(t, ok, "should not have mentioned any user with @here")

	// user with empty mention keys
	userNoMentionKeys := &model.User{
		Id:        model.NewId(),
		FirstName: "First",
		Username:  "User",
		NotifyProps: map[string]string{
			"mention_keys": ",",
		},
	}

	channelMemberNotifyPropsMapEmptyOff := map[string]model.StringMap{
		userNoMentionKeys.Id: {
			"ignore_channel_mentions": model.IGNORE_CHANNEL_MENTIONS_OFF,
		},
	}

	profiles = map[string]*model.User{userNoMentionKeys.Id: userNoMentionKeys}
	mentions = th.App.getMentionKeywordsInChannel(profiles, true, channelMemberNotifyPropsMapEmptyOff)
	assert.Equal(t, 1, len(mentions), "should've returned one metion keyword")
	ids, ok = mentions["@user"]
	assert.True(t, ok)
	assert.Equal(t, userNoMentionKeys.Id, ids[0], "should've returned mention key of @user")
}

func TestAddMentionKeywordsForUser(t *testing.T) {
	t.Run("should add @user", func(t *testing.T) {
		user := &model.User{
			Id:       model.NewId(),
			Username: "user",
		}
		channelNotifyProps := map[string]string{}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user, channelNotifyProps, nil, false)

		assert.Contains(t, keywords["@user"], user.Id)
	})

	t.Run("should add custom mention keywords", func(t *testing.T) {
		user := &model.User{
			Id:       model.NewId(),
			Username: "user",
			NotifyProps: map[string]string{
				model.MENTION_KEYS_NOTIFY_PROP: "apple,BANANA,OrAnGe",
			},
		}
		channelNotifyProps := map[string]string{}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user, channelNotifyProps, nil, false)

		assert.Contains(t, keywords["apple"], user.Id)
		assert.Contains(t, keywords["banana"], user.Id)
		assert.Contains(t, keywords["orange"], user.Id)
	})

	t.Run("should not add empty custom keywords", func(t *testing.T) {
		user := &model.User{
			Id:       model.NewId(),
			Username: "user",
			NotifyProps: map[string]string{
				model.MENTION_KEYS_NOTIFY_PROP: ",,",
			},
		}
		channelNotifyProps := map[string]string{}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user, channelNotifyProps, nil, false)

		assert.Nil(t, keywords[""])
	})

	t.Run("should add case sensitive first name if enabled", func(t *testing.T) {
		user := &model.User{
			Id:        model.NewId(),
			Username:  "user",
			FirstName: "William",
			LastName:  "Robert",
			NotifyProps: map[string]string{
				model.FIRST_NAME_NOTIFY_PROP: "true",
			},
		}
		channelNotifyProps := map[string]string{}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user, channelNotifyProps, nil, false)

		assert.Contains(t, keywords["William"], user.Id)
		assert.NotContains(t, keywords["william"], user.Id)
		assert.NotContains(t, keywords["Robert"], user.Id)
	})

	t.Run("should not add case sensitive first name if disabled", func(t *testing.T) {
		user := &model.User{
			Id:        model.NewId(),
			Username:  "user",
			FirstName: "William",
			LastName:  "Robert",
			NotifyProps: map[string]string{
				model.FIRST_NAME_NOTIFY_PROP: "false",
			},
		}
		channelNotifyProps := map[string]string{}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user, channelNotifyProps, nil, false)

		assert.NotContains(t, keywords["William"], user.Id)
		assert.NotContains(t, keywords["william"], user.Id)
		assert.NotContains(t, keywords["Robert"], user.Id)
	})

	t.Run("should add @channel/@all/@here when allowed", func(t *testing.T) {
		user := &model.User{
			Id:       model.NewId(),
			Username: "user",
			NotifyProps: map[string]string{
				model.CHANNEL_MENTIONS_NOTIFY_PROP: "true",
			},
		}
		channelNotifyProps := map[string]string{}
		status := &model.Status{
			Status: model.STATUS_ONLINE,
		}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user, channelNotifyProps, status, true)

		assert.Contains(t, keywords["@channel"], user.Id)
		assert.Contains(t, keywords["@all"], user.Id)
		assert.Contains(t, keywords["@here"], user.Id)
	})

	t.Run("should not add @channel/@all/@here when not allowed", func(t *testing.T) {
		user := &model.User{
			Id:       model.NewId(),
			Username: "user",
			NotifyProps: map[string]string{
				model.CHANNEL_MENTIONS_NOTIFY_PROP: "true",
			},
		}
		channelNotifyProps := map[string]string{}
		status := &model.Status{
			Status: model.STATUS_ONLINE,
		}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user, channelNotifyProps, status, false)

		assert.NotContains(t, keywords["@channel"], user.Id)
		assert.NotContains(t, keywords["@all"], user.Id)
		assert.NotContains(t, keywords["@here"], user.Id)
	})

	t.Run("should not add @channel/@all/@here when disabled for user", func(t *testing.T) {
		user := &model.User{
			Id:       model.NewId(),
			Username: "user",
			NotifyProps: map[string]string{
				model.CHANNEL_MENTIONS_NOTIFY_PROP: "false",
			},
		}
		channelNotifyProps := map[string]string{}
		status := &model.Status{
			Status: model.STATUS_ONLINE,
		}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user, channelNotifyProps, status, true)

		assert.NotContains(t, keywords["@channel"], user.Id)
		assert.NotContains(t, keywords["@all"], user.Id)
		assert.NotContains(t, keywords["@here"], user.Id)
	})

	t.Run("should not add @channel/@all/@here when disabled for channel", func(t *testing.T) {
		user := &model.User{
			Id:       model.NewId(),
			Username: "user",
			NotifyProps: map[string]string{
				model.CHANNEL_MENTIONS_NOTIFY_PROP: "true",
			},
		}
		channelNotifyProps := map[string]string{
			model.IGNORE_CHANNEL_MENTIONS_NOTIFY_PROP: model.IGNORE_CHANNEL_MENTIONS_ON,
		}
		status := &model.Status{
			Status: model.STATUS_ONLINE,
		}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user, channelNotifyProps, status, true)

		assert.NotContains(t, keywords["@channel"], user.Id)
		assert.NotContains(t, keywords["@all"], user.Id)
		assert.NotContains(t, keywords["@here"], user.Id)
	})

	t.Run("should not add @here when when user is not online", func(t *testing.T) {
		user := &model.User{
			Id:       model.NewId(),
			Username: "user",
			NotifyProps: map[string]string{
				model.CHANNEL_MENTIONS_NOTIFY_PROP: "true",
			},
		}
		channelNotifyProps := map[string]string{}
		status := &model.Status{
			Status: model.STATUS_AWAY,
		}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user, channelNotifyProps, status, true)

		assert.Contains(t, keywords["@channel"], user.Id)
		assert.Contains(t, keywords["@all"], user.Id)
		assert.NotContains(t, keywords["@here"], user.Id)
	})

	t.Run("should add for multiple users", func(t *testing.T) {
		user1 := &model.User{
			Id:       model.NewId(),
			Username: "user1",
			NotifyProps: map[string]string{
				model.CHANNEL_MENTIONS_NOTIFY_PROP: "true",
			},
		}
		user2 := &model.User{
			Id:       model.NewId(),
			Username: "user2",
			NotifyProps: map[string]string{
				model.CHANNEL_MENTIONS_NOTIFY_PROP: "true",
			},
		}

		keywords := map[string][]string{}
		addMentionKeywordsForUser(keywords, user1, map[string]string{}, nil, true)
		addMentionKeywordsForUser(keywords, user2, map[string]string{}, nil, true)

		assert.Contains(t, keywords["@user1"], user1.Id)
		assert.Contains(t, keywords["@user2"], user2.Id)
		assert.Contains(t, keywords["@all"], user1.Id)
		assert.Contains(t, keywords["@all"], user2.Id)
	})
}

func TestGetMentionsEnabledFields(t *testing.T) {

	attachmentWithTextAndPreText := model.SlackAttachment{
		Text:    "@here with mentions",
		Pretext: "@Channel some comment for the channel",
	}

	attachmentWithOutPreText := model.SlackAttachment{
		Text: "some text",
	}
	attachments := []*model.SlackAttachment{
		&attachmentWithTextAndPreText,
		&attachmentWithOutPreText,
	}

	post := &model.Post{
		Message: "This is the message",
		Props: model.StringInterface{
			"attachments": attachments,
		},
	}
	expectedFields := []string{
		"This is the message",
		"@Channel some comment for the channel",
		"@here with mentions",
		"some text"}

	mentionEnabledFields := getMentionsEnabledFields(post)

	assert.EqualValues(t, 4, len(mentionEnabledFields))
	assert.EqualValues(t, expectedFields, mentionEnabledFields)
}

func TestPostNotificationGetChannelName(t *testing.T) {
	sender := &model.User{Id: model.NewId(), Username: "sender", FirstName: "Sender", LastName: "Sender", Nickname: "Sender"}
	recipient := &model.User{Id: model.NewId(), Username: "recipient", FirstName: "Recipient", LastName: "Recipient", Nickname: "Recipient"}
	otherUser := &model.User{Id: model.NewId(), Username: "other", FirstName: "Other", LastName: "Other", Nickname: "Other"}
	profileMap := map[string]*model.User{
		sender.Id:    sender,
		recipient.Id: recipient,
		otherUser.Id: otherUser,
	}

	for name, testCase := range map[string]struct {
		channel     *model.Channel
		nameFormat  string
		recipientId string
		expected    string
	}{
		"regular channel": {
			channel:  &model.Channel{Type: model.CHANNEL_OPEN, Name: "channel", DisplayName: "My Channel"},
			expected: "My Channel",
		},
		"direct channel, unspecified": {
			channel:  &model.Channel{Type: model.CHANNEL_DIRECT},
			expected: "@sender",
		},
		"direct channel, username": {
			channel:    &model.Channel{Type: model.CHANNEL_DIRECT},
			nameFormat: model.SHOW_USERNAME,
			expected:   "@sender",
		},
		"direct channel, full name": {
			channel:    &model.Channel{Type: model.CHANNEL_DIRECT},
			nameFormat: model.SHOW_FULLNAME,
			expected:   "Sender Sender",
		},
		"direct channel, nickname": {
			channel:    &model.Channel{Type: model.CHANNEL_DIRECT},
			nameFormat: model.SHOW_NICKNAME_FULLNAME,
			expected:   "Sender",
		},
		"group channel, unspecified": {
			channel:  &model.Channel{Type: model.CHANNEL_GROUP},
			expected: "other, sender",
		},
		"group channel, username": {
			channel:    &model.Channel{Type: model.CHANNEL_GROUP},
			nameFormat: model.SHOW_USERNAME,
			expected:   "other, sender",
		},
		"group channel, full name": {
			channel:    &model.Channel{Type: model.CHANNEL_GROUP},
			nameFormat: model.SHOW_FULLNAME,
			expected:   "Other Other, Sender Sender",
		},
		"group channel, nickname": {
			channel:    &model.Channel{Type: model.CHANNEL_GROUP},
			nameFormat: model.SHOW_NICKNAME_FULLNAME,
			expected:   "Other, Sender",
		},
		"group channel, not excluding current user": {
			channel:     &model.Channel{Type: model.CHANNEL_GROUP},
			nameFormat:  model.SHOW_NICKNAME_FULLNAME,
			expected:    "Other, Sender",
			recipientId: "",
		},
	} {
		t.Run(name, func(t *testing.T) {
			notification := &PostNotification{
				Channel:    testCase.channel,
				Sender:     sender,
				ProfileMap: profileMap,
			}

			recipientId := recipient.Id
			if testCase.recipientId != "" {
				recipientId = testCase.recipientId
			}

			assert.Equal(t, testCase.expected, notification.GetChannelName(testCase.nameFormat, recipientId))
		})
	}
}

func TestPostNotificationGetSenderName(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	defaultChannel := &model.Channel{Type: model.CHANNEL_OPEN}
	defaultPost := &model.Post{Props: model.StringInterface{}}
	sender := &model.User{Id: model.NewId(), Username: "sender", FirstName: "Sender", LastName: "Sender", Nickname: "Sender"}

	overriddenPost := &model.Post{
		Props: model.StringInterface{
			"override_username": "Overridden",
			"from_webhook":      "true",
		},
	}

	for name, testCase := range map[string]struct {
		channel        *model.Channel
		post           *model.Post
		nameFormat     string
		allowOverrides bool
		expected       string
	}{
		"name format unspecified": {
			expected: "@" + sender.Username,
		},
		"name format username": {
			nameFormat: model.SHOW_USERNAME,
			expected:   "@" + sender.Username,
		},
		"name format full name": {
			nameFormat: model.SHOW_FULLNAME,
			expected:   sender.FirstName + " " + sender.LastName,
		},
		"name format nickname": {
			nameFormat: model.SHOW_NICKNAME_FULLNAME,
			expected:   sender.Nickname,
		},
		"system message": {
			post:     &model.Post{Type: model.POST_SYSTEM_MESSAGE_PREFIX + "custom"},
			expected: utils.T("system.message.name"),
		},
		"overridden username": {
			post:           overriddenPost,
			allowOverrides: true,
			expected:       overriddenPost.GetProp("override_username").(string),
		},
		"overridden username, direct channel": {
			channel:        &model.Channel{Type: model.CHANNEL_DIRECT},
			post:           overriddenPost,
			allowOverrides: true,
			expected:       "@" + sender.Username,
		},
		"overridden username, overrides disabled": {
			post:           overriddenPost,
			allowOverrides: false,
			expected:       "@" + sender.Username,
		},
	} {
		t.Run(name, func(t *testing.T) {
			channel := defaultChannel
			if testCase.channel != nil {
				channel = testCase.channel
			}

			post := defaultPost
			if testCase.post != nil {
				post = testCase.post
			}

			notification := &PostNotification{
				Channel: channel,
				Post:    post,
				Sender:  sender,
			}

			assert.Equal(t, testCase.expected, notification.GetSenderName(testCase.nameFormat, testCase.allowOverrides))
		})
	}
}

func TestIsKeywordMultibyte(t *testing.T) {
	id1 := model.NewId()

	for name, tc := range map[string]struct {
		Message     string
		Attachments []*model.SlackAttachment
		Keywords    map[string][]string
		Expected    *ExplicitMentions
	}{
		"MultibyteCharacter": {
			Message:  "My name is 萌",
			Keywords: map[string][]string{"萌": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"MultibyteCharacterWithNoUser": {
			Message:  "My name is 萌",
			Keywords: map[string][]string{"萌": {}},
			Expected: &ExplicitMentions{
				Mentions: nil,
			},
		},
		"MultibyteCharacterAtBeginningOfSentence": {
			Message:  "이메일을 보내다.",
			Keywords: map[string][]string{"이메일": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"MultibyteCharacterAtBeginningOfSentenceWithNoUser": {
			Message:  "이메일을 보내다.",
			Keywords: map[string][]string{"이메일": {}},
			Expected: &ExplicitMentions{
				Mentions: nil,
			},
		},
		"MultibyteCharacterInPartOfSentence": {
			Message:  "我爱吃番茄炒饭",
			Keywords: map[string][]string{"番茄": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"MultibyteCharacterInPartOfSentenceWithNoUser": {
			Message:  "我爱吃番茄炒饭",
			Keywords: map[string][]string{"番茄": {}},
			Expected: &ExplicitMentions{
				Mentions: nil,
			},
		},
		"MultibyteCharacterAtEndOfSentence": {
			Message:  "こんにちは、世界",
			Keywords: map[string][]string{"世界": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"MultibyteCharacterAtEndOfSentenceWithNoUser": {
			Message:  "こんにちは、世界",
			Keywords: map[string][]string{"世界": {}},
			Expected: &ExplicitMentions{
				Mentions: nil,
			},
		},
		"MultibyteCharacterTwiceInSentence": {
			Message:  "石橋さんが石橋を渡る",
			Keywords: map[string][]string{"石橋": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"MultibyteCharacterTwiceInSentenceWithNoUser": {
			Message:  "石橋さんが石橋を渡る",
			Keywords: map[string][]string{"石橋": {}},
			Expected: &ExplicitMentions{
				Mentions: nil,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			post := &model.Post{
				Message: tc.Message,
				Props: model.StringInterface{
					"attachments": tc.Attachments,
				},
			}

			m := getExplicitMentions(post, tc.Keywords)
			// if tc.Expected.MentionedUserIds == nil {
			// 	tc.Expected.MentionedUserIds = make(map[string]bool)
			// }
			assert.EqualValues(t, tc.Expected, m)
		})
	}
}

func TestAddMention(t *testing.T) {
	t.Run("should initialize Mentions and store new mentions", func(t *testing.T) {
		m := &ExplicitMentions{}

		userId1 := model.NewId()
		userId2 := model.NewId()

		m.addMention(userId1, KeywordMention)
		m.addMention(userId2, CommentMention)

		assert.Equal(t, map[string]MentionType{
			userId1: KeywordMention,
			userId2: CommentMention,
		}, m.Mentions)
	})

	t.Run("should replace existing mentions with higher priority ones", func(t *testing.T) {
		m := &ExplicitMentions{}

		userId1 := model.NewId()
		userId2 := model.NewId()

		m.addMention(userId1, ThreadMention)
		m.addMention(userId2, DMMention)

		m.addMention(userId1, ChannelMention)
		m.addMention(userId2, KeywordMention)

		assert.Equal(t, map[string]MentionType{
			userId1: ChannelMention,
			userId2: KeywordMention,
		}, m.Mentions)
	})

	t.Run("should not replace high priority mentions with low priority ones", func(t *testing.T) {
		m := &ExplicitMentions{}

		userId1 := model.NewId()
		userId2 := model.NewId()

		m.addMention(userId1, KeywordMention)
		m.addMention(userId2, CommentMention)

		m.addMention(userId1, DMMention)
		m.addMention(userId2, ThreadMention)

		assert.Equal(t, map[string]MentionType{
			userId1: KeywordMention,
			userId2: CommentMention,
		}, m.Mentions)
	})
}

func TestCheckForMentionUsers(t *testing.T) {
	id1 := model.NewId()
	id2 := model.NewId()

	for name, tc := range map[string]struct {
		Word        string
		Attachments []*model.SlackAttachment
		Keywords    map[string][]string
		Expected    *ExplicitMentions
	}{
		"Nobody": {
			Word:     "nothing",
			Keywords: map[string][]string{},
			Expected: &ExplicitMentions{},
		},
		"UppercaseUser1": {
			Word:     "@User",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"LowercaseUser1": {
			Word:     "@user",
			Keywords: map[string][]string{"@user": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"LowercaseUser2": {
			Word:     "@user2",
			Keywords: map[string][]string{"@user2": {id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id2: KeywordMention,
				},
			},
		},
		"UppercaseUser2": {
			Word:     "@UsEr2",
			Keywords: map[string][]string{"@user2": {id2}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id2: KeywordMention,
				},
			},
		},
		"HereMention": {
			Word: "@here",
			Expected: &ExplicitMentions{
				HereMentioned: true,
			},
		},
		"ChannelMention": {
			Word: "@channel",
			Expected: &ExplicitMentions{
				ChannelMentioned: true,
			},
		},
		"AllMention": {
			Word: "@all",
			Expected: &ExplicitMentions{
				AllMentioned: true,
			},
		},
		"UppercaseHere": {
			Word: "@HeRe",
			Expected: &ExplicitMentions{
				HereMentioned: true,
			},
		},
		"UppercaseChannel": {
			Word: "@ChaNNel",
			Expected: &ExplicitMentions{
				ChannelMentioned: true,
			},
		},
		"UppercaseAll": {
			Word: "@ALL",
			Expected: &ExplicitMentions{
				AllMentioned: true,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {

			e := &ExplicitMentions{}
			e.checkForMention(tc.Word, tc.Keywords)

			assert.EqualValues(t, tc.Expected, e)
		})
	}
}
func TestProcessText(t *testing.T) {
	id1 := model.NewId()

	for name, tc := range map[string]struct {
		Text     string
		Keywords map[string][]string
		Expected *ExplicitMentions
	}{
		"Mention user in text": {
			Text:     "hello user @user1",
			Keywords: map[string][]string{"@user1": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"Mention user after ending a sentence with full stop": {
			Text:     "hello user.@user1",
			Keywords: map[string][]string{"@user1": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"Mention user after hyphen": {
			Text:     "hello user-@user1",
			Keywords: map[string][]string{"@user1": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"Mention user after colon": {
			Text:     "hello user:@user1",
			Keywords: map[string][]string{"@user1": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
			},
		},
		"Mention here after colon": {
			Text:     "hello all:@here",
			Keywords: map[string][]string{},
			Expected: &ExplicitMentions{
				HereMentioned: true,
			},
		},
		"Mention all after hyphen": {
			Text:     "hello all-@all",
			Keywords: map[string][]string{},
			Expected: &ExplicitMentions{
				AllMentioned: true,
			},
		},
		"Mention channel after full stop": {
			Text:     "hello channel.@channel",
			Keywords: map[string][]string{},
			Expected: &ExplicitMentions{
				ChannelMentioned: true,
			},
		},
		"Mention other pontential users or system calls": {
			Text:     "hello @potentialuser and @otherpotentialuser",
			Keywords: map[string][]string{},
			Expected: &ExplicitMentions{
				OtherPotentialMentions: []string{"potentialuser", "otherpotentialuser"},
			},
		},
		"Mention a real user and another potential user": {
			Text:     "@user1, you can use @systembot to get help",
			Keywords: map[string][]string{"@user1": {id1}},
			Expected: &ExplicitMentions{
				Mentions: map[string]MentionType{
					id1: KeywordMention,
				},
				OtherPotentialMentions: []string{"systembot"},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			e := &ExplicitMentions{}
			e.processText(tc.Text, tc.Keywords)

			assert.EqualValues(t, tc.Expected, e)
		})
	}
}

func TestGetNotificationNameFormat(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("show full name on", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.PrivacySettings.ShowFullName = true
			*cfg.TeamSettings.TeammateNameDisplay = model.SHOW_FULLNAME
		})

		assert.Equal(t, model.SHOW_FULLNAME, th.App.GetNotificationNameFormat(th.BasicUser))
	})

	t.Run("show full name off", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.PrivacySettings.ShowFullName = false
			*cfg.TeamSettings.TeammateNameDisplay = model.SHOW_FULLNAME
		})

		assert.Equal(t, model.SHOW_USERNAME, th.App.GetNotificationNameFormat(th.BasicUser))
	})
}

func TestUserAllowsEmail(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("should return true", func(t *testing.T) {
		user := th.CreateUser()

		th.App.SetStatusOffline(user.Id, true)

		channelMemberNotificationProps := model.StringMap{
			model.EMAIL_NOTIFY_PROP:       model.CHANNEL_NOTIFY_DEFAULT,
			model.MARK_UNREAD_NOTIFY_PROP: model.CHANNEL_MARK_UNREAD_ALL,
		}

		assert.True(t, th.App.userAllowsEmail(user, channelMemberNotificationProps, &model.Post{Type: "some-post-type"}))
	})

	t.Run("should return false in case the status is ONLINE", func(t *testing.T) {
		user := th.CreateUser()

		th.App.SetStatusOnline(user.Id, true)

		channelMemberNotificationProps := model.StringMap{
			model.EMAIL_NOTIFY_PROP:       model.CHANNEL_NOTIFY_DEFAULT,
			model.MARK_UNREAD_NOTIFY_PROP: model.CHANNEL_MARK_UNREAD_ALL,
		}

		assert.False(t, th.App.userAllowsEmail(user, channelMemberNotificationProps, &model.Post{Type: "some-post-type"}))
	})

	t.Run("should return false in case the EMAIL_NOTIFY_PROP is false", func(t *testing.T) {
		user := th.CreateUser()

		th.App.SetStatusOffline(user.Id, true)

		channelMemberNotificationProps := model.StringMap{
			model.EMAIL_NOTIFY_PROP:       "false",
			model.MARK_UNREAD_NOTIFY_PROP: model.CHANNEL_MARK_UNREAD_ALL,
		}

		assert.False(t, th.App.userAllowsEmail(user, channelMemberNotificationProps, &model.Post{Type: "some-post-type"}))
	})

	t.Run("should return false in case the MARK_UNREAD_NOTIFY_PROP is CHANNEL_MARK_UNREAD_MENTION", func(t *testing.T) {
		user := th.CreateUser()

		th.App.SetStatusOffline(user.Id, true)

		channelMemberNotificationProps := model.StringMap{
			model.EMAIL_NOTIFY_PROP:       model.CHANNEL_NOTIFY_DEFAULT,
			model.MARK_UNREAD_NOTIFY_PROP: model.CHANNEL_MARK_UNREAD_MENTION,
		}

		assert.False(t, th.App.userAllowsEmail(user, channelMemberNotificationProps, &model.Post{Type: "some-post-type"}))
	})

	t.Run("should return false in case the Post type is POST_AUTO_RESPONDER", func(t *testing.T) {
		user := th.CreateUser()

		th.App.SetStatusOffline(user.Id, true)

		channelMemberNotificationProps := model.StringMap{
			model.EMAIL_NOTIFY_PROP:       model.CHANNEL_NOTIFY_DEFAULT,
			model.MARK_UNREAD_NOTIFY_PROP: model.CHANNEL_MARK_UNREAD_ALL,
		}

		assert.False(t, th.App.userAllowsEmail(user, channelMemberNotificationProps, &model.Post{Type: model.POST_AUTO_RESPONDER}))
	})

	t.Run("should return false in case the status is STATUS_OUT_OF_OFFICE", func(t *testing.T) {
		user := th.CreateUser()

		th.App.SetStatusOutOfOffice(user.Id)

		channelMemberNotificationProps := model.StringMap{
			model.EMAIL_NOTIFY_PROP:       model.CHANNEL_NOTIFY_DEFAULT,
			model.MARK_UNREAD_NOTIFY_PROP: model.CHANNEL_MARK_UNREAD_ALL,
		}

		assert.False(t, th.App.userAllowsEmail(user, channelMemberNotificationProps, &model.Post{Type: model.POST_AUTO_RESPONDER}))
	})

	t.Run("should return false in case the status is STATUS_ONLINE", func(t *testing.T) {
		user := th.CreateUser()

		th.App.SetStatusDoNotDisturb(user.Id)

		channelMemberNotificationProps := model.StringMap{
			model.EMAIL_NOTIFY_PROP:       model.CHANNEL_NOTIFY_DEFAULT,
			model.MARK_UNREAD_NOTIFY_PROP: model.CHANNEL_MARK_UNREAD_ALL,
		}

		assert.False(t, th.App.userAllowsEmail(user, channelMemberNotificationProps, &model.Post{Type: model.POST_AUTO_RESPONDER}))
	})

}

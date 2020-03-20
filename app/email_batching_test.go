// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/stretchr/testify/require"
)

func TestHandleNewNotifications(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	id1 := model.NewId()
	id2 := model.NewId()
	id3 := model.NewId()

	// test queueing of received posts by user
	job := NewEmailBatchingJob(th.Server, 128)

	job.handleNewNotifications()

	require.Empty(t, job.pendingNotifications, "shouldn't have added any pending notifications")

	job.Add(&model.User{Id: id1}, &model.Post{UserId: id1, Message: "test"}, &model.Team{Name: "team"})
	require.Empty(t, job.pendingNotifications, "shouldn't have added any pending notifications")

	job.handleNewNotifications()
	require.Len(t, job.pendingNotifications, 1, "should have received posts for 1 user")
	require.Len(t, job.pendingNotifications[id1], 1, "should have received 1 post for user")

	job.Add(&model.User{Id: id1}, &model.Post{UserId: id1, Message: "test"}, &model.Team{Name: "team"})
	job.handleNewNotifications()
	require.Len(t, job.pendingNotifications, 1, "should have received posts for 1 user")
	require.Len(t, job.pendingNotifications[id1], 2, "should have received 2 posts for user1")

	job.Add(&model.User{Id: id2}, &model.Post{UserId: id1, Message: "test"}, &model.Team{Name: "team"})
	job.handleNewNotifications()
	require.Len(t, job.pendingNotifications, 2, "should have received posts for 2 users")
	require.Len(t, job.pendingNotifications[id1], 2, "should have received 2 posts for user1")
	require.Len(t, job.pendingNotifications[id2], 1, "should have received 1 post for user2")

	job.Add(&model.User{Id: id2}, &model.Post{UserId: id2, Message: "test"}, &model.Team{Name: "team"})
	job.Add(&model.User{Id: id1}, &model.Post{UserId: id3, Message: "test"}, &model.Team{Name: "team"})
	job.Add(&model.User{Id: id3}, &model.Post{UserId: id3, Message: "test"}, &model.Team{Name: "team"})
	job.Add(&model.User{Id: id2}, &model.Post{UserId: id2, Message: "test"}, &model.Team{Name: "team"})
	job.handleNewNotifications()
	require.Len(t, job.pendingNotifications, 3, "should have received posts for 3 users")
	require.Len(t, job.pendingNotifications[id1], 3, "should have received 3 posts for user1")
	require.Len(t, job.pendingNotifications[id2], 3, "should have received 3 posts for user2")
	require.Len(t, job.pendingNotifications[id3], 1, "should have received 1 post for user3")

	// test ordering of received posts
	job = NewEmailBatchingJob(th.Server, 128)

	job.Add(&model.User{Id: id1}, &model.Post{UserId: id1, Message: "test1"}, &model.Team{Name: "team"})
	job.Add(&model.User{Id: id1}, &model.Post{UserId: id1, Message: "test2"}, &model.Team{Name: "team"})
	job.Add(&model.User{Id: id2}, &model.Post{UserId: id1, Message: "test3"}, &model.Team{Name: "team"})
	job.Add(&model.User{Id: id1}, &model.Post{UserId: id1, Message: "test4"}, &model.Team{Name: "team"})
	job.Add(&model.User{Id: id2}, &model.Post{UserId: id1, Message: "test5"}, &model.Team{Name: "team"})
	job.handleNewNotifications()
	assert.Equal(t, job.pendingNotifications[id1][0].post.Message, "test1", "incorrect order of received posts for user1")
	assert.Equal(t, job.pendingNotifications[id1][1].post.Message, "test2", "incorrect order of received posts for user1")
	assert.Equal(t, job.pendingNotifications[id1][2].post.Message, "test4", "incorrect order of received posts for user1")
	assert.Equal(t, job.pendingNotifications[id2][0].post.Message, "test3", "incorrect order of received posts for user2")
	assert.Equal(t, job.pendingNotifications[id2][1].post.Message, "test5", "incorrect order of received posts for user2")
}

func TestCheckPendingNotifications(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	job := NewEmailBatchingJob(th.Server, 128)
	job.pendingNotifications[th.BasicUser.Id] = []*batchedNotification{
		{
			post: &model.Post{
				UserId:    th.BasicUser.Id,
				ChannelId: th.BasicChannel.Id,
				CreateAt:  10000000,
			},
			teamName: th.BasicTeam.Name,
		},
	}

	channelMember, err := th.App.Srv().Store.Channel().GetMember(th.BasicChannel.Id, th.BasicUser.Id)
	require.Nil(t, err)
	channelMember.LastViewedAt = 9999999
	_, err = th.App.Srv().Store.Channel().UpdateMember(channelMember)
	require.Nil(t, err)

	err = th.App.Srv().Store.Preference().Save(&model.Preferences{{
		UserId:   th.BasicUser.Id,
		Category: model.PREFERENCE_CATEGORY_NOTIFICATIONS,
		Name:     model.PREFERENCE_NAME_EMAIL_INTERVAL,
		Value:    "60",
	}})
	require.Nil(t, err)

	// test that notifications aren't sent before interval
	job.checkPendingNotifications(time.Unix(10001, 0), func(string, []*batchedNotification) {})

	require.NotNil(t, job.pendingNotifications[th.BasicUser.Id])
	require.Len(t, job.pendingNotifications[th.BasicUser.Id], 1, "shouldn't have sent queued post")

	// test that notifications are cleared if the user has acted
	channelMember, err = th.App.Srv().Store.Channel().GetMember(th.BasicChannel.Id, th.BasicUser.Id)
	require.Nil(t, err)
	channelMember.LastViewedAt = 10001000
	_, err = th.App.Srv().Store.Channel().UpdateMember(channelMember)
	require.Nil(t, err)

	job.checkPendingNotifications(time.Unix(10002, 0), func(string, []*batchedNotification) {})

	require.Nil(t, job.pendingNotifications[th.BasicUser.Id])
	require.Empty(t, job.pendingNotifications[th.BasicUser.Id], "should've remove queued post since user acted")

	// test that notifications are sent if enough time passes since the first message
	job.pendingNotifications[th.BasicUser.Id] = []*batchedNotification{
		{
			post: &model.Post{
				UserId:    th.BasicUser.Id,
				ChannelId: th.BasicChannel.Id,
				CreateAt:  10060000,
				Message:   "post1",
			},
			teamName: th.BasicTeam.Name,
		},
		{
			post: &model.Post{
				UserId:    th.BasicUser.Id,
				ChannelId: th.BasicChannel.Id,
				CreateAt:  10090000,
				Message:   "post2",
			},
			teamName: th.BasicTeam.Name,
		},
	}

	received := make(chan *model.Post, 2)
	timeout := make(chan bool)

	job.checkPendingNotifications(time.Unix(10130, 0), func(s string, notifications []*batchedNotification) {
		for _, notification := range notifications {
			received <- notification.post
		}
	})

	go func() {
		// start a timeout to make sure that we don't get stuck here on a failed test
		time.Sleep(5 * time.Second)
		timeout <- true
	}()

	require.Nil(t, job.pendingNotifications[th.BasicUser.Id], "shouldn't have sent queued post")

	select {
	case post := <-received:
		require.Equal(t, post.Message, "post1", "should've received post1 first")

	case <-timeout:
		require.Fail(t, "timed out waiting for first post notification")
	}

	select {
	case post := <-received:
		require.Equal(t, post.Message, "post2", "should've received post2 second")

	case <-timeout:
		require.Fail(t, "timed out waiting for second post notification")
	}
}

/**
 * Ensures that email batch interval defaults to 15 minutes for users that haven't explicitly set this preference
 */
func TestCheckPendingNotificationsDefaultInterval(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	job := NewEmailBatchingJob(th.Server, 128)

	// bypasses recent user activity check
	channelMember, err := th.App.Srv().Store.Channel().GetMember(th.BasicChannel.Id, th.BasicUser.Id)
	require.Nil(t, err)
	channelMember.LastViewedAt = 9999000
	_, err = th.App.Srv().Store.Channel().UpdateMember(channelMember)
	require.Nil(t, err)

	job.pendingNotifications[th.BasicUser.Id] = []*batchedNotification{
		{
			post: &model.Post{
				UserId:    th.BasicUser.Id,
				ChannelId: th.BasicChannel.Id,
				CreateAt:  10000000,
			},
			teamName: th.BasicTeam.Name,
		},
	}

	// notifications should not be sent 1s after post was created, because default batch interval is 15mins
	job.checkPendingNotifications(time.Unix(10001, 0), func(string, []*batchedNotification) {})
	require.NotNil(t, job.pendingNotifications[th.BasicUser.Id])
	require.Len(t, job.pendingNotifications[th.BasicUser.Id], 1, "shouldn't have sent queued post")

	// notifications should be sent 901s after post was created, because default batch interval is 15mins
	job.checkPendingNotifications(time.Unix(10901, 0), func(string, []*batchedNotification) {})
	require.Nil(t, job.pendingNotifications[th.BasicUser.Id])
	require.Empty(t, job.pendingNotifications[th.BasicUser.Id], "should have sent queued post")
}

/**
 * Ensures that email batch interval defaults to 15 minutes if user preference is invalid
 */
func TestCheckPendingNotificationsCantParseInterval(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	job := NewEmailBatchingJob(th.Server, 128)

	// bypasses recent user activity check
	channelMember, err := th.App.Srv().Store.Channel().GetMember(th.BasicChannel.Id, th.BasicUser.Id)
	require.Nil(t, err)
	channelMember.LastViewedAt = 9999000
	_, err = th.App.Srv().Store.Channel().UpdateMember(channelMember)
	require.Nil(t, err)

	// preference value is not an integer, so we'll fall back to the default 15min value
	err = th.App.Srv().Store.Preference().Save(&model.Preferences{{
		UserId:   th.BasicUser.Id,
		Category: model.PREFERENCE_CATEGORY_NOTIFICATIONS,
		Name:     model.PREFERENCE_NAME_EMAIL_INTERVAL,
		Value:    "notAnIntegerValue",
	}})
	require.Nil(t, err)

	job.pendingNotifications[th.BasicUser.Id] = []*batchedNotification{
		{
			post: &model.Post{
				UserId:    th.BasicUser.Id,
				ChannelId: th.BasicChannel.Id,
				CreateAt:  10000000,
			},
			teamName: th.BasicTeam.Name,
		},
	}

	// notifications should not be sent 1s after post was created, because default batch interval is 15mins
	job.checkPendingNotifications(time.Unix(10001, 0), func(string, []*batchedNotification) {})
	require.NotNil(t, job.pendingNotifications[th.BasicUser.Id])
	require.Len(t, job.pendingNotifications[th.BasicUser.Id], 1, "shouldn't have sent queued post")

	// notifications should be sent 901s after post was created, because default batch interval is 15mins
	job.checkPendingNotifications(time.Unix(10901, 0), func(string, []*batchedNotification) {})

	require.Nil(t, job.pendingNotifications[th.BasicUser.Id], "should have sent queued post")
}

/*
 * Ensures that post contents are not included in notification email when email notification content type is set to generic
 */
func TestRenderBatchedPostGeneric(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	var post = &model.Post{}
	post.Message = "This is the message"
	var notification = &batchedNotification{}
	notification.post = post
	var channel = &model.Channel{}
	channel.DisplayName = "Some Test Channel"
	var sender = &model.User{}
	sender.Email = "sender@test.com"

	translateFunc := func(translationID string, args ...interface{}) string {
		// mock translateFunc just returns the translation id - this is good enough for our purposes
		return translationID
	}

	var rendered = th.Server.renderBatchedPost(notification, channel, sender, "http://localhost:8065", "", translateFunc, "en", model.EMAIL_NOTIFICATION_CONTENTS_GENERIC)
	require.NotContains(t, rendered, post.Message, "Rendered email should not contain post contents when email notification contents type is set to Generic.")
}

/*
 * Ensures that post contents included in notification email when email notification content type is set to full
 */
func TestRenderBatchedPostFull(t *testing.T) {
	th := SetupWithStoreMock(t)
	defer th.TearDown()

	var post = &model.Post{}
	post.Message = "This is the message"
	var notification = &batchedNotification{}
	notification.post = post
	var channel = &model.Channel{}
	channel.DisplayName = "Some Test Channel"
	var sender = &model.User{}
	sender.Email = "sender@test.com"

	translateFunc := func(translationID string, args ...interface{}) string {
		// mock translateFunc just returns the translation id - this is good enough for our purposes
		return translationID
	}

	var rendered = th.Server.renderBatchedPost(notification, channel, sender, "http://localhost:8065", "", translateFunc, "en", model.EMAIL_NOTIFICATION_CONTENTS_FULL)
	require.Contains(t, rendered, post.Message, "Rendered email should contain post contents when email notification contents type is set to Full.")
}

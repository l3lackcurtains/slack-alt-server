// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	team := th.BasicTeam

	channel := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.CHANNEL_OPEN, TeamId: team.Id}
	private := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.CHANNEL_PRIVATE, TeamId: team.Id}

	rchannel, resp := Client.CreateChannel(channel)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)

	require.Equal(t, channel.Name, rchannel.Name, "names did not match")
	require.Equal(t, channel.DisplayName, rchannel.DisplayName, "display names did not match")
	require.Equal(t, channel.TeamId, rchannel.TeamId, "team ids did not match")

	rprivate, resp := Client.CreateChannel(private)
	CheckNoError(t, resp)

	require.Equal(t, private.Name, rprivate.Name, "names did not match")
	require.Equal(t, model.CHANNEL_PRIVATE, rprivate.Type, "wrong channel type")
	require.Equal(t, th.BasicUser.Id, rprivate.CreatorId, "wrong creator id")

	_, resp = Client.CreateChannel(channel)
	CheckErrorMessage(t, resp, "store.sql_channel.save_channel.exists.app_error")
	CheckBadRequestStatus(t, resp)

	direct := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.CHANNEL_DIRECT, TeamId: team.Id}
	_, resp = Client.CreateChannel(direct)
	CheckErrorMessage(t, resp, "api.channel.create_channel.direct_channel.app_error")
	CheckBadRequestStatus(t, resp)

	Client.Logout()
	_, resp = Client.CreateChannel(channel)
	CheckUnauthorizedStatus(t, resp)

	userNotOnTeam := th.CreateUser()
	Client.Login(userNotOnTeam.Email, userNotOnTeam.Password)

	_, resp = Client.CreateChannel(channel)
	CheckForbiddenStatus(t, resp)

	_, resp = Client.CreateChannel(private)
	CheckForbiddenStatus(t, resp)

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()

	th.AddPermissionToRole(model.PERMISSION_CREATE_PUBLIC_CHANNEL.Id, model.TEAM_USER_ROLE_ID)
	th.AddPermissionToRole(model.PERMISSION_CREATE_PRIVATE_CHANNEL.Id, model.TEAM_USER_ROLE_ID)

	th.LoginBasic()

	channel.Name = GenerateTestChannelName()
	_, resp = Client.CreateChannel(channel)
	CheckNoError(t, resp)

	private.Name = GenerateTestChannelName()
	_, resp = Client.CreateChannel(private)
	CheckNoError(t, resp)

	th.AddPermissionToRole(model.PERMISSION_CREATE_PUBLIC_CHANNEL.Id, model.TEAM_ADMIN_ROLE_ID)
	th.AddPermissionToRole(model.PERMISSION_CREATE_PRIVATE_CHANNEL.Id, model.TEAM_ADMIN_ROLE_ID)
	th.RemovePermissionFromRole(model.PERMISSION_CREATE_PUBLIC_CHANNEL.Id, model.TEAM_USER_ROLE_ID)
	th.RemovePermissionFromRole(model.PERMISSION_CREATE_PRIVATE_CHANNEL.Id, model.TEAM_USER_ROLE_ID)

	_, resp = Client.CreateChannel(channel)
	CheckForbiddenStatus(t, resp)

	_, resp = Client.CreateChannel(private)
	CheckForbiddenStatus(t, resp)

	th.LoginTeamAdmin()

	channel.Name = GenerateTestChannelName()
	_, resp = Client.CreateChannel(channel)
	CheckNoError(t, resp)

	private.Name = GenerateTestChannelName()
	_, resp = Client.CreateChannel(private)
	CheckNoError(t, resp)

	channel.Name = GenerateTestChannelName()
	_, resp = th.SystemAdminClient.CreateChannel(channel)
	CheckNoError(t, resp)

	private.Name = GenerateTestChannelName()
	_, resp = th.SystemAdminClient.CreateChannel(private)
	CheckNoError(t, resp)

	// Test posting Garbage
	r, err := Client.DoApiPost("/channels", "garbage")
	require.NotNil(t, err, "expected error")
	require.Equal(t, http.StatusBadRequest, r.StatusCode, "Expected 400 Bad Request")

	// Test GroupConstrained flag
	groupConstrainedChannel := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.CHANNEL_OPEN, TeamId: team.Id, GroupConstrained: model.NewBool(true)}
	rchannel, resp = Client.CreateChannel(groupConstrainedChannel)
	CheckNoError(t, resp)

	require.Equal(t, *groupConstrainedChannel.GroupConstrained, *rchannel.GroupConstrained, "GroupConstrained flags do not match")
}

func TestUpdateChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	team := th.BasicTeam

	channel := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.CHANNEL_OPEN, TeamId: team.Id}
	private := &model.Channel{DisplayName: "Test API Name", Name: GenerateTestChannelName(), Type: model.CHANNEL_PRIVATE, TeamId: team.Id}

	channel, _ = Client.CreateChannel(channel)
	private, _ = Client.CreateChannel(private)

	//Update a open channel
	channel.DisplayName = "My new display name"
	channel.Header = "My fancy header"
	channel.Purpose = "Mattermost ftw!"

	newChannel, resp := Client.UpdateChannel(channel)
	CheckNoError(t, resp)

	require.Equal(t, channel.DisplayName, newChannel.DisplayName, "Update failed for DisplayName")
	require.Equal(t, channel.Header, newChannel.Header, "Update failed for Header")
	require.Equal(t, channel.Purpose, newChannel.Purpose, "Update failed for Purpose")

	// Test GroupConstrained flag
	channel.GroupConstrained = model.NewBool(true)
	rchannel, resp := Client.UpdateChannel(channel)
	CheckNoError(t, resp)
	CheckOKStatus(t, resp)

	require.Equal(t, *channel.GroupConstrained, *rchannel.GroupConstrained, "GroupConstrained flags do not match")

	//Update a private channel
	private.DisplayName = "My new display name for private channel"
	private.Header = "My fancy private header"
	private.Purpose = "Mattermost ftw! in private mode"

	newPrivateChannel, resp := Client.UpdateChannel(private)
	CheckNoError(t, resp)

	require.Equal(t, private.DisplayName, newPrivateChannel.DisplayName, "Update failed for DisplayName in private channel")
	require.Equal(t, private.Header, newPrivateChannel.Header, "Update failed for Header in private channel")
	require.Equal(t, private.Purpose, newPrivateChannel.Purpose, "Update failed for Purpose in private channel")

	// Test that changing the type fails and returns error

	private.Type = model.CHANNEL_OPEN
	newPrivateChannel, resp = Client.UpdateChannel(private)
	CheckBadRequestStatus(t, resp)

	// Test that keeping the same type succeeds

	private.Type = model.CHANNEL_PRIVATE
	newPrivateChannel, resp = Client.UpdateChannel(private)
	CheckNoError(t, resp)

	//Non existing channel
	channel1 := &model.Channel{DisplayName: "Test API Name for apiv4", Name: GenerateTestChannelName(), Type: model.CHANNEL_OPEN, TeamId: team.Id}
	_, resp = Client.UpdateChannel(channel1)
	CheckNotFoundStatus(t, resp)

	//Try to update with not logged user
	Client.Logout()
	_, resp = Client.UpdateChannel(channel)
	CheckUnauthorizedStatus(t, resp)

	//Try to update using another user
	user := th.CreateUser()
	Client.Login(user.Email, user.Password)

	channel.DisplayName = "Should not update"
	_, resp = Client.UpdateChannel(channel)
	CheckForbiddenStatus(t, resp)

	// Test updating the header of someone else's GM channel.
	user1 := th.CreateUser()
	user2 := th.CreateUser()
	user3 := th.CreateUser()

	groupChannel, resp := Client.CreateGroupChannel([]string{user1.Id, user2.Id})
	CheckNoError(t, resp)

	groupChannel.Header = "lolololol"
	Client.Logout()
	Client.Login(user3.Email, user3.Password)
	_, resp = Client.UpdateChannel(groupChannel)
	CheckForbiddenStatus(t, resp)

	// Test updating the header of someone else's GM channel.
	Client.Logout()
	Client.Login(user.Email, user.Password)

	directChannel, resp := Client.CreateDirectChannel(user.Id, user1.Id)
	CheckNoError(t, resp)

	directChannel.Header = "lolololol"
	Client.Logout()
	Client.Login(user3.Email, user3.Password)
	_, resp = Client.UpdateChannel(directChannel)
	CheckForbiddenStatus(t, resp)
}

func TestPatchChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	patch := &model.ChannelPatch{
		Name:        new(string),
		DisplayName: new(string),
		Header:      new(string),
		Purpose:     new(string),
	}
	*patch.Name = model.NewId()
	*patch.DisplayName = model.NewId()
	*patch.Header = model.NewId()
	*patch.Purpose = model.NewId()

	channel, resp := Client.PatchChannel(th.BasicChannel.Id, patch)
	CheckNoError(t, resp)

	require.Equal(t, *patch.Name, channel.Name, "do not match")
	require.Equal(t, *patch.DisplayName, channel.DisplayName, "do not match")
	require.Equal(t, *patch.Header, channel.Header, "do not match")
	require.Equal(t, *patch.Purpose, channel.Purpose, "do not match")

	patch.Name = nil
	oldName := channel.Name
	channel, resp = Client.PatchChannel(th.BasicChannel.Id, patch)
	CheckNoError(t, resp)

	require.Equal(t, oldName, channel.Name, "should not have updated")

	// Test GroupConstrained flag
	patch.GroupConstrained = model.NewBool(true)
	rchannel, resp := Client.PatchChannel(th.BasicChannel.Id, patch)
	CheckNoError(t, resp)
	CheckOKStatus(t, resp)

	require.Equal(t, *rchannel.GroupConstrained, *patch.GroupConstrained, "GroupConstrained flags do not match")
	patch.GroupConstrained = nil

	_, resp = Client.PatchChannel("junk", patch)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.PatchChannel(model.NewId(), patch)
	CheckNotFoundStatus(t, resp)

	user := th.CreateUser()
	Client.Login(user.Email, user.Password)
	_, resp = Client.PatchChannel(th.BasicChannel.Id, patch)
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.PatchChannel(th.BasicChannel.Id, patch)
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.PatchChannel(th.BasicPrivateChannel.Id, patch)
	CheckNoError(t, resp)

	// Test updating the header of someone else's GM channel.
	user1 := th.CreateUser()
	user2 := th.CreateUser()
	user3 := th.CreateUser()

	groupChannel, resp := Client.CreateGroupChannel([]string{user1.Id, user2.Id})
	CheckNoError(t, resp)

	Client.Logout()
	Client.Login(user3.Email, user3.Password)

	channelPatch := &model.ChannelPatch{}
	channelPatch.Header = new(string)
	*channelPatch.Header = "lolololol"

	_, resp = Client.PatchChannel(groupChannel.Id, channelPatch)
	CheckForbiddenStatus(t, resp)

	// Test updating the header of someone else's GM channel.
	Client.Logout()
	Client.Login(user.Email, user.Password)

	directChannel, resp := Client.CreateDirectChannel(user.Id, user1.Id)
	CheckNoError(t, resp)

	Client.Logout()
	Client.Login(user3.Email, user3.Password)
	_, resp = Client.PatchChannel(directChannel.Id, channelPatch)
	CheckForbiddenStatus(t, resp)
}

func TestCreateDirectChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user1 := th.BasicUser
	user2 := th.BasicUser2
	user3 := th.CreateUser()

	dm, resp := Client.CreateDirectChannel(user1.Id, user2.Id)
	CheckNoError(t, resp)

	channelName := ""
	if user2.Id > user1.Id {
		channelName = user1.Id + "__" + user2.Id
	} else {
		channelName = user2.Id + "__" + user1.Id
	}

	require.Equal(t, channelName, dm.Name, "dm name didn't match")

	_, resp = Client.CreateDirectChannel("junk", user2.Id)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.CreateDirectChannel(user1.Id, model.NewId())
	CheckBadRequestStatus(t, resp)

	_, resp = Client.CreateDirectChannel(model.NewId(), user1.Id)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.CreateDirectChannel(model.NewId(), user2.Id)
	CheckForbiddenStatus(t, resp)

	r, err := Client.DoApiPost("/channels/direct", "garbage")
	require.NotNil(t, err)
	require.Equal(t, http.StatusBadRequest, r.StatusCode)

	Client.Logout()
	_, resp = Client.CreateDirectChannel(model.NewId(), user2.Id)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.CreateDirectChannel(user3.Id, user2.Id)
	CheckNoError(t, resp)
}

func TestCreateDirectChannelAsGuest(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user1 := th.BasicUser

	enableGuestAccounts := *th.App.Config().GuestAccountsSettings.Enable
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = enableGuestAccounts })
		th.App.RemoveLicense()
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })
	th.App.SetLicense(model.NewTestLicense())

	id := model.NewId()
	guest := &model.User{
		Email:         "success+" + id + "@simulator.amazonses.com",
		Username:      "un_" + id,
		Nickname:      "nn_" + id,
		Password:      "Password1",
		EmailVerified: true,
	}
	guest, err := th.App.CreateGuest(guest)
	require.Nil(t, err)

	_, resp := Client.Login(guest.Username, "Password1")
	CheckNoError(t, resp)

	t.Run("Try to created DM with not visible user", func(t *testing.T) {
		_, resp := Client.CreateDirectChannel(guest.Id, user1.Id)
		CheckForbiddenStatus(t, resp)

		_, resp = Client.CreateDirectChannel(user1.Id, guest.Id)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("Creating DM with visible user", func(t *testing.T) {
		th.LinkUserToTeam(guest, th.BasicTeam)
		th.AddUserToChannel(guest, th.BasicChannel)

		_, resp := Client.CreateDirectChannel(guest.Id, user1.Id)
		CheckNoError(t, resp)
	})
}

func TestDeleteDirectChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user := th.BasicUser
	user2 := th.BasicUser2

	rgc, resp := Client.CreateDirectChannel(user.Id, user2.Id)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)
	require.NotNil(t, rgc, "should have created a direct channel")

	deleted, resp := Client.DeleteChannel(rgc.Id)
	CheckErrorMessage(t, resp, "api.channel.delete_channel.type.invalid")
	require.False(t, deleted, "should not have been able to delete direct channel.")
}

func TestCreateGroupChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user := th.BasicUser
	user2 := th.BasicUser2
	user3 := th.CreateUser()

	userIds := []string{user.Id, user2.Id, user3.Id}

	rgc, resp := Client.CreateGroupChannel(userIds)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)

	require.NotNil(t, rgc, "should have created a group channel")
	require.Equal(t, model.CHANNEL_GROUP, rgc.Type, "should have created a channel of group type")

	m, _ := th.App.GetChannelMembersPage(rgc.Id, 0, 10)
	require.Len(t, *m, 3, "should have 3 channel members")

	// saving duplicate group channel
	rgc2, resp := Client.CreateGroupChannel([]string{user3.Id, user2.Id})
	CheckNoError(t, resp)
	require.Equal(t, rgc.Id, rgc2.Id, "should have returned existing channel")

	m2, _ := th.App.GetChannelMembersPage(rgc2.Id, 0, 10)
	require.Equal(t, m, m2)

	_, resp = Client.CreateGroupChannel([]string{user2.Id})
	CheckBadRequestStatus(t, resp)

	user4 := th.CreateUser()
	user5 := th.CreateUser()
	user6 := th.CreateUser()
	user7 := th.CreateUser()
	user8 := th.CreateUser()
	user9 := th.CreateUser()

	rgc, resp = Client.CreateGroupChannel([]string{user.Id, user2.Id, user3.Id, user4.Id, user5.Id, user6.Id, user7.Id, user8.Id, user9.Id})
	CheckBadRequestStatus(t, resp)
	require.Nil(t, rgc)

	_, resp = Client.CreateGroupChannel([]string{user.Id, user2.Id, user3.Id, GenerateTestId()})
	CheckBadRequestStatus(t, resp)

	_, resp = Client.CreateGroupChannel([]string{user.Id, user2.Id, user3.Id, "junk"})
	CheckBadRequestStatus(t, resp)

	Client.Logout()

	_, resp = Client.CreateGroupChannel(userIds)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.CreateGroupChannel(userIds)
	CheckNoError(t, resp)
}

func TestCreateGroupChannelAsGuest(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user1 := th.BasicUser
	user2 := th.BasicUser2
	user3 := th.CreateUser()
	user4 := th.CreateUser()
	user5 := th.CreateUser()
	th.LinkUserToTeam(user2, th.BasicTeam)
	th.AddUserToChannel(user2, th.BasicChannel)
	th.LinkUserToTeam(user3, th.BasicTeam)
	th.AddUserToChannel(user3, th.BasicChannel)

	enableGuestAccounts := *th.App.Config().GuestAccountsSettings.Enable
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = enableGuestAccounts })
		th.App.RemoveLicense()
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })
	th.App.SetLicense(model.NewTestLicense())

	id := model.NewId()
	guest := &model.User{
		Email:         "success+" + id + "@simulator.amazonses.com",
		Username:      "un_" + id,
		Nickname:      "nn_" + id,
		Password:      "Password1",
		EmailVerified: true,
	}
	guest, err := th.App.CreateGuest(guest)
	require.Nil(t, err)

	_, resp := Client.Login(guest.Username, "Password1")
	CheckNoError(t, resp)

	t.Run("Try to created GM with not visible users", func(t *testing.T) {
		_, resp := Client.CreateGroupChannel([]string{guest.Id, user1.Id, user2.Id, user3.Id})
		CheckForbiddenStatus(t, resp)

		_, resp = Client.CreateGroupChannel([]string{user1.Id, user2.Id, guest.Id, user3.Id})
		CheckForbiddenStatus(t, resp)
	})

	t.Run("Try to created GM with visible and not visible users", func(t *testing.T) {
		th.LinkUserToTeam(guest, th.BasicTeam)
		th.AddUserToChannel(guest, th.BasicChannel)

		_, resp := Client.CreateGroupChannel([]string{guest.Id, user1.Id, user3.Id, user4.Id, user5.Id})
		CheckForbiddenStatus(t, resp)

		_, resp = Client.CreateGroupChannel([]string{user1.Id, user2.Id, guest.Id, user4.Id, user5.Id})
		CheckForbiddenStatus(t, resp)
	})

	t.Run("Creating GM with visible users", func(t *testing.T) {
		_, resp := Client.CreateGroupChannel([]string{guest.Id, user1.Id, user2.Id, user3.Id})
		CheckNoError(t, resp)
	})
}

func TestDeleteGroupChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user := th.BasicUser
	user2 := th.BasicUser2
	user3 := th.CreateUser()

	userIds := []string{user.Id, user2.Id, user3.Id}

	rgc, resp := Client.CreateGroupChannel(userIds)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)
	require.NotNil(t, rgc, "should have created a group channel")

	deleted, resp := Client.DeleteChannel(rgc.Id)
	CheckErrorMessage(t, resp, "api.channel.delete_channel.type.invalid")
	require.False(t, deleted, "should not have been able to delete group channel.")
}

func TestGetChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	channel, resp := Client.GetChannel(th.BasicChannel.Id, "")
	CheckNoError(t, resp)
	require.Equal(t, th.BasicChannel.Id, channel.Id, "ids did not match")

	Client.RemoveUserFromChannel(th.BasicChannel.Id, th.BasicUser.Id)
	_, resp = Client.GetChannel(th.BasicChannel.Id, "")
	CheckNoError(t, resp)

	channel, resp = Client.GetChannel(th.BasicPrivateChannel.Id, "")
	CheckNoError(t, resp)
	require.Equal(t, th.BasicPrivateChannel.Id, channel.Id, "ids did not match")

	Client.RemoveUserFromChannel(th.BasicPrivateChannel.Id, th.BasicUser.Id)
	_, resp = Client.GetChannel(th.BasicPrivateChannel.Id, "")
	CheckForbiddenStatus(t, resp)

	_, resp = Client.GetChannel(model.NewId(), "")
	CheckNotFoundStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetChannel(th.BasicChannel.Id, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	Client.Login(user.Email, user.Password)
	_, resp = Client.GetChannel(th.BasicChannel.Id, "")
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetChannel(th.BasicChannel.Id, "")
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.GetChannel(th.BasicPrivateChannel.Id, "")
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.GetChannel(th.BasicUser.Id, "")
	CheckNotFoundStatus(t, resp)
}

func TestGetDeletedChannelsForTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	team := th.BasicTeam

	th.LoginTeamAdmin()

	channels, resp := Client.GetDeletedChannelsForTeam(team.Id, 0, 100, "")
	CheckNoError(t, resp)
	numInitialChannelsForTeam := len(channels)

	// create and delete public channel
	publicChannel1 := th.CreatePublicChannel()
	Client.DeleteChannel(publicChannel1.Id)

	channels, resp = Client.GetDeletedChannelsForTeam(team.Id, 0, 100, "")
	CheckNoError(t, resp)
	require.Len(t, channels, numInitialChannelsForTeam+1, "should be 1 deleted channel")

	publicChannel2 := th.CreatePublicChannel()
	Client.DeleteChannel(publicChannel2.Id)

	channels, resp = Client.GetDeletedChannelsForTeam(team.Id, 0, 100, "")
	CheckNoError(t, resp)
	require.Len(t, channels, numInitialChannelsForTeam+2, "should be 2 deleted channels")

	th.LoginBasic()

	privateChannel1 := th.CreatePrivateChannel()
	Client.DeleteChannel(privateChannel1.Id)

	channels, resp = Client.GetDeletedChannelsForTeam(team.Id, 0, 100, "")
	CheckNoError(t, resp)
	require.Len(t, channels, numInitialChannelsForTeam+3)

	// Login as different user and create private channel
	th.LoginBasic2()
	privateChannel2 := th.CreatePrivateChannel()
	Client.DeleteChannel(privateChannel2.Id)

	// Log back in as first user
	th.LoginBasic()

	channels, resp = Client.GetDeletedChannelsForTeam(team.Id, 0, 100, "")
	CheckNoError(t, resp)
	require.Len(t, channels, numInitialChannelsForTeam+3)

	channels, resp = Client.GetDeletedChannelsForTeam(team.Id, 0, 1, "")
	CheckNoError(t, resp)
	require.Len(t, channels, 1, "should be one channel per page")

	channels, resp = Client.GetDeletedChannelsForTeam(team.Id, 1, 1, "")
	CheckNoError(t, resp)
	require.Len(t, channels, 1, "should be one channel per page")
}

func TestGetPublicChannelsForTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	team := th.BasicTeam
	publicChannel1 := th.BasicChannel
	publicChannel2 := th.BasicChannel2

	channels, resp := Client.GetPublicChannelsForTeam(team.Id, 0, 100, "")
	CheckNoError(t, resp)
	require.Len(t, channels, 4, "wrong path")

	for i, c := range channels {
		// check all channels included are open
		require.Equal(t, model.CHANNEL_OPEN, c.Type, "should include open channel only")

		// only check the created 2 public channels
		require.False(t, i < 2 && !(c.DisplayName == publicChannel1.DisplayName || c.DisplayName == publicChannel2.DisplayName), "should match public channel display name")
	}

	privateChannel := th.CreatePrivateChannel()
	channels, resp = Client.GetPublicChannelsForTeam(team.Id, 0, 100, "")
	CheckNoError(t, resp)
	require.Len(t, channels, 4, "incorrect length of team public channels")

	for _, c := range channels {
		require.Equal(t, model.CHANNEL_OPEN, c.Type, "should not include private channel")
		require.NotEqual(t, privateChannel.DisplayName, c.DisplayName, "should not match private channel display name")
	}

	channels, resp = Client.GetPublicChannelsForTeam(team.Id, 0, 1, "")
	CheckNoError(t, resp)
	require.Len(t, channels, 1, "should be one channel per page")

	channels, resp = Client.GetPublicChannelsForTeam(team.Id, 1, 1, "")
	CheckNoError(t, resp)
	require.Len(t, channels, 1, "should be one channel per page")

	channels, resp = Client.GetPublicChannelsForTeam(team.Id, 10000, 100, "")
	CheckNoError(t, resp)
	require.Empty(t, channels, "should be no channel")

	_, resp = Client.GetPublicChannelsForTeam("junk", 0, 100, "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetPublicChannelsForTeam(model.NewId(), 0, 100, "")
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetPublicChannelsForTeam(team.Id, 0, 100, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	Client.Login(user.Email, user.Password)
	_, resp = Client.GetPublicChannelsForTeam(team.Id, 0, 100, "")
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetPublicChannelsForTeam(team.Id, 0, 100, "")
	CheckNoError(t, resp)
}

func TestGetPublicChannelsByIdsForTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	teamId := th.BasicTeam.Id
	input := []string{th.BasicChannel.Id}
	output := []string{th.BasicChannel.DisplayName}

	channels, resp := Client.GetPublicChannelsByIdsForTeam(teamId, input)
	CheckNoError(t, resp)
	require.Len(t, channels, 1, "should return 1 channel")
	require.Equal(t, output[0], channels[0].DisplayName, "missing channel")

	input = append(input, GenerateTestId())
	input = append(input, th.BasicChannel2.Id)
	input = append(input, th.BasicPrivateChannel.Id)
	output = append(output, th.BasicChannel2.DisplayName)
	sort.Strings(output)

	channels, resp = Client.GetPublicChannelsByIdsForTeam(teamId, input)
	CheckNoError(t, resp)
	require.Len(t, channels, 2, "should return 2 channels")

	for i, c := range channels {
		require.Equal(t, output[i], c.DisplayName, "missing channel")
	}

	_, resp = Client.GetPublicChannelsByIdsForTeam(GenerateTestId(), input)
	CheckForbiddenStatus(t, resp)

	_, resp = Client.GetPublicChannelsByIdsForTeam(teamId, []string{})
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetPublicChannelsByIdsForTeam(teamId, []string{"junk"})
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetPublicChannelsByIdsForTeam(teamId, []string{GenerateTestId()})
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetPublicChannelsByIdsForTeam(teamId, []string{th.BasicPrivateChannel.Id})
	CheckNotFoundStatus(t, resp)

	Client.Logout()

	_, resp = Client.GetPublicChannelsByIdsForTeam(teamId, input)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.GetPublicChannelsByIdsForTeam(teamId, input)
	CheckNoError(t, resp)
}

func TestGetChannelsForTeamForUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	t.Run("get channels for the team for user", func(t *testing.T) {
		channels, resp := Client.GetChannelsForTeamForUser(th.BasicTeam.Id, th.BasicUser.Id, false, "")
		CheckNoError(t, resp)

		found := make([]bool, 3)
		for _, c := range channels {
			if c.Id == th.BasicChannel.Id {
				found[0] = true
			} else if c.Id == th.BasicChannel2.Id {
				found[1] = true
			} else if c.Id == th.BasicPrivateChannel.Id {
				found[2] = true
			}

			require.True(t, c.TeamId == "" || c.TeamId == th.BasicTeam.Id)
		}

		for _, f := range found {
			require.True(t, f, "missing a channel")
		}

		channels, resp = Client.GetChannelsForTeamForUser(th.BasicTeam.Id, th.BasicUser.Id, false, resp.Etag)
		CheckEtag(t, channels, resp)

		_, resp = Client.GetChannelsForTeamForUser(th.BasicTeam.Id, "junk", false, "")
		CheckBadRequestStatus(t, resp)

		_, resp = Client.GetChannelsForTeamForUser("junk", th.BasicUser.Id, false, "")
		CheckBadRequestStatus(t, resp)

		_, resp = Client.GetChannelsForTeamForUser(th.BasicTeam.Id, th.BasicUser2.Id, false, "")
		CheckForbiddenStatus(t, resp)

		_, resp = Client.GetChannelsForTeamForUser(model.NewId(), th.BasicUser.Id, false, "")
		CheckForbiddenStatus(t, resp)

		_, resp = th.SystemAdminClient.GetChannelsForTeamForUser(th.BasicTeam.Id, th.BasicUser.Id, false, "")
		CheckNoError(t, resp)
	})

	t.Run("deleted channel could be retrieved using the proper flag", func(t *testing.T) {
		testChannel := &model.Channel{
			DisplayName: "dn_" + model.NewId(),
			Name:        GenerateTestChannelName(),
			Type:        model.CHANNEL_OPEN,
			TeamId:      th.BasicTeam.Id,
			CreatorId:   th.BasicUser.Id,
		}
		th.App.CreateChannel(testChannel, true)
		defer th.App.PermanentDeleteChannel(testChannel)
		channels, resp := Client.GetChannelsForTeamForUser(th.BasicTeam.Id, th.BasicUser.Id, false, "")
		CheckNoError(t, resp)
		assert.Equal(t, 6, len(channels))
		th.App.DeleteChannel(testChannel, th.BasicUser.Id)
		channels, resp = Client.GetChannelsForTeamForUser(th.BasicTeam.Id, th.BasicUser.Id, false, "")
		CheckNoError(t, resp)
		assert.Equal(t, 5, len(channels))
	})
}

func TestGetAllChannels(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	channels, resp := th.SystemAdminClient.GetAllChannels(0, 20, "")
	CheckNoError(t, resp)

	// At least, all the not-deleted channels created during the InitBasic
	require.True(t, len(*channels) >= 3)
	for _, c := range *channels {
		require.NotEqual(t, c.TeamId, "")
	}

	channels, resp = th.SystemAdminClient.GetAllChannels(0, 10, "")
	CheckNoError(t, resp)
	require.True(t, len(*channels) >= 3)

	channels, resp = th.SystemAdminClient.GetAllChannels(1, 1, "")
	CheckNoError(t, resp)
	require.Len(t, *channels, 1)

	channels, resp = th.SystemAdminClient.GetAllChannels(10000, 10000, "")
	CheckNoError(t, resp)
	require.Empty(t, *channels)

	_, resp = Client.GetAllChannels(0, 20, "")
	CheckForbiddenStatus(t, resp)
}

func TestGetAllChannelsWithCount(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	channels, total, resp := th.SystemAdminClient.GetAllChannelsWithCount(0, 20, "")
	CheckNoError(t, resp)

	// At least, all the not-deleted channels created during the InitBasic
	require.True(t, len(*channels) >= 3)
	for _, c := range *channels {
		require.NotEqual(t, c.TeamId, "")
	}
	require.Equal(t, int64(6), total)

	channels, _, resp = th.SystemAdminClient.GetAllChannelsWithCount(0, 10, "")
	CheckNoError(t, resp)
	require.True(t, len(*channels) >= 3)

	channels, _, resp = th.SystemAdminClient.GetAllChannelsWithCount(1, 1, "")
	CheckNoError(t, resp)
	require.Len(t, *channels, 1)

	channels, _, resp = th.SystemAdminClient.GetAllChannelsWithCount(10000, 10000, "")
	CheckNoError(t, resp)
	require.Empty(t, *channels)

	_, _, resp = Client.GetAllChannelsWithCount(0, 20, "")
	CheckForbiddenStatus(t, resp)
}

func TestSearchChannels(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	search := &model.ChannelSearch{Term: th.BasicChannel.Name}

	channels, resp := Client.SearchChannels(th.BasicTeam.Id, search)
	CheckNoError(t, resp)

	found := false
	for _, c := range channels {
		require.Equal(t, model.CHANNEL_OPEN, c.Type, "should only return public channels")

		if c.Id == th.BasicChannel.Id {
			found = true
		}
	}
	require.True(t, found, "didn't find channel")

	search.Term = th.BasicPrivateChannel.Name
	channels, resp = Client.SearchChannels(th.BasicTeam.Id, search)
	CheckNoError(t, resp)

	found = false
	for _, c := range channels {
		if c.Id == th.BasicPrivateChannel.Id {
			found = true
		}
	}
	require.False(t, found, "shouldn't find private channel")

	search.Term = ""
	_, resp = Client.SearchChannels(th.BasicTeam.Id, search)
	CheckNoError(t, resp)

	search.Term = th.BasicChannel.Name
	_, resp = Client.SearchChannels(model.NewId(), search)
	CheckNotFoundStatus(t, resp)

	_, resp = Client.SearchChannels("junk", search)
	CheckBadRequestStatus(t, resp)

	_, resp = th.SystemAdminClient.SearchChannels(th.BasicTeam.Id, search)
	CheckNoError(t, resp)

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()

	// Remove list channels permission from the user
	th.RemovePermissionFromRole(model.PERMISSION_LIST_TEAM_CHANNELS.Id, model.TEAM_USER_ROLE_ID)

	t.Run("Search for a BasicChannel, which the user is a member of", func(t *testing.T) {
		search.Term = th.BasicChannel.Name
		channelList, resp := Client.SearchChannels(th.BasicTeam.Id, search)
		CheckNoError(t, resp)

		channelNames := []string{}
		for _, c := range channelList {
			channelNames = append(channelNames, c.Name)
		}
		require.Contains(t, channelNames, th.BasicChannel.Name)
	})

	t.Run("Remove the user from BasicChannel and search again, should not be returned", func(t *testing.T) {
		th.App.RemoveUserFromChannel(th.BasicUser.Id, th.BasicUser.Id, th.BasicChannel)

		search.Term = th.BasicChannel.Name
		channelList, resp := Client.SearchChannels(th.BasicTeam.Id, search)
		CheckNoError(t, resp)

		channelNames := []string{}
		for _, c := range channelList {
			channelNames = append(channelNames, c.Name)
		}
		require.NotContains(t, channelNames, th.BasicChannel.Name)
	})
}

func TestSearchArchivedChannels(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	search := &model.ChannelSearch{Term: th.BasicChannel.Name}

	Client.DeleteChannel(th.BasicChannel.Id)

	channels, resp := Client.SearchArchivedChannels(th.BasicTeam.Id, search)
	CheckNoError(t, resp)

	found := false
	for _, c := range channels {
		require.Equal(t, model.CHANNEL_OPEN, c.Type)

		if c.Id == th.BasicChannel.Id {
			found = true
		}
	}

	require.True(t, found)

	search.Term = th.BasicPrivateChannel.Name
	Client.DeleteChannel(th.BasicPrivateChannel.Id)

	channels, resp = Client.SearchArchivedChannels(th.BasicTeam.Id, search)
	CheckNoError(t, resp)

	found = false
	for _, c := range channels {
		if c.Id == th.BasicPrivateChannel.Id {
			found = true
		}
	}

	require.True(t, found)

	search.Term = ""
	_, resp = Client.SearchArchivedChannels(th.BasicTeam.Id, search)
	CheckNoError(t, resp)

	search.Term = th.BasicDeletedChannel.Name
	_, resp = Client.SearchArchivedChannels(model.NewId(), search)
	CheckNotFoundStatus(t, resp)

	_, resp = Client.SearchArchivedChannels("junk", search)
	CheckBadRequestStatus(t, resp)

	_, resp = th.SystemAdminClient.SearchArchivedChannels(th.BasicTeam.Id, search)
	CheckNoError(t, resp)

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()

	// Remove list channels permission from the user
	th.RemovePermissionFromRole(model.PERMISSION_LIST_TEAM_CHANNELS.Id, model.TEAM_USER_ROLE_ID)

	t.Run("Search for a BasicDeletedChannel, which the user is a member of", func(t *testing.T) {
		search.Term = th.BasicDeletedChannel.Name
		channelList, resp := Client.SearchArchivedChannels(th.BasicTeam.Id, search)
		CheckNoError(t, resp)

		channelNames := []string{}
		for _, c := range channelList {
			channelNames = append(channelNames, c.Name)
		}
		require.Contains(t, channelNames, th.BasicDeletedChannel.Name)
	})

	t.Run("Remove the user from BasicDeletedChannel and search again, should still return", func(t *testing.T) {
		th.App.RemoveUserFromChannel(th.BasicUser.Id, th.BasicUser.Id, th.BasicDeletedChannel)

		search.Term = th.BasicDeletedChannel.Name
		channelList, resp := Client.SearchArchivedChannels(th.BasicTeam.Id, search)
		CheckNoError(t, resp)

		channelNames := []string{}
		for _, c := range channelList {
			channelNames = append(channelNames, c.Name)
		}
		require.Contains(t, channelNames, th.BasicDeletedChannel.Name)
	})
}

func TestSearchAllChannels(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	search := &model.ChannelSearch{Term: th.BasicChannel.Name}

	channels, resp := th.SystemAdminClient.SearchAllChannels(search)
	CheckNoError(t, resp)

	assert.Len(t, *channels, 1)
	assert.Equal(t, th.BasicChannel.Id, (*channels)[0].Id)

	search.Term = th.BasicPrivateChannel.Name
	channels, resp = th.SystemAdminClient.SearchAllChannels(search)
	CheckNoError(t, resp)

	assert.Len(t, *channels, 1)
	assert.Equal(t, th.BasicPrivateChannel.Id, (*channels)[0].Id)

	search.Term = ""
	channels, resp = th.SystemAdminClient.SearchAllChannels(search)
	CheckNoError(t, resp)
	// At least, all the not-deleted channels created during the InitBasic
	assert.True(t, len(*channels) >= 3)

	search.Term = th.BasicChannel.Name
	_, resp = Client.SearchAllChannels(search)
	CheckForbiddenStatus(t, resp)
}

func TestSearchAllChannelsPaged(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	search := &model.ChannelSearch{Term: th.BasicChannel.Name}
	search.Term = ""
	search.Page = model.NewInt(0)
	search.PerPage = model.NewInt(2)
	channelsWithCount, resp := th.SystemAdminClient.SearchAllChannelsPaged(search)
	CheckNoError(t, resp)
	require.Len(t, *channelsWithCount.Channels, 2)

	search.Term = th.BasicChannel.Name
	_, resp = Client.SearchAllChannels(search)
	CheckForbiddenStatus(t, resp)
}

func TestSearchGroupChannels(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	u1 := th.CreateUserWithClient(th.SystemAdminClient)

	// Create a group channel in which base user belongs but not sysadmin
	gc1, resp := th.Client.CreateGroupChannel([]string{th.BasicUser.Id, th.BasicUser2.Id, u1.Id})
	CheckNoError(t, resp)
	defer th.Client.DeleteChannel(gc1.Id)

	gc2, resp := th.Client.CreateGroupChannel([]string{th.BasicUser.Id, th.BasicUser2.Id, th.SystemAdminUser.Id})
	CheckNoError(t, resp)
	defer th.Client.DeleteChannel(gc2.Id)

	search := &model.ChannelSearch{Term: th.BasicUser2.Username}

	// sysadmin should only find gc2 as he doesn't belong to gc1
	channels, resp := th.SystemAdminClient.SearchGroupChannels(search)
	CheckNoError(t, resp)

	assert.Len(t, channels, 1)
	assert.Equal(t, channels[0].Id, gc2.Id)

	// basic user should find both
	Client.Login(th.BasicUser.Username, th.BasicUser.Password)
	channels, resp = Client.SearchGroupChannels(search)
	CheckNoError(t, resp)

	assert.Len(t, channels, 2)
	channelIds := []string{}
	for _, c := range channels {
		channelIds = append(channelIds, c.Id)
	}
	assert.ElementsMatch(t, channelIds, []string{gc1.Id, gc2.Id})

	// searching for sysadmin, it should only find gc1
	search = &model.ChannelSearch{Term: th.SystemAdminUser.Username}
	channels, resp = Client.SearchGroupChannels(search)
	CheckNoError(t, resp)

	assert.Len(t, channels, 1)
	assert.Equal(t, channels[0].Id, gc2.Id)

	// with an empty search, response should be empty
	search = &model.ChannelSearch{Term: ""}
	channels, resp = Client.SearchGroupChannels(search)
	CheckNoError(t, resp)

	assert.Empty(t, channels)

	// search unprivileged, forbidden
	th.Client.Logout()
	_, resp = Client.SearchAllChannels(search)
	CheckUnauthorizedStatus(t, resp)
}

func TestDeleteChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	team := th.BasicTeam
	user := th.BasicUser
	user2 := th.BasicUser2

	// successful delete of public channel
	publicChannel1 := th.CreatePublicChannel()
	pass, resp := Client.DeleteChannel(publicChannel1.Id)
	CheckNoError(t, resp)

	require.True(t, pass, "should have passed")

	ch, err := th.App.GetChannel(publicChannel1.Id)
	require.True(t, err != nil || ch.DeleteAt != 0, "should have failed to get deleted channel, or returned one with a populated DeleteAt.")

	post1 := &model.Post{ChannelId: publicChannel1.Id, Message: "a" + GenerateTestId() + "a"}
	_, resp = Client.CreatePost(post1)
	require.NotNil(t, resp, "expected response to not be nil")

	// successful delete of private channel
	privateChannel2 := th.CreatePrivateChannel()
	_, resp = Client.DeleteChannel(privateChannel2.Id)
	CheckNoError(t, resp)

	// successful delete of channel with multiple members
	publicChannel3 := th.CreatePublicChannel()
	th.App.AddUserToChannel(user, publicChannel3)
	th.App.AddUserToChannel(user2, publicChannel3)
	_, resp = Client.DeleteChannel(publicChannel3.Id)
	CheckNoError(t, resp)

	// default channel cannot be deleted.
	defaultChannel, _ := th.App.GetChannelByName(model.DEFAULT_CHANNEL, team.Id, false)
	pass, resp = Client.DeleteChannel(defaultChannel.Id)
	CheckBadRequestStatus(t, resp)
	require.False(t, pass, "should have failed")

	// check system admin can delete a channel without any appropriate team or channel membership.
	sdTeam := th.CreateTeamWithClient(Client)
	sdPublicChannel := &model.Channel{
		DisplayName: "dn_" + model.NewId(),
		Name:        GenerateTestChannelName(),
		Type:        model.CHANNEL_OPEN,
		TeamId:      sdTeam.Id,
	}
	sdPublicChannel, resp = Client.CreateChannel(sdPublicChannel)
	CheckNoError(t, resp)
	_, resp = th.SystemAdminClient.DeleteChannel(sdPublicChannel.Id)
	CheckNoError(t, resp)

	sdPrivateChannel := &model.Channel{
		DisplayName: "dn_" + model.NewId(),
		Name:        GenerateTestChannelName(),
		Type:        model.CHANNEL_PRIVATE,
		TeamId:      sdTeam.Id,
	}
	sdPrivateChannel, resp = Client.CreateChannel(sdPrivateChannel)
	CheckNoError(t, resp)
	_, resp = th.SystemAdminClient.DeleteChannel(sdPrivateChannel.Id)
	CheckNoError(t, resp)

	th.LoginBasic()
	publicChannel5 := th.CreatePublicChannel()
	Client.Logout()

	Client.Login(user.Id, user.Password)
	_, resp = Client.DeleteChannel(publicChannel5.Id)
	CheckUnauthorizedStatus(t, resp)

	_, resp = Client.DeleteChannel("junk")
	CheckUnauthorizedStatus(t, resp)

	Client.Logout()
	_, resp = Client.DeleteChannel(GenerateTestId())
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.DeleteChannel(publicChannel5.Id)
	CheckNoError(t, resp)
}

func TestDeleteChannel2(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user := th.BasicUser

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()

	th.AddPermissionToRole(model.PERMISSION_DELETE_PUBLIC_CHANNEL.Id, model.CHANNEL_USER_ROLE_ID)
	th.AddPermissionToRole(model.PERMISSION_DELETE_PRIVATE_CHANNEL.Id, model.CHANNEL_USER_ROLE_ID)

	// channels created by SystemAdmin
	publicChannel6 := th.CreateChannelWithClient(th.SystemAdminClient, model.CHANNEL_OPEN)
	privateChannel7 := th.CreateChannelWithClient(th.SystemAdminClient, model.CHANNEL_PRIVATE)
	th.App.AddUserToChannel(user, publicChannel6)
	th.App.AddUserToChannel(user, privateChannel7)
	th.App.AddUserToChannel(user, privateChannel7)

	// successful delete by user
	_, resp := Client.DeleteChannel(publicChannel6.Id)
	CheckNoError(t, resp)

	_, resp = Client.DeleteChannel(privateChannel7.Id)
	CheckNoError(t, resp)

	// Restrict permissions to Channel Admins
	th.RemovePermissionFromRole(model.PERMISSION_DELETE_PUBLIC_CHANNEL.Id, model.CHANNEL_USER_ROLE_ID)
	th.RemovePermissionFromRole(model.PERMISSION_DELETE_PRIVATE_CHANNEL.Id, model.CHANNEL_USER_ROLE_ID)
	th.AddPermissionToRole(model.PERMISSION_DELETE_PUBLIC_CHANNEL.Id, model.CHANNEL_ADMIN_ROLE_ID)
	th.AddPermissionToRole(model.PERMISSION_DELETE_PRIVATE_CHANNEL.Id, model.CHANNEL_ADMIN_ROLE_ID)

	// channels created by SystemAdmin
	publicChannel6 = th.CreateChannelWithClient(th.SystemAdminClient, model.CHANNEL_OPEN)
	privateChannel7 = th.CreateChannelWithClient(th.SystemAdminClient, model.CHANNEL_PRIVATE)
	th.App.AddUserToChannel(user, publicChannel6)
	th.App.AddUserToChannel(user, privateChannel7)
	th.App.AddUserToChannel(user, privateChannel7)

	// cannot delete by user
	_, resp = Client.DeleteChannel(publicChannel6.Id)
	CheckForbiddenStatus(t, resp)

	_, resp = Client.DeleteChannel(privateChannel7.Id)
	CheckForbiddenStatus(t, resp)

	// successful delete by channel admin
	th.MakeUserChannelAdmin(user, publicChannel6)
	th.MakeUserChannelAdmin(user, privateChannel7)
	th.App.Srv().Store.Channel().ClearCaches()

	_, resp = Client.DeleteChannel(publicChannel6.Id)
	CheckNoError(t, resp)

	_, resp = Client.DeleteChannel(privateChannel7.Id)
	CheckNoError(t, resp)

	// Make sure team admins don't have permission to delete channels.
	th.RemovePermissionFromRole(model.PERMISSION_DELETE_PUBLIC_CHANNEL.Id, model.CHANNEL_ADMIN_ROLE_ID)
	th.RemovePermissionFromRole(model.PERMISSION_DELETE_PRIVATE_CHANNEL.Id, model.CHANNEL_ADMIN_ROLE_ID)

	// last member of a public channel should have required permission to delete
	publicChannel6 = th.CreateChannelWithClient(th.Client, model.CHANNEL_OPEN)
	_, resp = Client.DeleteChannel(publicChannel6.Id)
	CheckForbiddenStatus(t, resp)

	// last member of a private channel should not be able to delete it if they don't have required permissions
	privateChannel7 = th.CreateChannelWithClient(th.Client, model.CHANNEL_PRIVATE)
	_, resp = Client.DeleteChannel(privateChannel7.Id)
	CheckForbiddenStatus(t, resp)
}

func TestConvertChannelToPrivate(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	defaultChannel, _ := th.App.GetChannelByName(model.DEFAULT_CHANNEL, th.BasicTeam.Id, false)
	_, resp := Client.ConvertChannelToPrivate(defaultChannel.Id)
	CheckForbiddenStatus(t, resp)

	privateChannel := th.CreatePrivateChannel()
	_, resp = Client.ConvertChannelToPrivate(privateChannel.Id)
	CheckForbiddenStatus(t, resp)

	publicChannel := th.CreatePublicChannel()
	_, resp = Client.ConvertChannelToPrivate(publicChannel.Id)
	CheckForbiddenStatus(t, resp)

	th.LoginTeamAdmin()
	rchannel, resp := Client.ConvertChannelToPrivate(publicChannel.Id)
	CheckOKStatus(t, resp)
	require.Equal(t, model.CHANNEL_PRIVATE, rchannel.Type, "channel should be converted from public to private")

	rchannel, resp = th.SystemAdminClient.ConvertChannelToPrivate(privateChannel.Id)
	CheckBadRequestStatus(t, resp)
	require.Nil(t, rchannel, "should not return a channel")

	rchannel, resp = th.SystemAdminClient.ConvertChannelToPrivate(defaultChannel.Id)
	CheckBadRequestStatus(t, resp)
	require.Nil(t, rchannel, "should not return a channel")

	WebSocketClient, err := th.CreateWebSocketClient()
	require.Nil(t, err)
	WebSocketClient.Listen()

	publicChannel2 := th.CreatePublicChannel()
	rchannel, resp = th.SystemAdminClient.ConvertChannelToPrivate(publicChannel2.Id)
	CheckOKStatus(t, resp)
	require.Equal(t, model.CHANNEL_PRIVATE, rchannel.Type, "channel should be converted from public to private")

	stop := make(chan bool)
	eventHit := false

	go func() {
		for {
			select {
			case resp := <-WebSocketClient.EventChannel:
				if resp.EventType() == model.WEBSOCKET_EVENT_CHANNEL_CONVERTED && resp.GetData()["channel_id"].(string) == publicChannel2.Id {
					eventHit = true
				}
			case <-stop:
				return
			}
		}
	}()

	time.Sleep(400 * time.Millisecond)

	stop <- true

	require.True(t, eventHit, "did not receive channel_converted event")
}

func TestUpdateChannelPrivacy(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	type testTable []struct {
		name            string
		channel         *model.Channel
		expectedPrivacy string
	}

	defaultChannel, _ := th.App.GetChannelByName(model.DEFAULT_CHANNEL, th.BasicTeam.Id, false)
	privateChannel := th.CreatePrivateChannel()
	publicChannel := th.CreatePublicChannel()

	tt := testTable{
		{"Updating default channel should fail with forbidden status if not logged in", defaultChannel, model.CHANNEL_OPEN},
		{"Updating private channel should fail with forbidden status if not logged in", privateChannel, model.CHANNEL_PRIVATE},
		{"Updating public channel should fail with forbidden status if not logged in", publicChannel, model.CHANNEL_OPEN},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			_, resp := Client.UpdateChannelPrivacy(tc.channel.Id, tc.expectedPrivacy)
			CheckForbiddenStatus(t, resp)
		})
	}

	th.LoginTeamAdmin()

	tt = testTable{
		{"Converting default channel to private should fail", defaultChannel, model.CHANNEL_PRIVATE},
		{"Updating privacy to an invalid setting should fail", publicChannel, "invalid"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			_, resp := Client.UpdateChannelPrivacy(tc.channel.Id, tc.expectedPrivacy)
			CheckBadRequestStatus(t, resp)
		})
	}

	tt = testTable{
		{"Default channel should stay public", defaultChannel, model.CHANNEL_OPEN},
		{"Public channel should stay public", publicChannel, model.CHANNEL_OPEN},
		{"Private channel should stay private", privateChannel, model.CHANNEL_PRIVATE},
		{"Public channel should convert to private", publicChannel, model.CHANNEL_PRIVATE},
		{"Private channel should convert to public", privateChannel, model.CHANNEL_OPEN},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			updatedChannel, resp := Client.UpdateChannelPrivacy(tc.channel.Id, tc.expectedPrivacy)
			CheckNoError(t, resp)
			assert.Equal(t, tc.expectedPrivacy, updatedChannel.Type)
			updatedChannel, err := th.App.GetChannel(tc.channel.Id)
			require.Nil(t, err)
			assert.Equal(t, tc.expectedPrivacy, updatedChannel.Type)
		})
	}
}

func TestRestoreChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	publicChannel1 := th.CreatePublicChannel()
	Client.DeleteChannel(publicChannel1.Id)

	privateChannel1 := th.CreatePrivateChannel()
	Client.DeleteChannel(privateChannel1.Id)

	_, resp := Client.RestoreChannel(publicChannel1.Id)
	CheckForbiddenStatus(t, resp)

	_, resp = Client.RestoreChannel(privateChannel1.Id)
	CheckForbiddenStatus(t, resp)

	th.LoginTeamAdmin()

	_, resp = Client.RestoreChannel(publicChannel1.Id)
	CheckOKStatus(t, resp)

	_, resp = Client.RestoreChannel(privateChannel1.Id)
	CheckOKStatus(t, resp)
}

func TestGetChannelByName(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	channel, resp := Client.GetChannelByName(th.BasicChannel.Name, th.BasicTeam.Id, "")
	CheckNoError(t, resp)
	require.Equal(t, th.BasicChannel.Name, channel.Name, "names did not match")

	channel, resp = Client.GetChannelByName(th.BasicPrivateChannel.Name, th.BasicTeam.Id, "")
	CheckNoError(t, resp)
	require.Equal(t, th.BasicPrivateChannel.Name, channel.Name, "names did not match")

	_, resp = Client.GetChannelByName(strings.ToUpper(th.BasicPrivateChannel.Name), th.BasicTeam.Id, "")
	CheckNoError(t, resp)

	_, resp = Client.GetChannelByName(th.BasicDeletedChannel.Name, th.BasicTeam.Id, "")
	CheckNotFoundStatus(t, resp)

	channel, resp = Client.GetChannelByNameIncludeDeleted(th.BasicDeletedChannel.Name, th.BasicTeam.Id, "")
	CheckNoError(t, resp)
	require.Equal(t, th.BasicDeletedChannel.Name, channel.Name, "names did not match")

	Client.RemoveUserFromChannel(th.BasicChannel.Id, th.BasicUser.Id)
	_, resp = Client.GetChannelByName(th.BasicChannel.Name, th.BasicTeam.Id, "")
	CheckNoError(t, resp)

	Client.RemoveUserFromChannel(th.BasicPrivateChannel.Id, th.BasicUser.Id)
	_, resp = Client.GetChannelByName(th.BasicPrivateChannel.Name, th.BasicTeam.Id, "")
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetChannelByName(GenerateTestChannelName(), th.BasicTeam.Id, "")
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetChannelByName(GenerateTestChannelName(), "junk", "")
	CheckBadRequestStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetChannelByName(th.BasicChannel.Name, th.BasicTeam.Id, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	Client.Login(user.Email, user.Password)
	_, resp = Client.GetChannelByName(th.BasicChannel.Name, th.BasicTeam.Id, "")
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetChannelByName(th.BasicChannel.Name, th.BasicTeam.Id, "")
	CheckNoError(t, resp)
}

func TestGetChannelByNameForTeamName(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	channel, resp := th.SystemAdminClient.GetChannelByNameForTeamName(th.BasicChannel.Name, th.BasicTeam.Name, "")
	CheckNoError(t, resp)
	require.Equal(t, th.BasicChannel.Name, channel.Name, "names did not match")

	_, resp = Client.GetChannelByNameForTeamName(th.BasicChannel.Name, th.BasicTeam.Name, "")
	CheckNoError(t, resp)

	_, resp = Client.GetChannelByNameForTeamName(th.BasicDeletedChannel.Name, th.BasicTeam.Name, "")
	CheckNotFoundStatus(t, resp)

	channel, resp = Client.GetChannelByNameForTeamNameIncludeDeleted(th.BasicDeletedChannel.Name, th.BasicTeam.Name, "")
	CheckNoError(t, resp)
	require.Equal(t, th.BasicDeletedChannel.Name, channel.Name, "names did not match")

	_, resp = Client.GetChannelByNameForTeamName(th.BasicChannel.Name, model.NewRandomString(15), "")
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetChannelByNameForTeamName(GenerateTestChannelName(), th.BasicTeam.Name, "")
	CheckNotFoundStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetChannelByNameForTeamName(th.BasicChannel.Name, th.BasicTeam.Name, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	Client.Login(user.Email, user.Password)
	_, resp = Client.GetChannelByNameForTeamName(th.BasicChannel.Name, th.BasicTeam.Name, "")
	CheckNotFoundStatus(t, resp)
}

func TestGetChannelMembers(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	members, resp := Client.GetChannelMembers(th.BasicChannel.Id, 0, 60, "")
	CheckNoError(t, resp)
	require.Len(t, *members, 3, "should only be 3 users in channel")

	members, resp = Client.GetChannelMembers(th.BasicChannel.Id, 0, 2, "")
	CheckNoError(t, resp)
	require.Len(t, *members, 2, "should only be 2 users")

	members, resp = Client.GetChannelMembers(th.BasicChannel.Id, 1, 1, "")
	CheckNoError(t, resp)
	require.Len(t, *members, 1, "should only be 1 user")

	members, resp = Client.GetChannelMembers(th.BasicChannel.Id, 1000, 100000, "")
	CheckNoError(t, resp)
	require.Empty(t, *members, "should be 0 users")

	_, resp = Client.GetChannelMembers("", 0, 60, "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetChannelMembers("junk", 0, 60, "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetChannelMembers(model.NewId(), 0, 60, "")
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetChannelMembers(th.BasicChannel.Id, 0, 60, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	Client.Login(user.Email, user.Password)
	_, resp = Client.GetChannelMembers(th.BasicChannel.Id, 0, 60, "")
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetChannelMembers(th.BasicChannel.Id, 0, 60, "")
	CheckNoError(t, resp)
}

func TestGetChannelMembersByIds(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	cm, resp := Client.GetChannelMembersByIds(th.BasicChannel.Id, []string{th.BasicUser.Id})
	CheckNoError(t, resp)
	require.Equal(t, th.BasicUser.Id, (*cm)[0].UserId, "returned wrong user")

	_, resp = Client.GetChannelMembersByIds(th.BasicChannel.Id, []string{})
	CheckBadRequestStatus(t, resp)

	cm1, resp := Client.GetChannelMembersByIds(th.BasicChannel.Id, []string{"junk"})
	CheckNoError(t, resp)
	require.Empty(t, *cm1, "no users should be returned")

	cm1, resp = Client.GetChannelMembersByIds(th.BasicChannel.Id, []string{"junk", th.BasicUser.Id})
	CheckNoError(t, resp)
	require.Len(t, *cm1, 1, "1 member should be returned")

	cm1, resp = Client.GetChannelMembersByIds(th.BasicChannel.Id, []string{th.BasicUser2.Id, th.BasicUser.Id})
	CheckNoError(t, resp)
	require.Len(t, *cm1, 2, "2 members should be returned")

	_, resp = Client.GetChannelMembersByIds("junk", []string{th.BasicUser.Id})
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetChannelMembersByIds(model.NewId(), []string{th.BasicUser.Id})
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetChannelMembersByIds(th.BasicChannel.Id, []string{th.BasicUser.Id})
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.GetChannelMembersByIds(th.BasicChannel.Id, []string{th.BasicUser2.Id, th.BasicUser.Id})
	CheckNoError(t, resp)
}

func TestGetChannelMember(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	member, resp := Client.GetChannelMember(th.BasicChannel.Id, th.BasicUser.Id, "")
	CheckNoError(t, resp)
	require.Equal(t, th.BasicChannel.Id, member.ChannelId, "wrong channel id")
	require.Equal(t, th.BasicUser.Id, member.UserId, "wrong user id")

	_, resp = Client.GetChannelMember("", th.BasicUser.Id, "")
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetChannelMember("junk", th.BasicUser.Id, "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetChannelMember(model.NewId(), th.BasicUser.Id, "")
	CheckForbiddenStatus(t, resp)

	_, resp = Client.GetChannelMember(th.BasicChannel.Id, "", "")
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetChannelMember(th.BasicChannel.Id, "junk", "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetChannelMember(th.BasicChannel.Id, model.NewId(), "")
	CheckNotFoundStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetChannelMember(th.BasicChannel.Id, th.BasicUser.Id, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	Client.Login(user.Email, user.Password)
	_, resp = Client.GetChannelMember(th.BasicChannel.Id, th.BasicUser.Id, "")
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetChannelMember(th.BasicChannel.Id, th.BasicUser.Id, "")
	CheckNoError(t, resp)
}

func TestGetChannelMembersForUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	members, resp := Client.GetChannelMembersForUser(th.BasicUser.Id, th.BasicTeam.Id, "")
	CheckNoError(t, resp)
	require.Len(t, *members, 6, "should have 6 members on team")

	_, resp = Client.GetChannelMembersForUser("", th.BasicTeam.Id, "")
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetChannelMembersForUser("junk", th.BasicTeam.Id, "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetChannelMembersForUser(model.NewId(), th.BasicTeam.Id, "")
	CheckForbiddenStatus(t, resp)

	_, resp = Client.GetChannelMembersForUser(th.BasicUser.Id, "", "")
	CheckNotFoundStatus(t, resp)

	_, resp = Client.GetChannelMembersForUser(th.BasicUser.Id, "junk", "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetChannelMembersForUser(th.BasicUser.Id, model.NewId(), "")
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetChannelMembersForUser(th.BasicUser.Id, th.BasicTeam.Id, "")
	CheckUnauthorizedStatus(t, resp)

	user := th.CreateUser()
	Client.Login(user.Email, user.Password)
	_, resp = Client.GetChannelMembersForUser(th.BasicUser.Id, th.BasicTeam.Id, "")
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetChannelMembersForUser(th.BasicUser.Id, th.BasicTeam.Id, "")
	CheckNoError(t, resp)
}

func TestViewChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	view := &model.ChannelView{
		ChannelId: th.BasicChannel.Id,
	}

	viewResp, resp := Client.ViewChannel(th.BasicUser.Id, view)
	CheckNoError(t, resp)
	require.Equal(t, "OK", viewResp.Status, "should have passed")

	channel, _ := th.App.GetChannel(th.BasicChannel.Id)

	require.Equal(t, channel.LastPostAt, viewResp.LastViewedAtTimes[channel.Id], "LastPostAt does not match returned LastViewedAt time")

	view.PrevChannelId = th.BasicChannel.Id
	_, resp = Client.ViewChannel(th.BasicUser.Id, view)
	CheckNoError(t, resp)

	view.PrevChannelId = ""
	_, resp = Client.ViewChannel(th.BasicUser.Id, view)
	CheckNoError(t, resp)

	view.PrevChannelId = "junk"
	_, resp = Client.ViewChannel(th.BasicUser.Id, view)
	CheckBadRequestStatus(t, resp)

	// All blank is OK we use it for clicking off of the browser.
	view.PrevChannelId = ""
	view.ChannelId = ""
	_, resp = Client.ViewChannel(th.BasicUser.Id, view)
	CheckNoError(t, resp)

	view.PrevChannelId = ""
	view.ChannelId = "junk"
	_, resp = Client.ViewChannel(th.BasicUser.Id, view)
	CheckBadRequestStatus(t, resp)

	view.ChannelId = "correctlysizedjunkdddfdfdf"
	_, resp = Client.ViewChannel(th.BasicUser.Id, view)
	CheckBadRequestStatus(t, resp)
	view.ChannelId = th.BasicChannel.Id

	member, resp := Client.GetChannelMember(th.BasicChannel.Id, th.BasicUser.Id, "")
	CheckNoError(t, resp)
	channel, resp = Client.GetChannel(th.BasicChannel.Id, "")
	CheckNoError(t, resp)
	require.Equal(t, channel.TotalMsgCount, member.MsgCount, "should match message counts")
	require.Equal(t, int64(0), member.MentionCount, "should have no mentions")

	_, resp = Client.ViewChannel("junk", view)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.ViewChannel(th.BasicUser2.Id, view)
	CheckForbiddenStatus(t, resp)

	r, err := Client.DoApiPost(fmt.Sprintf("/channels/members/%v/view", th.BasicUser.Id), "garbage")
	require.NotNil(t, err)
	require.Equal(t, http.StatusBadRequest, r.StatusCode)

	Client.Logout()
	_, resp = Client.ViewChannel(th.BasicUser.Id, view)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.ViewChannel(th.BasicUser.Id, view)
	CheckNoError(t, resp)
}

func TestGetChannelUnread(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user := th.BasicUser
	channel := th.BasicChannel

	channelUnread, resp := Client.GetChannelUnread(channel.Id, user.Id)
	CheckNoError(t, resp)
	require.Equal(t, th.BasicTeam.Id, channelUnread.TeamId, "wrong team id returned for a regular user call")
	require.Equal(t, channel.Id, channelUnread.ChannelId, "wrong team id returned for a regular user call")

	_, resp = Client.GetChannelUnread("junk", user.Id)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetChannelUnread(channel.Id, "junk")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetChannelUnread(channel.Id, model.NewId())
	CheckForbiddenStatus(t, resp)

	_, resp = Client.GetChannelUnread(model.NewId(), user.Id)
	CheckForbiddenStatus(t, resp)

	newUser := th.CreateUser()
	Client.Login(newUser.Email, newUser.Password)
	_, resp = Client.GetChannelUnread(th.BasicChannel.Id, user.Id)
	CheckForbiddenStatus(t, resp)

	Client.Logout()

	_, resp = th.SystemAdminClient.GetChannelUnread(channel.Id, user.Id)
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.GetChannelUnread(model.NewId(), user.Id)
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetChannelUnread(channel.Id, model.NewId())
	CheckNotFoundStatus(t, resp)
}

func TestGetChannelStats(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	channel := th.CreatePrivateChannel()

	stats, resp := Client.GetChannelStats(channel.Id, "")
	CheckNoError(t, resp)

	require.Equal(t, channel.Id, stats.ChannelId, "couldnt't get extra info")
	require.Equal(t, int64(1), stats.MemberCount, "got incorrect member count")
	require.Equal(t, int64(0), stats.PinnedPostCount, "got incorrect pinned post count")

	th.CreatePinnedPostWithClient(th.Client, channel)
	stats, resp = Client.GetChannelStats(channel.Id, "")
	CheckNoError(t, resp)
	require.Equal(t, int64(1), stats.PinnedPostCount, "should have returned 1 pinned post count")

	_, resp = Client.GetChannelStats("junk", "")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.GetChannelStats(model.NewId(), "")
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetChannelStats(channel.Id, "")
	CheckUnauthorizedStatus(t, resp)

	th.LoginBasic2()

	_, resp = Client.GetChannelStats(channel.Id, "")
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.GetChannelStats(channel.Id, "")
	CheckNoError(t, resp)
}

func TestGetPinnedPosts(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	channel := th.BasicChannel

	posts, resp := Client.GetPinnedPosts(channel.Id, "")
	CheckNoError(t, resp)
	require.Empty(t, posts.Posts, "should not have gotten a pinned post")

	pinnedPost := th.CreatePinnedPost()
	posts, resp = Client.GetPinnedPosts(channel.Id, "")
	CheckNoError(t, resp)
	require.Len(t, posts.Posts, 1, "should have returned 1 pinned post")
	require.Contains(t, posts.Posts, pinnedPost.Id, "missing pinned post")

	posts, resp = Client.GetPinnedPosts(channel.Id, resp.Etag)
	CheckEtag(t, posts, resp)

	_, resp = Client.GetPinnedPosts(GenerateTestId(), "")
	CheckForbiddenStatus(t, resp)

	_, resp = Client.GetPinnedPosts("junk", "")
	CheckBadRequestStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetPinnedPosts(channel.Id, "")
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.GetPinnedPosts(channel.Id, "")
	CheckNoError(t, resp)
}

func TestUpdateChannelRoles(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	const CHANNEL_ADMIN = "channel_user channel_admin"
	const CHANNEL_MEMBER = "channel_user"

	// User 1 creates a channel, making them channel admin by default.
	channel := th.CreatePublicChannel()

	// Adds User 2 to the channel, making them a channel member by default.
	th.App.AddUserToChannel(th.BasicUser2, channel)

	// User 1 promotes User 2
	pass, resp := Client.UpdateChannelRoles(channel.Id, th.BasicUser2.Id, CHANNEL_ADMIN)
	CheckNoError(t, resp)
	require.True(t, pass, "should have passed")

	member, resp := Client.GetChannelMember(channel.Id, th.BasicUser2.Id, "")
	CheckNoError(t, resp)
	require.Equal(t, CHANNEL_ADMIN, member.Roles, "roles don't match")

	// User 1 demotes User 2
	_, resp = Client.UpdateChannelRoles(channel.Id, th.BasicUser2.Id, CHANNEL_MEMBER)
	CheckNoError(t, resp)

	th.LoginBasic2()

	// User 2 cannot demote User 1
	_, resp = Client.UpdateChannelRoles(channel.Id, th.BasicUser.Id, CHANNEL_MEMBER)
	CheckForbiddenStatus(t, resp)

	// User 2 cannot promote self
	_, resp = Client.UpdateChannelRoles(channel.Id, th.BasicUser2.Id, CHANNEL_ADMIN)
	CheckForbiddenStatus(t, resp)

	th.LoginBasic()

	// User 1 demotes self
	_, resp = Client.UpdateChannelRoles(channel.Id, th.BasicUser.Id, CHANNEL_MEMBER)
	CheckNoError(t, resp)

	// System Admin promotes User 1
	_, resp = th.SystemAdminClient.UpdateChannelRoles(channel.Id, th.BasicUser.Id, CHANNEL_ADMIN)
	CheckNoError(t, resp)

	// System Admin demotes User 1
	_, resp = th.SystemAdminClient.UpdateChannelRoles(channel.Id, th.BasicUser.Id, CHANNEL_MEMBER)
	CheckNoError(t, resp)

	// System Admin promotes User 1
	_, resp = th.SystemAdminClient.UpdateChannelRoles(channel.Id, th.BasicUser.Id, CHANNEL_ADMIN)
	CheckNoError(t, resp)

	th.LoginBasic()

	_, resp = Client.UpdateChannelRoles(channel.Id, th.BasicUser.Id, "junk")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.UpdateChannelRoles(channel.Id, "junk", CHANNEL_MEMBER)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.UpdateChannelRoles("junk", th.BasicUser.Id, CHANNEL_MEMBER)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.UpdateChannelRoles(channel.Id, model.NewId(), CHANNEL_MEMBER)
	CheckNotFoundStatus(t, resp)

	_, resp = Client.UpdateChannelRoles(model.NewId(), th.BasicUser.Id, CHANNEL_MEMBER)
	CheckForbiddenStatus(t, resp)
}

func TestUpdateChannelMemberSchemeRoles(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	SystemAdminClient := th.SystemAdminClient
	WebSocketClient, err := th.CreateWebSocketClient()
	WebSocketClient.Listen()
	require.Nil(t, err)

	th.LoginBasic()

	s1 := &model.SchemeRoles{
		SchemeAdmin: false,
		SchemeUser:  false,
		SchemeGuest: false,
	}
	_, r1 := SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.Id, th.BasicUser.Id, s1)
	CheckNoError(t, r1)

	timeout := time.After(600 * time.Millisecond)
	waiting := true
	for waiting {
		select {
		case event := <-WebSocketClient.EventChannel:
			if event.Event == model.WEBSOCKET_EVENT_CHANNEL_MEMBER_UPDATED {
				require.Equal(t, model.WEBSOCKET_EVENT_CHANNEL_MEMBER_UPDATED, event.Event)
				waiting = false
			}
		case <-timeout:
			require.Fail(t, "Should have received event channel member websocket event and not timedout")
			waiting = false
		}
	}

	tm1, rtm1 := SystemAdminClient.GetChannelMember(th.BasicChannel.Id, th.BasicUser.Id, "")
	CheckNoError(t, rtm1)
	assert.Equal(t, false, tm1.SchemeGuest)
	assert.Equal(t, false, tm1.SchemeUser)
	assert.Equal(t, false, tm1.SchemeAdmin)

	s2 := &model.SchemeRoles{
		SchemeAdmin: false,
		SchemeUser:  true,
		SchemeGuest: false,
	}
	_, r2 := SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.Id, th.BasicUser.Id, s2)
	CheckNoError(t, r2)

	tm2, rtm2 := SystemAdminClient.GetChannelMember(th.BasicChannel.Id, th.BasicUser.Id, "")
	CheckNoError(t, rtm2)
	assert.Equal(t, false, tm2.SchemeGuest)
	assert.Equal(t, true, tm2.SchemeUser)
	assert.Equal(t, false, tm2.SchemeAdmin)

	s3 := &model.SchemeRoles{
		SchemeAdmin: true,
		SchemeUser:  false,
		SchemeGuest: false,
	}
	_, r3 := SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.Id, th.BasicUser.Id, s3)
	CheckNoError(t, r3)

	tm3, rtm3 := SystemAdminClient.GetChannelMember(th.BasicChannel.Id, th.BasicUser.Id, "")
	CheckNoError(t, rtm3)
	assert.Equal(t, false, tm3.SchemeGuest)
	assert.Equal(t, false, tm3.SchemeUser)
	assert.Equal(t, true, tm3.SchemeAdmin)

	s4 := &model.SchemeRoles{
		SchemeAdmin: true,
		SchemeUser:  true,
		SchemeGuest: false,
	}
	_, r4 := SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.Id, th.BasicUser.Id, s4)
	CheckNoError(t, r4)

	tm4, rtm4 := SystemAdminClient.GetChannelMember(th.BasicChannel.Id, th.BasicUser.Id, "")
	CheckNoError(t, rtm4)
	assert.Equal(t, false, tm4.SchemeGuest)
	assert.Equal(t, true, tm4.SchemeUser)
	assert.Equal(t, true, tm4.SchemeAdmin)

	s5 := &model.SchemeRoles{
		SchemeAdmin: false,
		SchemeUser:  false,
		SchemeGuest: true,
	}
	_, r5 := SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.Id, th.BasicUser.Id, s5)
	CheckNoError(t, r5)

	tm5, rtm5 := SystemAdminClient.GetChannelMember(th.BasicChannel.Id, th.BasicUser.Id, "")
	CheckNoError(t, rtm5)
	assert.Equal(t, true, tm5.SchemeGuest)
	assert.Equal(t, false, tm5.SchemeUser)
	assert.Equal(t, false, tm5.SchemeAdmin)

	s6 := &model.SchemeRoles{
		SchemeAdmin: false,
		SchemeUser:  true,
		SchemeGuest: true,
	}
	_, resp := SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.Id, th.BasicUser.Id, s6)
	CheckBadRequestStatus(t, resp)

	_, resp = SystemAdminClient.UpdateChannelMemberSchemeRoles(model.NewId(), th.BasicUser.Id, s4)
	CheckForbiddenStatus(t, resp)

	_, resp = SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.Id, model.NewId(), s4)
	CheckNotFoundStatus(t, resp)

	_, resp = SystemAdminClient.UpdateChannelMemberSchemeRoles("ASDF", th.BasicUser.Id, s4)
	CheckBadRequestStatus(t, resp)

	_, resp = SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.Id, "ASDF", s4)
	CheckBadRequestStatus(t, resp)

	th.LoginBasic2()
	_, resp = th.Client.UpdateChannelMemberSchemeRoles(th.BasicChannel.Id, th.BasicUser.Id, s4)
	CheckForbiddenStatus(t, resp)

	SystemAdminClient.Logout()
	_, resp = SystemAdminClient.UpdateChannelMemberSchemeRoles(th.BasicChannel.Id, th.SystemAdminUser.Id, s4)
	CheckUnauthorizedStatus(t, resp)
}

func TestUpdateChannelNotifyProps(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	props := map[string]string{}
	props[model.DESKTOP_NOTIFY_PROP] = model.CHANNEL_NOTIFY_MENTION
	props[model.MARK_UNREAD_NOTIFY_PROP] = model.CHANNEL_MARK_UNREAD_MENTION

	pass, resp := Client.UpdateChannelNotifyProps(th.BasicChannel.Id, th.BasicUser.Id, props)
	CheckNoError(t, resp)
	require.True(t, pass, "should have passed")

	member, err := th.App.GetChannelMember(th.BasicChannel.Id, th.BasicUser.Id)
	require.Nil(t, err)
	require.Equal(t, model.CHANNEL_NOTIFY_MENTION, member.NotifyProps[model.DESKTOP_NOTIFY_PROP], "bad update")
	require.Equal(t, model.CHANNEL_MARK_UNREAD_MENTION, member.NotifyProps[model.MARK_UNREAD_NOTIFY_PROP], "bad update")

	_, resp = Client.UpdateChannelNotifyProps("junk", th.BasicUser.Id, props)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.UpdateChannelNotifyProps(th.BasicChannel.Id, "junk", props)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.UpdateChannelNotifyProps(model.NewId(), th.BasicUser.Id, props)
	CheckNotFoundStatus(t, resp)

	_, resp = Client.UpdateChannelNotifyProps(th.BasicChannel.Id, model.NewId(), props)
	CheckForbiddenStatus(t, resp)

	_, resp = Client.UpdateChannelNotifyProps(th.BasicChannel.Id, th.BasicUser.Id, map[string]string{})
	CheckNoError(t, resp)

	Client.Logout()
	_, resp = Client.UpdateChannelNotifyProps(th.BasicChannel.Id, th.BasicUser.Id, props)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.UpdateChannelNotifyProps(th.BasicChannel.Id, th.BasicUser.Id, props)
	CheckNoError(t, resp)
}

func TestAddChannelMember(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user := th.BasicUser
	user2 := th.BasicUser2
	team := th.BasicTeam
	publicChannel := th.CreatePublicChannel()
	privateChannel := th.CreatePrivateChannel()

	user3 := th.CreateUserWithClient(th.SystemAdminClient)
	_, resp := th.SystemAdminClient.AddTeamMember(team.Id, user3.Id)
	CheckNoError(t, resp)

	cm, resp := Client.AddChannelMember(publicChannel.Id, user2.Id)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)
	require.Equal(t, publicChannel.Id, cm.ChannelId, "should have returned exact channel")
	require.Equal(t, user2.Id, cm.UserId, "should have returned exact user added to public channel")

	cm, resp = Client.AddChannelMember(privateChannel.Id, user2.Id)
	CheckNoError(t, resp)
	require.Equal(t, privateChannel.Id, cm.ChannelId, "should have returned exact channel")
	require.Equal(t, user2.Id, cm.UserId, "should have returned exact user added to private channel")

	post := &model.Post{ChannelId: publicChannel.Id, Message: "a" + GenerateTestId() + "a"}
	rpost, err := Client.CreatePost(post)
	require.NotNil(t, err)

	Client.RemoveUserFromChannel(publicChannel.Id, user.Id)
	_, resp = Client.AddChannelMemberWithRootId(publicChannel.Id, user.Id, rpost.Id)
	CheckNoError(t, resp)
	CheckCreatedStatus(t, resp)

	Client.RemoveUserFromChannel(publicChannel.Id, user.Id)
	_, resp = Client.AddChannelMemberWithRootId(publicChannel.Id, user.Id, "junk")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.AddChannelMemberWithRootId(publicChannel.Id, user.Id, GenerateTestId())
	CheckNotFoundStatus(t, resp)

	Client.RemoveUserFromChannel(publicChannel.Id, user.Id)
	_, resp = Client.AddChannelMember(publicChannel.Id, user.Id)
	CheckNoError(t, resp)

	cm, resp = Client.AddChannelMember(publicChannel.Id, "junk")
	CheckBadRequestStatus(t, resp)
	require.Nil(t, cm, "should return nothing")

	_, resp = Client.AddChannelMember(publicChannel.Id, GenerateTestId())
	CheckNotFoundStatus(t, resp)

	_, resp = Client.AddChannelMember("junk", user2.Id)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.AddChannelMember(GenerateTestId(), user2.Id)
	CheckNotFoundStatus(t, resp)

	otherUser := th.CreateUser()
	otherChannel := th.CreatePublicChannel()
	Client.Logout()
	Client.Login(user2.Id, user2.Password)

	_, resp = Client.AddChannelMember(publicChannel.Id, otherUser.Id)
	CheckUnauthorizedStatus(t, resp)

	_, resp = Client.AddChannelMember(privateChannel.Id, otherUser.Id)
	CheckUnauthorizedStatus(t, resp)

	_, resp = Client.AddChannelMember(otherChannel.Id, otherUser.Id)
	CheckUnauthorizedStatus(t, resp)

	Client.Logout()
	Client.Login(user.Id, user.Password)

	// should fail adding user who is not a member of the team
	_, resp = Client.AddChannelMember(otherChannel.Id, otherUser.Id)
	CheckUnauthorizedStatus(t, resp)

	Client.DeleteChannel(otherChannel.Id)

	// should fail adding user to a deleted channel
	_, resp = Client.AddChannelMember(otherChannel.Id, user2.Id)
	CheckUnauthorizedStatus(t, resp)

	Client.Logout()
	_, resp = Client.AddChannelMember(publicChannel.Id, user2.Id)
	CheckUnauthorizedStatus(t, resp)

	_, resp = Client.AddChannelMember(privateChannel.Id, user2.Id)
	CheckUnauthorizedStatus(t, resp)

	_, resp = th.SystemAdminClient.AddChannelMember(publicChannel.Id, user2.Id)
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.AddChannelMember(privateChannel.Id, user2.Id)
	CheckNoError(t, resp)

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()

	th.AddPermissionToRole(model.PERMISSION_MANAGE_PRIVATE_CHANNEL_MEMBERS.Id, model.CHANNEL_USER_ROLE_ID)

	// Check that a regular channel user can add other users.
	Client.Login(user2.Username, user2.Password)
	privateChannel = th.CreatePrivateChannel()
	_, resp = Client.AddChannelMember(privateChannel.Id, user.Id)
	CheckNoError(t, resp)
	Client.Logout()

	Client.Login(user.Username, user.Password)
	_, resp = Client.AddChannelMember(privateChannel.Id, user3.Id)
	CheckNoError(t, resp)
	Client.Logout()

	// Restrict the permission for adding users to Channel Admins
	th.AddPermissionToRole(model.PERMISSION_MANAGE_PRIVATE_CHANNEL_MEMBERS.Id, model.CHANNEL_ADMIN_ROLE_ID)
	th.RemovePermissionFromRole(model.PERMISSION_MANAGE_PRIVATE_CHANNEL_MEMBERS.Id, model.CHANNEL_USER_ROLE_ID)

	Client.Login(user2.Username, user2.Password)
	privateChannel = th.CreatePrivateChannel()
	_, resp = Client.AddChannelMember(privateChannel.Id, user.Id)
	CheckNoError(t, resp)
	Client.Logout()

	Client.Login(user.Username, user.Password)
	_, resp = Client.AddChannelMember(privateChannel.Id, user3.Id)
	CheckForbiddenStatus(t, resp)
	Client.Logout()

	th.MakeUserChannelAdmin(user, privateChannel)
	th.App.InvalidateAllCaches()

	Client.Login(user.Username, user.Password)
	_, resp = Client.AddChannelMember(privateChannel.Id, user3.Id)
	CheckNoError(t, resp)
	Client.Logout()

	// Set a channel to group-constrained
	privateChannel.GroupConstrained = model.NewBool(true)
	_, appErr := th.App.UpdateChannel(privateChannel)
	require.Nil(t, appErr)

	// User is not in associated groups so shouldn't be allowed
	_, resp = th.SystemAdminClient.AddChannelMember(privateChannel.Id, user.Id)
	CheckErrorMessage(t, resp, "api.channel.add_members.user_denied")

	// Associate group to team
	_, appErr = th.App.UpsertGroupSyncable(&model.GroupSyncable{
		GroupId:    th.Group.Id,
		SyncableId: privateChannel.Id,
		Type:       model.GroupSyncableTypeChannel,
	})
	require.Nil(t, appErr)

	// Add user to group
	_, appErr = th.App.UpsertGroupMember(th.Group.Id, user.Id)
	require.Nil(t, appErr)

	_, resp = th.SystemAdminClient.AddChannelMember(privateChannel.Id, user.Id)
	CheckNoError(t, resp)
}

func TestAddChannelMemberAddMyself(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	user := th.CreateUser()
	th.LinkUserToTeam(user, th.BasicTeam)
	notMemberPublicChannel1 := th.CreatePublicChannel()
	notMemberPublicChannel2 := th.CreatePublicChannel()
	notMemberPrivateChannel := th.CreatePrivateChannel()

	memberPublicChannel := th.CreatePublicChannel()
	memberPrivateChannel := th.CreatePrivateChannel()
	th.AddUserToChannel(user, memberPublicChannel)
	th.AddUserToChannel(user, memberPrivateChannel)

	testCases := []struct {
		Name                     string
		Channel                  *model.Channel
		WithJoinPublicPermission bool
		ExpectedError            string
	}{
		{
			"Add myself to a public channel with JOIN_PUBLIC_CHANNEL permission",
			notMemberPublicChannel1,
			true,
			"",
		},
		{
			"Try to add myself to a private channel with the JOIN_PUBLIC_CHANNEL permission",
			notMemberPrivateChannel,
			true,
			"api.context.permissions.app_error",
		},
		{
			"Try to add myself to a public channel without the JOIN_PUBLIC_CHANNEL permission",
			notMemberPublicChannel2,
			false,
			"api.context.permissions.app_error",
		},
		{
			"Add myself a public channel where I'm already a member, not having JOIN_PUBLIC_CHANNEL or MANAGE MEMBERS permission",
			memberPublicChannel,
			false,
			"",
		},
		{
			"Add myself a private channel where I'm already a member, not having JOIN_PUBLIC_CHANNEL or MANAGE MEMBERS permission",
			memberPrivateChannel,
			false,
			"",
		},
	}
	Client.Login(user.Email, user.Password)
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {

			// Check the appropriate permissions are enforced.
			defaultRolePermissions := th.SaveDefaultRolePermissions()
			defer func() {
				th.RestoreDefaultRolePermissions(defaultRolePermissions)
			}()

			if !tc.WithJoinPublicPermission {
				th.RemovePermissionFromRole(model.PERMISSION_JOIN_PUBLIC_CHANNELS.Id, model.TEAM_USER_ROLE_ID)
			}

			_, resp := Client.AddChannelMember(tc.Channel.Id, user.Id)
			if tc.ExpectedError == "" {
				CheckNoError(t, resp)
			} else {
				CheckErrorMessage(t, resp, tc.ExpectedError)
			}
		})
	}
}

func TestRemoveChannelMember(t *testing.T) {
	th := Setup(t).InitBasic()
	user1 := th.BasicUser
	user2 := th.BasicUser2
	team := th.BasicTeam
	defer th.TearDown()
	Client := th.Client

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableBotAccountCreation = true
	})
	bot := th.CreateBotWithSystemAdminClient()
	th.App.AddUserToTeam(team.Id, bot.UserId, "")

	pass, resp := Client.RemoveUserFromChannel(th.BasicChannel.Id, th.BasicUser2.Id)
	CheckNoError(t, resp)
	require.True(t, pass, "should have passed")

	_, resp = Client.RemoveUserFromChannel(th.BasicChannel.Id, "junk")
	CheckBadRequestStatus(t, resp)

	_, resp = Client.RemoveUserFromChannel(th.BasicChannel.Id, model.NewId())
	CheckNotFoundStatus(t, resp)

	_, resp = Client.RemoveUserFromChannel(model.NewId(), th.BasicUser2.Id)
	CheckNotFoundStatus(t, resp)

	th.LoginBasic2()
	_, resp = Client.RemoveUserFromChannel(th.BasicChannel.Id, th.BasicUser.Id)
	CheckForbiddenStatus(t, resp)

	t.Run("success", func(t *testing.T) {
		// Setup the system administrator to listen for websocket events from the channels.
		th.LinkUserToTeam(th.SystemAdminUser, th.BasicTeam)
		_, err := th.App.AddUserToChannel(th.SystemAdminUser, th.BasicChannel)
		require.Nil(t, err)
		_, err = th.App.AddUserToChannel(th.SystemAdminUser, th.BasicChannel2)
		require.Nil(t, err)
		props := map[string]string{}
		props[model.DESKTOP_NOTIFY_PROP] = model.CHANNEL_NOTIFY_ALL
		_, resp = th.SystemAdminClient.UpdateChannelNotifyProps(th.BasicChannel.Id, th.SystemAdminUser.Id, props)
		_, resp = th.SystemAdminClient.UpdateChannelNotifyProps(th.BasicChannel2.Id, th.SystemAdminUser.Id, props)
		CheckNoError(t, resp)

		wsClient, err := th.CreateWebSocketSystemAdminClient()
		require.Nil(t, err)
		wsClient.Listen()
		var closeWsClient sync.Once
		defer closeWsClient.Do(func() {
			wsClient.Close()
		})

		wsr := <-wsClient.EventChannel
		require.Equal(t, model.WEBSOCKET_EVENT_HELLO, wsr.EventType())

		// requirePost listens for websocket events and tries to find the post matching
		// the expected post's channel and message.
		requirePost := func(expectedPost *model.Post) {
			t.Helper()
			for {
				select {
				case event := <-wsClient.EventChannel:
					postData, ok := event.GetData()["post"]
					if !ok {
						continue
					}

					post := model.PostFromJson(strings.NewReader(postData.(string)))
					if post.ChannelId == expectedPost.ChannelId && post.Message == expectedPost.Message {
						return
					}
				case <-time.After(5 * time.Second):
					require.FailNow(t, "failed to find expected post after 5 seconds")
					return
				}
			}
		}

		th.App.AddUserToChannel(th.BasicUser2, th.BasicChannel)
		_, resp = Client.RemoveUserFromChannel(th.BasicChannel.Id, th.BasicUser2.Id)
		CheckNoError(t, resp)

		requirePost(&model.Post{
			Message:   fmt.Sprintf("@%s left the channel.", th.BasicUser2.Username),
			ChannelId: th.BasicChannel.Id,
		})

		_, resp = Client.RemoveUserFromChannel(th.BasicChannel2.Id, th.BasicUser.Id)
		CheckNoError(t, resp)
		requirePost(&model.Post{
			Message:   fmt.Sprintf("@%s removed from the channel.", th.BasicUser.Username),
			ChannelId: th.BasicChannel2.Id,
		})

		_, resp = th.SystemAdminClient.RemoveUserFromChannel(th.BasicChannel.Id, th.BasicUser.Id)
		CheckNoError(t, resp)
		requirePost(&model.Post{
			Message:   fmt.Sprintf("@%s removed from the channel.", th.BasicUser.Username),
			ChannelId: th.BasicChannel.Id,
		})

		closeWsClient.Do(func() {
			wsClient.Close()
		})
	})

	// Leave deleted channel
	th.LoginBasic()
	deletedChannel := th.CreatePublicChannel()
	th.App.AddUserToChannel(th.BasicUser, deletedChannel)
	th.App.AddUserToChannel(th.BasicUser2, deletedChannel)

	deletedChannel.DeleteAt = 1
	th.App.UpdateChannel(deletedChannel)

	_, resp = Client.RemoveUserFromChannel(deletedChannel.Id, th.BasicUser.Id)
	CheckNoError(t, resp)

	th.LoginBasic()
	private := th.CreatePrivateChannel()
	th.App.AddUserToChannel(th.BasicUser2, private)

	_, resp = Client.RemoveUserFromChannel(private.Id, th.BasicUser2.Id)
	CheckNoError(t, resp)

	th.LoginBasic2()
	_, resp = Client.RemoveUserFromChannel(private.Id, th.BasicUser.Id)
	CheckForbiddenStatus(t, resp)

	_, resp = th.SystemAdminClient.RemoveUserFromChannel(private.Id, th.BasicUser.Id)
	CheckNoError(t, resp)

	th.LoginBasic()
	th.UpdateUserToNonTeamAdmin(user1, team)
	th.App.InvalidateAllCaches()

	// Check the appropriate permissions are enforced.
	defaultRolePermissions := th.SaveDefaultRolePermissions()
	defer func() {
		th.RestoreDefaultRolePermissions(defaultRolePermissions)
	}()

	th.AddPermissionToRole(model.PERMISSION_MANAGE_PRIVATE_CHANNEL_MEMBERS.Id, model.CHANNEL_USER_ROLE_ID)

	// Check that a regular channel user can remove other users.
	privateChannel := th.CreateChannelWithClient(th.SystemAdminClient, model.CHANNEL_PRIVATE)
	_, resp = th.SystemAdminClient.AddChannelMember(privateChannel.Id, user1.Id)
	CheckNoError(t, resp)
	_, resp = th.SystemAdminClient.AddChannelMember(privateChannel.Id, user2.Id)
	CheckNoError(t, resp)

	_, resp = Client.RemoveUserFromChannel(privateChannel.Id, user2.Id)
	CheckNoError(t, resp)

	// Restrict the permission for adding users to Channel Admins
	th.AddPermissionToRole(model.PERMISSION_MANAGE_PRIVATE_CHANNEL_MEMBERS.Id, model.CHANNEL_ADMIN_ROLE_ID)
	th.RemovePermissionFromRole(model.PERMISSION_MANAGE_PRIVATE_CHANNEL_MEMBERS.Id, model.CHANNEL_USER_ROLE_ID)

	privateChannel = th.CreateChannelWithClient(th.SystemAdminClient, model.CHANNEL_PRIVATE)
	_, resp = th.SystemAdminClient.AddChannelMember(privateChannel.Id, user1.Id)
	CheckNoError(t, resp)
	_, resp = th.SystemAdminClient.AddChannelMember(privateChannel.Id, user2.Id)
	CheckNoError(t, resp)
	_, resp = th.SystemAdminClient.AddChannelMember(privateChannel.Id, bot.UserId)
	CheckNoError(t, resp)

	_, resp = Client.RemoveUserFromChannel(privateChannel.Id, user2.Id)
	CheckForbiddenStatus(t, resp)

	th.MakeUserChannelAdmin(user1, privateChannel)
	th.App.InvalidateAllCaches()

	_, resp = Client.RemoveUserFromChannel(privateChannel.Id, user2.Id)
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.AddChannelMember(privateChannel.Id, th.SystemAdminUser.Id)
	CheckNoError(t, resp)

	// If the channel is group-constrained the user cannot be removed
	privateChannel.GroupConstrained = model.NewBool(true)
	_, err := th.App.UpdateChannel(privateChannel)
	require.Nil(t, err)
	_, resp = Client.RemoveUserFromChannel(privateChannel.Id, user2.Id)
	require.Equal(t, "api.channel.remove_member.group_constrained.app_error", resp.Error.Id)

	// If the channel is group-constrained user can remove self
	_, resp = th.SystemAdminClient.RemoveUserFromChannel(privateChannel.Id, th.SystemAdminUser.Id)
	CheckNoError(t, resp)

	// Test on preventing removal of user from a direct channel
	directChannel, resp := Client.CreateDirectChannel(user1.Id, user2.Id)
	CheckNoError(t, resp)

	// If the channel is group-constrained a user can remove a bot
	_, resp = Client.RemoveUserFromChannel(privateChannel.Id, bot.UserId)
	CheckNoError(t, resp)

	_, resp = Client.RemoveUserFromChannel(directChannel.Id, user1.Id)
	CheckBadRequestStatus(t, resp)

	_, resp = Client.RemoveUserFromChannel(directChannel.Id, user2.Id)
	CheckBadRequestStatus(t, resp)

	_, resp = th.SystemAdminClient.RemoveUserFromChannel(directChannel.Id, user1.Id)
	CheckBadRequestStatus(t, resp)

	// Test on preventing removal of user from a group channel
	user3 := th.CreateUser()
	groupChannel, resp := Client.CreateGroupChannel([]string{user1.Id, user2.Id, user3.Id})
	CheckNoError(t, resp)

	_, resp = Client.RemoveUserFromChannel(groupChannel.Id, user1.Id)
	CheckBadRequestStatus(t, resp)

	_, resp = th.SystemAdminClient.RemoveUserFromChannel(groupChannel.Id, user1.Id)
	CheckBadRequestStatus(t, resp)
}

func TestAutocompleteChannels(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// A private channel to make sure private channels are not used
	utils.DisableDebugLogForTest()
	ptown, _ := th.Client.CreateChannel(&model.Channel{
		DisplayName: "Town",
		Name:        "town",
		Type:        model.CHANNEL_PRIVATE,
		TeamId:      th.BasicTeam.Id,
	})
	tower, _ := th.Client.CreateChannel(&model.Channel{
		DisplayName: "Tower",
		Name:        "tower",
		Type:        model.CHANNEL_OPEN,
		TeamId:      th.BasicTeam.Id,
	})
	utils.EnableDebugLogForTest()
	defer func() {
		th.Client.DeleteChannel(ptown.Id)
		th.Client.DeleteChannel(tower.Id)
	}()

	for _, tc := range []struct {
		description      string
		teamId           string
		fragment         string
		expectedIncludes []string
		expectedExcludes []string
	}{
		{
			"Basic town-square",
			th.BasicTeam.Id,
			"town",
			[]string{"town-square"},
			[]string{"off-topic", "town", "tower"},
		},
		{
			"Basic off-topic",
			th.BasicTeam.Id,
			"off-to",
			[]string{"off-topic"},
			[]string{"town-square", "town", "tower"},
		},
		{
			"Basic town square and off topic",
			th.BasicTeam.Id,
			"tow",
			[]string{"town-square", "tower"},
			[]string{"off-topic", "town"},
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
			channels, resp := th.Client.AutocompleteChannelsForTeam(tc.teamId, tc.fragment)
			require.Nil(t, resp.Error)
			names := make([]string, len(*channels))
			for i, c := range *channels {
				names[i] = c.Name
			}
			for _, name := range tc.expectedIncludes {
				require.Contains(t, names, name, "channel not included")
			}
			for _, name := range tc.expectedExcludes {
				require.NotContains(t, names, name, "channel not excluded")
			}
		})
	}
}

func TestAutocompleteChannelsForSearch(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.LoginSystemAdminWithClient(th.SystemAdminClient)
	th.LoginBasicWithClient(th.Client)

	u1 := th.CreateUserWithClient(th.SystemAdminClient)
	u2 := th.CreateUserWithClient(th.SystemAdminClient)
	u3 := th.CreateUserWithClient(th.SystemAdminClient)
	u4 := th.CreateUserWithClient(th.SystemAdminClient)

	// A private channel to make sure private channels are not used
	utils.DisableDebugLogForTest()
	ptown, _ := th.SystemAdminClient.CreateChannel(&model.Channel{
		DisplayName: "Town",
		Name:        "town",
		Type:        model.CHANNEL_PRIVATE,
		TeamId:      th.BasicTeam.Id,
	})
	defer func() {
		th.Client.DeleteChannel(ptown.Id)
	}()
	mypriv, _ := th.Client.CreateChannel(&model.Channel{
		DisplayName: "My private town",
		Name:        "townpriv",
		Type:        model.CHANNEL_PRIVATE,
		TeamId:      th.BasicTeam.Id,
	})
	defer func() {
		th.Client.DeleteChannel(mypriv.Id)
	}()
	utils.EnableDebugLogForTest()

	dc1, resp := th.Client.CreateDirectChannel(th.BasicUser.Id, u1.Id)
	CheckNoError(t, resp)
	defer func() {
		th.Client.DeleteChannel(dc1.Id)
	}()

	dc2, resp := th.SystemAdminClient.CreateDirectChannel(u2.Id, u3.Id)
	CheckNoError(t, resp)
	defer func() {
		th.SystemAdminClient.DeleteChannel(dc2.Id)
	}()

	gc1, resp := th.Client.CreateGroupChannel([]string{th.BasicUser.Id, u2.Id, u3.Id})
	CheckNoError(t, resp)
	defer func() {
		th.Client.DeleteChannel(gc1.Id)
	}()

	gc2, resp := th.SystemAdminClient.CreateGroupChannel([]string{u2.Id, u3.Id, u4.Id})
	CheckNoError(t, resp)
	defer func() {
		th.SystemAdminClient.DeleteChannel(gc2.Id)
	}()

	for _, tc := range []struct {
		description      string
		teamId           string
		fragment         string
		expectedIncludes []string
		expectedExcludes []string
	}{
		{
			"Basic town-square",
			th.BasicTeam.Id,
			"town",
			[]string{"town-square", "townpriv"},
			[]string{"off-topic", "town"},
		},
		{
			"Basic off-topic",
			th.BasicTeam.Id,
			"off-to",
			[]string{"off-topic"},
			[]string{"town-square", "town", "townpriv"},
		},
		{
			"Basic town square and townpriv",
			th.BasicTeam.Id,
			"tow",
			[]string{"town-square", "townpriv"},
			[]string{"off-topic", "town"},
		},
		{
			"Direct and group messages",
			th.BasicTeam.Id,
			"fakeuser",
			[]string{dc1.Name, gc1.Name},
			[]string{dc2.Name, gc2.Name},
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
			channels, resp := th.Client.AutocompleteChannelsForTeamForSearch(tc.teamId, tc.fragment)
			require.Nil(t, resp.Error)
			names := make([]string, len(*channels))
			for i, c := range *channels {
				names[i] = c.Name
			}
			for _, name := range tc.expectedIncludes {
				require.Contains(t, names, name, "channel not included")
			}
			for _, name := range tc.expectedExcludes {
				require.NotContains(t, names, name, "channel not excluded")
			}
		})
	}
}

func TestUpdateChannelScheme(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.SetLicense(model.NewTestLicense(""))

	th.App.SetPhase2PermissionsMigrationStatus(true)

	team, resp := th.SystemAdminClient.CreateTeam(&model.Team{
		DisplayName:     "Name",
		Description:     "Some description",
		CompanyName:     "Some company name",
		AllowOpenInvite: false,
		InviteId:        "inviteid0",
		Name:            "z-z-" + model.NewId() + "a",
		Email:           "success+" + model.NewId() + "@simulator.amazonses.com",
		Type:            model.TEAM_OPEN,
	})
	CheckNoError(t, resp)

	channel, resp := th.SystemAdminClient.CreateChannel(&model.Channel{
		DisplayName: "Name",
		Name:        "z-z-" + model.NewId() + "a",
		Type:        model.CHANNEL_OPEN,
		TeamId:      team.Id,
	})
	CheckNoError(t, resp)

	channelScheme, resp := th.SystemAdminClient.CreateScheme(&model.Scheme{
		DisplayName: "DisplayName",
		Name:        model.NewId(),
		Description: "Some description",
		Scope:       model.SCHEME_SCOPE_CHANNEL,
	})
	CheckNoError(t, resp)

	teamScheme, resp := th.SystemAdminClient.CreateScheme(&model.Scheme{
		DisplayName: "DisplayName",
		Name:        model.NewId(),
		Description: "Some description",
		Scope:       model.SCHEME_SCOPE_TEAM,
	})
	CheckNoError(t, resp)

	// Test the setup/base case.
	_, resp = th.SystemAdminClient.UpdateChannelScheme(channel.Id, channelScheme.Id)
	CheckNoError(t, resp)

	// Test various invalid channel and scheme id combinations.
	_, resp = th.SystemAdminClient.UpdateChannelScheme(channel.Id, "x")
	CheckBadRequestStatus(t, resp)
	_, resp = th.SystemAdminClient.UpdateChannelScheme("x", channelScheme.Id)
	CheckBadRequestStatus(t, resp)
	_, resp = th.SystemAdminClient.UpdateChannelScheme("x", "x")
	CheckBadRequestStatus(t, resp)

	// Test that permissions are required.
	_, resp = th.Client.UpdateChannelScheme(channel.Id, channelScheme.Id)
	CheckForbiddenStatus(t, resp)

	// Test that a license is required.
	th.App.SetLicense(nil)
	_, resp = th.SystemAdminClient.UpdateChannelScheme(channel.Id, channelScheme.Id)
	CheckNotImplementedStatus(t, resp)
	th.App.SetLicense(model.NewTestLicense(""))

	// Test an invalid scheme scope.
	_, resp = th.SystemAdminClient.UpdateChannelScheme(channel.Id, teamScheme.Id)
	CheckBadRequestStatus(t, resp)

	// Test that an unauthenticated user gets rejected.
	th.SystemAdminClient.Logout()
	_, resp = th.SystemAdminClient.UpdateChannelScheme(channel.Id, channelScheme.Id)
	CheckUnauthorizedStatus(t, resp)
}

func TestGetChannelMembersTimezones(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	user := th.BasicUser
	user.Timezone["useAutomaticTimezone"] = "false"
	user.Timezone["manualTimezone"] = "XOXO/BLABLA"
	_, resp := Client.UpdateUser(user)
	CheckNoError(t, resp)

	user2 := th.BasicUser2
	user2.Timezone["automaticTimezone"] = "NoWhere/Island"
	_, resp = th.SystemAdminClient.UpdateUser(user2)
	CheckNoError(t, resp)

	timezone, resp := Client.GetChannelMembersTimezones(th.BasicChannel.Id)
	CheckNoError(t, resp)
	require.Len(t, timezone, 2, "should return 2 timezones")

	//both users have same timezone
	user2.Timezone["automaticTimezone"] = "XOXO/BLABLA"
	_, resp = th.SystemAdminClient.UpdateUser(user2)
	CheckNoError(t, resp)

	timezone, resp = Client.GetChannelMembersTimezones(th.BasicChannel.Id)
	CheckNoError(t, resp)
	require.Len(t, timezone, 1, "should return 1 timezone")

	//no timezone set should return empty
	user2.Timezone["automaticTimezone"] = ""
	_, resp = th.SystemAdminClient.UpdateUser(user2)
	CheckNoError(t, resp)

	user.Timezone["manualTimezone"] = ""
	_, resp = Client.UpdateUser(user)
	CheckNoError(t, resp)

	timezone, resp = Client.GetChannelMembersTimezones(th.BasicChannel.Id)
	CheckNoError(t, resp)
	require.Empty(t, timezone, "should return 0 timezone")
}

func TestChannelMembersMinusGroupMembers(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user1 := th.BasicUser
	user2 := th.BasicUser2

	channel := th.CreatePrivateChannel()

	_, err := th.App.AddChannelMember(user1.Id, channel, "", "")
	require.Nil(t, err)
	_, err = th.App.AddChannelMember(user2.Id, channel, "", "")
	require.Nil(t, err)

	channel.GroupConstrained = model.NewBool(true)
	channel, err = th.App.UpdateChannel(channel)
	require.Nil(t, err)

	group1 := th.CreateGroup()
	group2 := th.CreateGroup()

	_, err = th.App.UpsertGroupMember(group1.Id, user1.Id)
	require.Nil(t, err)
	_, err = th.App.UpsertGroupMember(group2.Id, user2.Id)
	require.Nil(t, err)

	// No permissions
	_, _, res := th.Client.ChannelMembersMinusGroupMembers(channel.Id, []string{group1.Id, group2.Id}, 0, 100, "")
	require.Equal(t, "api.context.permissions.app_error", res.Error.Id)

	testCases := map[string]struct {
		groupIDs        []string
		page            int
		perPage         int
		length          int
		count           int
		otherAssertions func([]*model.UserWithGroups)
	}{
		"All groups, expect no users removed": {
			groupIDs: []string{group1.Id, group2.Id},
			page:     0,
			perPage:  100,
			length:   0,
			count:    0,
		},
		"Some nonexistent group, page 0": {
			groupIDs: []string{model.NewId()},
			page:     0,
			perPage:  1,
			length:   1,
			count:    2,
		},
		"Some nonexistent group, page 1": {
			groupIDs: []string{model.NewId()},
			page:     1,
			perPage:  1,
			length:   1,
			count:    2,
		},
		"One group, expect one user removed": {
			groupIDs: []string{group1.Id},
			page:     0,
			perPage:  100,
			length:   1,
			count:    1,
			otherAssertions: func(uwg []*model.UserWithGroups) {
				require.Equal(t, uwg[0].Id, user2.Id)
			},
		},
		"Other group, expect other user removed": {
			groupIDs: []string{group2.Id},
			page:     0,
			perPage:  100,
			length:   1,
			count:    1,
			otherAssertions: func(uwg []*model.UserWithGroups) {
				require.Equal(t, uwg[0].Id, user1.Id)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			uwg, count, res := th.SystemAdminClient.ChannelMembersMinusGroupMembers(channel.Id, tc.groupIDs, tc.page, tc.perPage, "")
			require.Nil(t, res.Error)
			require.Len(t, uwg, tc.length)
			require.Equal(t, tc.count, int(count))
			if tc.otherAssertions != nil {
				tc.otherAssertions(uwg)
			}
		})
	}
}

func TestGetChannelModerations(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channel := th.BasicChannel
	team := th.BasicTeam

	th.App.SetPhase2PermissionsMigrationStatus(true)

	t.Run("Errors without a license", func(t *testing.T) {
		_, res := th.SystemAdminClient.GetChannelModerations(channel.Id, "")
		require.Equal(t, "api.channel.get_channel_moderations.license.error", res.Error.Id)
	})

	th.App.SetLicense(model.NewTestLicense())

	t.Run("Errors as a non sysadmin", func(t *testing.T) {
		_, res := th.Client.GetChannelModerations(channel.Id, "")
		require.Equal(t, "api.context.permissions.app_error", res.Error.Id)
	})

	th.App.SetLicense(model.NewTestLicense())

	t.Run("Returns default moderations with default roles", func(t *testing.T) {
		moderations, res := th.SystemAdminClient.GetChannelModerations(channel.Id, "")
		require.Nil(t, res.Error)
		require.Equal(t, len(moderations), 4)
		for _, moderation := range moderations {
			if moderation.Name == "manage_members" {
				require.Empty(t, moderation.Roles.Guests)
			} else {
				require.Equal(t, moderation.Roles.Guests.Value, true)
				require.Equal(t, moderation.Roles.Guests.Enabled, true)
			}

			require.Equal(t, moderation.Roles.Members.Value, true)
			require.Equal(t, moderation.Roles.Members.Enabled, true)
		}
	})

	t.Run("Returns value false and enabled false for permissions that are not present in higher scoped scheme when no channel scheme present", func(t *testing.T) {
		scheme := th.SetupTeamScheme()
		team.SchemeId = &scheme.Id
		_, err := th.App.UpdateTeamScheme(team)
		require.Nil(t, err)

		th.RemovePermissionFromRole(model.PERMISSION_CREATE_POST.Id, scheme.DefaultChannelGuestRole)
		defer th.AddPermissionToRole(model.PERMISSION_CREATE_POST.Id, scheme.DefaultChannelGuestRole)

		moderations, res := th.SystemAdminClient.GetChannelModerations(channel.Id, "")
		require.Nil(t, res.Error)
		for _, moderation := range moderations {
			if moderation.Name == model.PERMISSION_CREATE_POST.Id {
				require.Equal(t, moderation.Roles.Members.Value, true)
				require.Equal(t, moderation.Roles.Members.Enabled, true)
				require.Equal(t, moderation.Roles.Guests.Value, false)
				require.Equal(t, moderation.Roles.Guests.Enabled, false)
			}
		}
	})

	t.Run("Returns value false and enabled true for permissions that are not present in channel scheme but present in team scheme", func(t *testing.T) {
		scheme := th.SetupChannelScheme()
		channel.SchemeId = &scheme.Id
		_, err := th.App.UpdateChannelScheme(channel)
		require.Nil(t, err)

		th.RemovePermissionFromRole(model.PERMISSION_CREATE_POST.Id, scheme.DefaultChannelGuestRole)
		defer th.AddPermissionToRole(model.PERMISSION_CREATE_POST.Id, scheme.DefaultChannelGuestRole)

		moderations, res := th.SystemAdminClient.GetChannelModerations(channel.Id, "")
		require.Nil(t, res.Error)
		for _, moderation := range moderations {
			if moderation.Name == model.PERMISSION_CREATE_POST.Id {
				require.Equal(t, moderation.Roles.Members.Value, true)
				require.Equal(t, moderation.Roles.Members.Enabled, true)
				require.Equal(t, moderation.Roles.Guests.Value, false)
				require.Equal(t, moderation.Roles.Guests.Enabled, true)
			}
		}
	})

	t.Run("Returns value false and enabled false for permissions that are not present in channel & team scheme", func(t *testing.T) {
		teamScheme := th.SetupTeamScheme()
		team.SchemeId = &teamScheme.Id
		th.App.UpdateTeamScheme(team)

		scheme := th.SetupChannelScheme()
		channel.SchemeId = &scheme.Id
		th.App.UpdateChannelScheme(channel)

		th.RemovePermissionFromRole(model.PERMISSION_CREATE_POST.Id, scheme.DefaultChannelGuestRole)
		th.RemovePermissionFromRole(model.PERMISSION_CREATE_POST.Id, teamScheme.DefaultChannelGuestRole)

		defer th.AddPermissionToRole(model.PERMISSION_CREATE_POST.Id, scheme.DefaultChannelGuestRole)
		defer th.AddPermissionToRole(model.PERMISSION_CREATE_POST.Id, teamScheme.DefaultChannelGuestRole)

		moderations, res := th.SystemAdminClient.GetChannelModerations(channel.Id, "")
		require.Nil(t, res.Error)
		for _, moderation := range moderations {
			if moderation.Name == model.PERMISSION_CREATE_POST.Id {
				require.Equal(t, moderation.Roles.Members.Value, true)
				require.Equal(t, moderation.Roles.Members.Enabled, true)
				require.Equal(t, moderation.Roles.Guests.Value, false)
				require.Equal(t, moderation.Roles.Guests.Enabled, false)
			}
		}
	})
}

func TestPatchChannelModerations(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	channel := th.BasicChannel

	emptyPatch := []*model.ChannelModerationPatch{}

	createPosts := model.CHANNEL_MODERATED_PERMISSIONS[0]

	th.App.SetPhase2PermissionsMigrationStatus(true)

	t.Run("Errors without a license", func(t *testing.T) {
		_, res := th.SystemAdminClient.PatchChannelModerations(channel.Id, emptyPatch)
		require.Equal(t, "api.channel.patch_channel_moderations.license.error", res.Error.Id)
	})

	th.App.SetLicense(model.NewTestLicense())

	t.Run("Errors as a non sysadmin", func(t *testing.T) {
		_, res := th.Client.PatchChannelModerations(channel.Id, emptyPatch)
		require.Equal(t, "api.context.permissions.app_error", res.Error.Id)
	})

	th.App.SetLicense(model.NewTestLicense())

	t.Run("Returns default moderations with empty patch", func(t *testing.T) {
		moderations, res := th.SystemAdminClient.PatchChannelModerations(channel.Id, emptyPatch)
		require.Nil(t, res.Error)
		require.Equal(t, len(moderations), 4)
		for _, moderation := range moderations {
			if moderation.Name == "manage_members" {
				require.Empty(t, moderation.Roles.Guests)
			} else {
				require.Equal(t, moderation.Roles.Guests.Value, true)
				require.Equal(t, moderation.Roles.Guests.Enabled, true)
			}

			require.Equal(t, moderation.Roles.Members.Value, true)
			require.Equal(t, moderation.Roles.Members.Enabled, true)
		}

		require.Nil(t, channel.SchemeId)
	})

	t.Run("Creates a scheme and returns the updated channel moderations when patching an existing permission", func(t *testing.T) {
		patch := []*model.ChannelModerationPatch{
			{
				Name:  &createPosts,
				Roles: &model.ChannelModeratedRolesPatch{Members: model.NewBool(false)},
			},
		}

		moderations, res := th.SystemAdminClient.PatchChannelModerations(channel.Id, patch)
		require.Nil(t, res.Error)
		require.Equal(t, len(moderations), 4)
		for _, moderation := range moderations {
			if moderation.Name == "manage_members" {
				require.Empty(t, moderation.Roles.Guests)
			} else {
				require.Equal(t, moderation.Roles.Guests.Value, true)
				require.Equal(t, moderation.Roles.Guests.Enabled, true)
			}

			if moderation.Name == createPosts {
				require.Equal(t, moderation.Roles.Members.Value, false)
				require.Equal(t, moderation.Roles.Members.Enabled, true)
			} else {
				require.Equal(t, moderation.Roles.Members.Value, true)
				require.Equal(t, moderation.Roles.Members.Enabled, true)
			}
		}
		channel, _ = th.App.GetChannel(channel.Id)
		require.NotNil(t, channel.SchemeId)
	})

	t.Run("Removes the existing scheme when moderated permissions are set back to higher scoped values", func(t *testing.T) {
		channel, _ = th.App.GetChannel(channel.Id)
		schemeId := channel.SchemeId

		scheme, _ := th.App.GetScheme(*schemeId)
		require.Equal(t, scheme.DeleteAt, int64(0))

		patch := []*model.ChannelModerationPatch{
			{
				Name:  &createPosts,
				Roles: &model.ChannelModeratedRolesPatch{Members: model.NewBool(true)},
			},
		}

		moderations, res := th.SystemAdminClient.PatchChannelModerations(channel.Id, patch)
		require.Nil(t, res.Error)
		require.Equal(t, len(moderations), 4)
		for _, moderation := range moderations {
			if moderation.Name == "manage_members" {
				require.Empty(t, moderation.Roles.Guests)
			} else {
				require.Equal(t, moderation.Roles.Guests.Value, true)
				require.Equal(t, moderation.Roles.Guests.Enabled, true)
			}

			require.Equal(t, moderation.Roles.Members.Value, true)
			require.Equal(t, moderation.Roles.Members.Enabled, true)
		}

		channel, _ = th.App.GetChannel(channel.Id)
		require.Nil(t, channel.SchemeId)

		scheme, _ = th.App.GetScheme(*schemeId)
		require.NotEqual(t, scheme.DeleteAt, int64(0))
	})

}

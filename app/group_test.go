// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/stretchr/testify/require"
)

func TestGetGroup(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group := th.CreateGroup()

	group, err := th.App.GetGroup(group.Id)
	require.Nil(t, err)
	require.NotNil(t, group)

	group, err = th.App.GetGroup(model.NewId())
	require.NotNil(t, err)
	require.Nil(t, group)
}

func TestGetGroupByRemoteID(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group := th.CreateGroup()

	g, err := th.App.GetGroupByRemoteID(group.RemoteId, model.GroupSourceLdap)
	require.Nil(t, err)
	require.NotNil(t, g)

	g, err = th.App.GetGroupByRemoteID(model.NewId(), model.GroupSourceLdap)
	require.NotNil(t, err)
	require.Nil(t, g)
}

func TestGetGroupsByType(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	th.CreateGroup()
	th.CreateGroup()
	th.CreateGroup()

	groups, err := th.App.GetGroupsBySource(model.GroupSourceLdap)
	require.Nil(t, err)
	require.NotEmpty(t, groups)

	groups, err = th.App.GetGroupsBySource(model.GroupSource("blah"))
	require.Nil(t, err)
	require.Empty(t, groups)
}

func TestCreateGroup(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewId()
	group := &model.Group{
		DisplayName: "dn_" + id,
		Name:        "name" + id,
		Source:      model.GroupSourceLdap,
		RemoteId:    model.NewId(),
	}

	g, err := th.App.CreateGroup(group)
	require.Nil(t, err)
	require.NotNil(t, g)

	g, err = th.App.CreateGroup(group)
	require.NotNil(t, err)
	require.Nil(t, g)
}

func TestUpdateGroup(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group := th.CreateGroup()
	group.DisplayName = model.NewId()

	g, err := th.App.UpdateGroup(group)
	require.Nil(t, err)
	require.NotNil(t, g)
}

func TestDeleteGroup(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group := th.CreateGroup()

	g, err := th.App.DeleteGroup(group.Id)
	require.Nil(t, err)
	require.NotNil(t, g)

	g, err = th.App.DeleteGroup(group.Id)
	require.NotNil(t, err)
	require.Nil(t, g)
}

func TestCreateOrRestoreGroupMember(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group := th.CreateGroup()

	g, err := th.App.UpsertGroupMember(group.Id, th.BasicUser.Id)
	require.Nil(t, err)
	require.NotNil(t, g)

	g, err = th.App.UpsertGroupMember(group.Id, th.BasicUser.Id)
	require.Nil(t, err)
	require.NotNil(t, g)
}

func TestDeleteGroupMember(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group := th.CreateGroup()
	groupMember, err := th.App.UpsertGroupMember(group.Id, th.BasicUser.Id)
	require.Nil(t, err)
	require.NotNil(t, groupMember)

	groupMember, err = th.App.DeleteGroupMember(groupMember.GroupId, groupMember.UserId)
	require.Nil(t, err)
	require.NotNil(t, groupMember)

	groupMember, err = th.App.DeleteGroupMember(groupMember.GroupId, groupMember.UserId)
	require.NotNil(t, err)
	require.Nil(t, groupMember)
}

func TestUpsertGroupSyncable(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group := th.CreateGroup()
	groupSyncable := model.NewGroupTeam(group.Id, th.BasicTeam.Id, false)

	gs, err := th.App.UpsertGroupSyncable(groupSyncable)
	require.Nil(t, err)
	require.NotNil(t, gs)

	// can update again without error
	gs, err = th.App.UpsertGroupSyncable(groupSyncable)
	require.Nil(t, err)
	require.NotNil(t, gs)

	gs, err = th.App.DeleteGroupSyncable(gs.GroupId, gs.SyncableId, gs.Type)
	require.Nil(t, err)
	require.NotEqual(t, int64(0), gs.DeleteAt)

	// Un-deleting works
	gs.DeleteAt = 0
	gs, err = th.App.UpsertGroupSyncable(gs)
	require.Nil(t, err)
	require.Equal(t, int64(0), gs.DeleteAt)
}

func TestGetGroupSyncable(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group := th.CreateGroup()
	groupSyncable := model.NewGroupTeam(group.Id, th.BasicTeam.Id, false)

	gs, err := th.App.UpsertGroupSyncable(groupSyncable)
	require.Nil(t, err)
	require.NotNil(t, gs)

	gs, err = th.App.GetGroupSyncable(group.Id, th.BasicTeam.Id, model.GroupSyncableTypeTeam)
	require.Nil(t, err)
	require.NotNil(t, gs)
}

func TestGetGroupSyncables(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group := th.CreateGroup()

	// Create a group team
	groupSyncable := model.NewGroupTeam(group.Id, th.BasicTeam.Id, false)

	gs, err := th.App.UpsertGroupSyncable(groupSyncable)
	require.Nil(t, err)
	require.NotNil(t, gs)

	groupTeams, err := th.App.GetGroupSyncables(group.Id, model.GroupSyncableTypeTeam)
	require.Nil(t, err)

	require.NotEmpty(t, groupTeams)
}

func TestDeleteGroupSyncable(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group := th.CreateGroup()
	groupChannel := model.NewGroupChannel(group.Id, th.BasicChannel.Id, false)

	gs, err := th.App.UpsertGroupSyncable(groupChannel)
	require.Nil(t, err)
	require.NotNil(t, gs)

	gs, err = th.App.DeleteGroupSyncable(group.Id, th.BasicChannel.Id, model.GroupSyncableTypeChannel)
	require.Nil(t, err)
	require.NotNil(t, gs)

	gs, err = th.App.DeleteGroupSyncable(group.Id, th.BasicChannel.Id, model.GroupSyncableTypeChannel)
	require.NotNil(t, err)
	require.Nil(t, gs)
}

func TestGetGroupsByChannel(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group := th.CreateGroup()

	// Create a group channel
	groupSyncable := &model.GroupSyncable{
		GroupId:    group.Id,
		AutoAdd:    false,
		SyncableId: th.BasicChannel.Id,
		Type:       model.GroupSyncableTypeChannel,
	}

	gs, err := th.App.UpsertGroupSyncable(groupSyncable)
	require.Nil(t, err)
	require.NotNil(t, gs)

	opts := model.GroupSearchOpts{
		PageOpts: &model.PageOpts{
			Page:    0,
			PerPage: 60,
		},
	}

	groups, _, err := th.App.GetGroupsByChannel(th.BasicChannel.Id, opts)
	require.Nil(t, err)
	require.ElementsMatch(t, []*model.GroupWithSchemeAdmin{{Group: *group, SchemeAdmin: model.NewBool(false)}}, groups)
	require.NotNil(t, groups[0].SchemeAdmin)

	groups, _, err = th.App.GetGroupsByChannel(model.NewId(), opts)
	require.Nil(t, err)
	require.Empty(t, groups)
}

func TestGetGroupsByTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group := th.CreateGroup()

	// Create a group team
	groupSyncable := &model.GroupSyncable{
		GroupId:    group.Id,
		AutoAdd:    false,
		SyncableId: th.BasicTeam.Id,
		Type:       model.GroupSyncableTypeTeam,
	}

	gs, err := th.App.UpsertGroupSyncable(groupSyncable)
	require.Nil(t, err)
	require.NotNil(t, gs)

	groups, _, err := th.App.GetGroupsByTeam(th.BasicTeam.Id, model.GroupSearchOpts{})
	require.Nil(t, err)
	require.ElementsMatch(t, []*model.GroupWithSchemeAdmin{{Group: *group, SchemeAdmin: model.NewBool(false)}}, groups)
	require.NotNil(t, groups[0].SchemeAdmin)

	groups, _, err = th.App.GetGroupsByTeam(model.NewId(), model.GroupSearchOpts{})
	require.Nil(t, err)
	require.Empty(t, groups)
}

func TestGetGroups(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group := th.CreateGroup()

	groups, err := th.App.GetGroups(0, 60, model.GroupSearchOpts{})
	require.Nil(t, err)
	require.ElementsMatch(t, []*model.Group{group}, groups)
}

func TestUserIsInAdminRoleGroup(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	group1 := th.CreateGroup()
	group2 := th.CreateGroup()

	g, err := th.App.UpsertGroupMember(group1.Id, th.BasicUser.Id)
	require.Nil(t, err)
	require.NotNil(t, g)

	g, err = th.App.UpsertGroupMember(group2.Id, th.BasicUser.Id)
	require.Nil(t, err)
	require.NotNil(t, g)

	_, err = th.App.UpsertGroupSyncable(&model.GroupSyncable{
		GroupId:    group1.Id,
		AutoAdd:    false,
		SyncableId: th.BasicTeam.Id,
		Type:       model.GroupSyncableTypeTeam,
	})
	require.Nil(t, err)

	groupSyncable2, err := th.App.UpsertGroupSyncable(&model.GroupSyncable{
		GroupId:    group2.Id,
		AutoAdd:    false,
		SyncableId: th.BasicTeam.Id,
		Type:       model.GroupSyncableTypeTeam,
	})
	require.Nil(t, err)

	// no syncables are set to scheme admin true, so this returns false
	actual, err := th.App.UserIsInAdminRoleGroup(th.BasicUser.Id, th.BasicTeam.Id, model.GroupSyncableTypeTeam)
	require.Nil(t, err)
	require.False(t, actual)

	// set a syncable to be scheme admins
	groupSyncable2.SchemeAdmin = true
	_, err = th.App.UpdateGroupSyncable(groupSyncable2)
	require.Nil(t, err)

	// a syncable is set to scheme admin true, so this returns true
	actual, err = th.App.UserIsInAdminRoleGroup(th.BasicUser.Id, th.BasicTeam.Id, model.GroupSyncableTypeTeam)
	require.Nil(t, err)
	require.True(t, actual)

	// delete the syncable, should be false again
	th.App.DeleteGroupSyncable(group2.Id, th.BasicTeam.Id, model.GroupSyncableTypeTeam)
	actual, err = th.App.UserIsInAdminRoleGroup(th.BasicUser.Id, th.BasicTeam.Id, model.GroupSyncableTypeTeam)
	require.Nil(t, err)
	require.False(t, actual)
}

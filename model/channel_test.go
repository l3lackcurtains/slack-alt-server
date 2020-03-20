// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelJson(t *testing.T) {
	o := Channel{Id: NewId(), Name: NewId()}
	json := o.ToJson()
	ro := ChannelFromJson(strings.NewReader(json))

	require.Equal(t, o.Id, ro.Id)

	p := ChannelPatch{Name: new(string)}
	*p.Name = NewId()
	json = p.ToJson()
	rp := ChannelPatchFromJson(strings.NewReader(json))

	require.Equal(t, *p.Name, *rp.Name)
}

func TestChannelCopy(t *testing.T) {
	o := Channel{Id: NewId(), Name: NewId()}
	ro := o.DeepCopy()

	require.Equal(t, o.Id, ro.Id, "Ids do not match")
}

func TestChannelPatch(t *testing.T) {
	p := &ChannelPatch{Name: new(string), DisplayName: new(string), Header: new(string), Purpose: new(string), GroupConstrained: new(bool)}
	*p.Name = NewId()
	*p.DisplayName = NewId()
	*p.Header = NewId()
	*p.Purpose = NewId()
	*p.GroupConstrained = true

	o := Channel{Id: NewId(), Name: NewId()}
	o.Patch(p)

	require.Equal(t, *p.Name, o.Name)
	require.Equal(t, *p.DisplayName, o.DisplayName)
	require.Equal(t, *p.Header, o.Header)
	require.Equal(t, *p.Purpose, o.Purpose)
	require.Equal(t, *p.GroupConstrained, *o.GroupConstrained)
}

func TestChannelIsValid(t *testing.T) {
	o := Channel{}

	require.Error(t, o.IsValid())

	o.Id = NewId()
	require.Error(t, o.IsValid())

	o.CreateAt = GetMillis()
	require.Error(t, o.IsValid())

	o.UpdateAt = GetMillis()
	require.Error(t, o.IsValid())

	o.DisplayName = strings.Repeat("01234567890", 20)
	require.Error(t, o.IsValid())

	o.DisplayName = "1234"
	o.Name = "ZZZZZZZ"
	require.Error(t, o.IsValid())

	o.Name = "zzzzz"
	require.Error(t, o.IsValid())

	o.Type = "U"
	require.Error(t, o.IsValid())

	o.Type = "P"
	require.Error(t, o.IsValid())

	o.Header = strings.Repeat("01234567890", 100)
	require.Error(t, o.IsValid())

	o.Header = "1234"
	require.Nil(t, o.IsValid())

	o.Purpose = strings.Repeat("01234567890", 30)
	require.Error(t, o.IsValid())

	o.Purpose = "1234"
	require.Nil(t, o.IsValid())

	o.Purpose = strings.Repeat("0123456789", 25)
	require.Nil(t, o.IsValid())
}

func TestChannelPreSave(t *testing.T) {
	o := Channel{Name: "test"}
	o.PreSave()
	o.Etag()
}

func TestChannelPreUpdate(t *testing.T) {
	o := Channel{Name: "test"}
	o.PreUpdate()
}

func TestGetGroupDisplayNameFromUsers(t *testing.T) {
	users := make([]*User, 4)
	users[0] = &User{Username: NewId()}
	users[1] = &User{Username: NewId()}
	users[2] = &User{Username: NewId()}
	users[3] = &User{Username: NewId()}

	name := GetGroupDisplayNameFromUsers(users, true)
	require.LessOrEqual(t, len(name), CHANNEL_NAME_MAX_LENGTH)
}

func TestGetGroupNameFromUserIds(t *testing.T) {
	name := GetGroupNameFromUserIds([]string{NewId(), NewId(), NewId(), NewId(), NewId()})

	require.LessOrEqual(t, len(name), CHANNEL_NAME_MAX_LENGTH)
}

// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package commands

import (
	"testing"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/stretchr/testify/require"
)

func TestCreateUserWithTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	id := model.NewId()
	email := "success+" + id + "@simulator.amazonses.com"
	username := "name" + id

	th.CheckCommand(t, "user", "create", "--email", email, "--password", "mypassword1", "--username", username)

	th.CheckCommand(t, "team", "add", th.BasicTeam.Id, email)

	profiles := th.SystemAdminClient.Must(th.SystemAdminClient.GetUsersInTeam(th.BasicTeam.Id, 0, 1000, "")).([]*model.User)

	found := false

	for _, user := range profiles {
		if user.Email == email {
			found = true
		}

	}

	require.True(t, found, "Failed to create User")
}

func TestCreateUserWithoutTeam(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	id := model.NewId()
	email := "success+" + id + "@simulator.amazonses.com"
	username := "name" + id

	th.CheckCommand(t, "user", "create", "--email", email, "--password", "mypassword1", "--username", username)

	user, err := th.App.Srv().Store.User().GetByEmail(email)
	require.Nil(t, err)

	require.Equal(t, email, user.Email)
}

func TestResetPassword(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.CheckCommand(t, "user", "password", th.BasicUser.Email, "password2")

	th.Client.Logout()
	th.BasicUser.Password = "password2"
	th.LoginBasic()
}

func TestMakeUserActiveAndInactive(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// first inactivate the user
	th.CheckCommand(t, "user", "deactivate", th.BasicUser.Email)

	// activate the inactive user
	th.CheckCommand(t, "user", "activate", th.BasicUser.Email)
}

func TestChangeUserEmail(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	newEmail := model.NewId() + "@mattermost-test.com"

	th.CheckCommand(t, "user", "email", th.BasicUser.Username, newEmail)
	_, err := th.App.Srv().Store.User().GetByEmail(th.BasicUser.Email)
	require.NotNil(t, err, "should've updated to the new email")

	user, err := th.App.Srv().Store.User().GetByEmail(newEmail)
	require.Nil(t, err)

	require.Equal(t, user.Email, newEmail, "should've updated to the new email")

	// should fail because using an invalid email
	require.Error(t, th.RunCommand(t, "user", "email", th.BasicUser.Username, "wrong$email.com"))

	// should fail because missing one parameter
	require.Error(t, th.RunCommand(t, "user", "email", th.BasicUser.Username))

	// should fail because missing both parameters
	require.Error(t, th.RunCommand(t, "user", "email"))

	// should fail because have more than 2  parameters
	require.Error(t, th.RunCommand(t, "user", "email", th.BasicUser.Username, "new@email.com", "extra!"))

	// should fail because user not found
	require.Error(t, th.RunCommand(t, "user", "email", "invalidUser", newEmail))

	// should fail because email already in use
	require.Error(t, th.RunCommand(t, "user", "email", th.BasicUser.Username, th.BasicUser2.Email))

}

func TestDeleteUserBotUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.CheckCommand(t, "user", "delete", th.BasicUser.Username, "--confirm")
	_, err := th.App.Srv().Store.User().Get(th.BasicUser.Id)
	require.Error(t, err)

	// Make a bot
	bot := &model.Bot{
		Username:    "bottodelete",
		Description: "Delete me!",
		OwnerId:     model.NewId(),
	}
	user, err := th.App.Srv().Store.User().Save(model.UserFromBot(bot))
	require.Nil(t, err)
	bot.UserId = user.Id
	bot, err = th.App.Srv().Store.Bot().Save(bot)
	require.Nil(t, err)

	th.CheckCommand(t, "user", "delete", bot.Username, "--confirm")
	_, err = th.App.Srv().Store.User().Get(user.Id)
	require.Error(t, err)
	_, err = th.App.Srv().Store.Bot().Get(user.Id, true)
	require.Error(t, err)
}

func TestConvertUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("Invalid command line input", func(t *testing.T) {
		err := th.RunCommand(t, "user", "convert", th.BasicUser.Username)
		require.NotNil(t, err)

		err = th.RunCommand(t, "user", "convert", th.BasicUser.Username, "--user", "--bot")
		require.NotNil(t, err)

		err = th.RunCommand(t, "user", "convert", "--bot")
		require.NotNil(t, err)
	})

	t.Run("Convert to bot from username", func(t *testing.T) {
		th.CheckCommand(t, "user", "convert", th.BasicUser.Username, "anotherinvaliduser", "--bot")
		_, err := th.App.Srv().Store.Bot().Get(th.BasicUser.Id, false)
		require.Nil(t, err)
	})

	t.Run("Unable to convert to user with missing password", func(t *testing.T) {
		err := th.RunCommand(t, "user", "convert", th.BasicUser.Username, "--user")
		require.NotNil(t, err)
	})

	t.Run("Unable to convert to user with invalid email", func(t *testing.T) {
		err := th.RunCommand(t, "user", "convert", th.BasicUser.Username, "--user",
			"--password", "password",
			"--email", "invalidEmail")
		require.NotNil(t, err)
	})

	t.Run("Convert to user with minimum flags", func(t *testing.T) {
		err := th.RunCommand(t, "user", "convert", th.BasicUser.Username, "--user",
			"--password", "password")
		require.Nil(t, err)
		_, err = th.App.Srv().Store.Bot().Get(th.BasicUser.Id, false)
		require.NotNil(t, err)
	})

	t.Run("Convert to bot from email", func(t *testing.T) {
		th.CheckCommand(t, "user", "convert", th.BasicUser2.Email, "--bot")
		_, err := th.App.Srv().Store.Bot().Get(th.BasicUser2.Id, false)
		require.Nil(t, err)
	})

	t.Run("Convert to user with all flags", func(t *testing.T) {
		err := th.RunCommand(t, "user", "convert", th.BasicUser2.Username, "--user",
			"--password", "password",
			"--username", "newusername",
			"--email", "valid@email.com",
			"--nickname", "newNickname",
			"--firstname", "newFirstName",
			"--lastname", "newLastName",
			"--locale", "en_CA",
			"--system_admin")
		require.Nil(t, err)

		_, err = th.App.Srv().Store.Bot().Get(th.BasicUser2.Id, false)
		require.NotNil(t, err)

		user, appErr := th.App.Srv().Store.User().Get(th.BasicUser2.Id)
		require.Nil(t, appErr)
		require.Equal(t, "newusername", user.Username)
		require.Equal(t, "valid@email.com", user.Email)
		require.Equal(t, "newNickname", user.Nickname)
		require.Equal(t, "newFirstName", user.FirstName)
		require.Equal(t, "newLastName", user.LastName)
		require.Equal(t, "en_CA", user.Locale)
		require.True(t, user.IsInRole("system_admin"))
	})

}

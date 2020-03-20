// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
)

func TestWebSocket(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	WebSocketClient, err := th.CreateWebSocketClient()
	require.Nil(t, err)
	defer WebSocketClient.Close()

	time.Sleep(300 * time.Millisecond)

	// Test closing and reconnecting
	WebSocketClient.Close()
	err = WebSocketClient.Connect()
	require.Nil(t, err)

	WebSocketClient.Listen()

	time.Sleep(300 * time.Millisecond)
	resp := <-WebSocketClient.ResponseChannel
	require.Equal(t, resp.Status, model.STATUS_OK, "should have responded OK to authentication challenge")

	WebSocketClient.SendMessage("ping", nil)
	time.Sleep(300 * time.Millisecond)
	resp = <-WebSocketClient.ResponseChannel
	require.Equal(t, resp.Data["text"].(string), "pong", "wrong response")

	WebSocketClient.SendMessage("", nil)
	time.Sleep(300 * time.Millisecond)
	resp = <-WebSocketClient.ResponseChannel
	require.Equal(t, resp.Error.Id, "api.web_socket_router.no_action.app_error", "should have been no action response")

	WebSocketClient.SendMessage("junk", nil)
	time.Sleep(300 * time.Millisecond)
	resp = <-WebSocketClient.ResponseChannel
	require.Equal(t, resp.Error.Id, "api.web_socket_router.bad_action.app_error", "should have been bad action response")

	req := &model.WebSocketRequest{}
	req.Seq = 0
	req.Action = "ping"
	WebSocketClient.Conn.WriteJSON(req)
	time.Sleep(300 * time.Millisecond)
	resp = <-WebSocketClient.ResponseChannel
	require.Equal(t, resp.Error.Id, "api.web_socket_router.bad_seq.app_error", "should have been bad action response")

	WebSocketClient.UserTyping("", "")
	time.Sleep(300 * time.Millisecond)
	resp = <-WebSocketClient.ResponseChannel
	require.Equal(t, resp.Error.Id, "api.websocket_handler.invalid_param.app_error", "should have been invalid param response")
	require.Equal(t, resp.Error.DetailedError, "", "detailed error not cleared")

	WebSocketClient.UserTyping(th.BasicChannel.Id, "")
	time.Sleep(300 * time.Millisecond)
	resp = <-WebSocketClient.ResponseChannel
	require.Nil(t, resp.Error)

	WebSocketClient.UserTyping(th.BasicPrivateChannel2.Id, "")
	time.Sleep(300 * time.Millisecond)
	resp = <-WebSocketClient.ResponseChannel
	require.Equal(t, resp.Error.Id, "api.websocket_handler.invalid_param.app_error", "should have been invalid param response")
	require.Equal(t, resp.Error.DetailedError, "", "detailed error not cleared")
}

func TestWebSocketTrailingSlash(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	url := fmt.Sprintf("ws://localhost:%v", th.App.Srv().ListenAddr.Port)
	_, _, err := websocket.DefaultDialer.Dial(url+model.API_URL_SUFFIX+"/websocket/", nil)
	require.NoError(t, err)
}

func TestWebSocketEvent(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	WebSocketClient, err := th.CreateWebSocketClient()
	require.Nil(t, err)
	defer WebSocketClient.Close()

	WebSocketClient.Listen()

	time.Sleep(300 * time.Millisecond)
	resp := <-WebSocketClient.ResponseChannel
	require.Equal(t, resp.Status, model.STATUS_OK, "should have responded OK to authentication challenge")

	omitUser := make(map[string]bool, 1)
	omitUser["somerandomid"] = true
	evt1 := model.NewWebSocketEvent(model.WEBSOCKET_EVENT_TYPING, "", th.BasicChannel.Id, "", omitUser)
	evt1.Add("user_id", "somerandomid")
	th.App.Publish(evt1)

	time.Sleep(300 * time.Millisecond)

	stop := make(chan bool)
	eventHit := false

	go func() {
		for {
			select {
			case resp := <-WebSocketClient.EventChannel:
				if resp.EventType() == model.WEBSOCKET_EVENT_TYPING && resp.GetData()["user_id"].(string) == "somerandomid" {
					eventHit = true
				}
			case <-stop:
				return
			}
		}
	}()

	time.Sleep(400 * time.Millisecond)

	stop <- true

	require.True(t, eventHit, "did not receive typing event")

	evt2 := model.NewWebSocketEvent(model.WEBSOCKET_EVENT_TYPING, "", "somerandomid", "", nil)
	th.App.Publish(evt2)
	time.Sleep(300 * time.Millisecond)

	eventHit = false

	go func() {
		for {
			select {
			case resp := <-WebSocketClient.EventChannel:
				if resp.EventType() == model.WEBSOCKET_EVENT_TYPING {
					eventHit = true
				}
			case <-stop:
				return
			}
		}
	}()

	time.Sleep(400 * time.Millisecond)

	stop <- true

	require.False(t, eventHit, "got typing event for bad channel id")
}

func TestCreateDirectChannelWithSocket(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	user2 := th.BasicUser2

	users := make([]*model.User, 0)
	users = append(users, user2)

	for i := 0; i < 10; i++ {
		users = append(users, th.CreateUser())
	}

	WebSocketClient, err := th.CreateWebSocketClient()
	require.Nil(t, err)
	defer WebSocketClient.Close()
	WebSocketClient.Listen()

	time.Sleep(300 * time.Millisecond)
	resp := <-WebSocketClient.ResponseChannel
	require.Equal(t, resp.Status, model.STATUS_OK, "should have responded OK to authentication challenge")

	wsr := <-WebSocketClient.EventChannel
	require.Equal(t, wsr.EventType(), model.WEBSOCKET_EVENT_HELLO, "missing hello")

	stop := make(chan bool)
	count := 0

	go func() {
		for {
			select {
			case wsr := <-WebSocketClient.EventChannel:
				if wsr != nil && wsr.EventType() == model.WEBSOCKET_EVENT_DIRECT_ADDED {
					count = count + 1
				}

			case <-stop:
				return
			}
		}
	}()

	for _, user := range users {
		time.Sleep(100 * time.Millisecond)
		_, resp := Client.CreateDirectChannel(th.BasicUser.Id, user.Id)
		require.Nil(t, resp.Error, "failed to create DM channel")
	}

	time.Sleep(5000 * time.Millisecond)

	stop <- true

	require.Equal(t, count, len(users), "We didn't get the proper amount of direct_added messages")
}

func TestWebsocketOriginSecurity(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	url := fmt.Sprintf("ws://localhost:%v", th.App.Srv().ListenAddr.Port)

	// Should fail because origin doesn't match
	_, _, err := websocket.DefaultDialer.Dial(url+model.API_URL_SUFFIX+"/websocket", http.Header{
		"Origin": []string{"http://www.evil.com"},
	})

	require.NotNil(t, err, "Should have errored because Origin does not match host! SECURITY ISSUE!")

	// We are not a browser so we can spoof this just fine
	_, _, err = websocket.DefaultDialer.Dial(url+model.API_URL_SUFFIX+"/websocket", http.Header{
		"Origin": []string{fmt.Sprintf("http://localhost:%v", th.App.Srv().ListenAddr.Port)},
	})
	require.Nil(t, err, err)

	// Should succeed now because open CORS
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.AllowCorsFrom = "*" })
	_, _, err = websocket.DefaultDialer.Dial(url+model.API_URL_SUFFIX+"/websocket", http.Header{
		"Origin": []string{"http://www.evil.com"},
	})
	require.Nil(t, err, err)

	// Should succeed now because matching CORS
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.AllowCorsFrom = "http://www.evil.com" })
	_, _, err = websocket.DefaultDialer.Dial(url+model.API_URL_SUFFIX+"/websocket", http.Header{
		"Origin": []string{"http://www.evil.com"},
	})
	require.Nil(t, err, err)

	// Should fail because non-matching CORS
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.AllowCorsFrom = "http://www.good.com" })
	_, _, err = websocket.DefaultDialer.Dial(url+model.API_URL_SUFFIX+"/websocket", http.Header{
		"Origin": []string{"http://www.evil.com"},
	})
	require.NotNil(t, err, "Should have errored because Origin contain AllowCorsFrom")

	// Should fail because non-matching CORS
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.AllowCorsFrom = "http://www.good.com" })
	_, _, err = websocket.DefaultDialer.Dial(url+model.API_URL_SUFFIX+"/websocket", http.Header{
		"Origin": []string{"http://www.good.co"},
	})
	require.NotNil(t, err, "Should have errored because Origin does not match host! SECURITY ISSUE!")

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.AllowCorsFrom = "" })
}

func TestWebSocketStatuses(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	Client := th.Client
	WebSocketClient, err := th.CreateWebSocketClient()
	require.Nil(t, err, err)
	defer WebSocketClient.Close()
	WebSocketClient.Listen()

	time.Sleep(300 * time.Millisecond)
	resp := <-WebSocketClient.ResponseChannel
	require.Equal(t, resp.Status, model.STATUS_OK, "should have responded OK to authentication challenge")

	team := model.Team{DisplayName: "Name", Name: "z-z-" + model.NewRandomTeamName() + "a", Email: "test@nowhere.com", Type: model.TEAM_OPEN}
	rteam, _ := Client.CreateTeam(&team)

	user := model.User{Email: strings.ToLower(model.NewId()) + "success+test@simulator.amazonses.com", Nickname: "Corey Hulen", Password: "passwd1"}
	ruser := Client.Must(Client.CreateUser(&user)).(*model.User)
	th.LinkUserToTeam(ruser, rteam)
	_, err = th.App.Srv().Store.User().VerifyEmail(ruser.Id, ruser.Email)
	require.Nil(t, err)

	user2 := model.User{Email: strings.ToLower(model.NewId()) + "success+test@simulator.amazonses.com", Nickname: "Corey Hulen", Password: "passwd1"}
	ruser2 := Client.Must(Client.CreateUser(&user2)).(*model.User)
	th.LinkUserToTeam(ruser2, rteam)
	_, err = th.App.Srv().Store.User().VerifyEmail(ruser2.Id, ruser2.Email)
	require.Nil(t, err)

	Client.Login(user.Email, user.Password)

	th.LoginBasic2()

	WebSocketClient2, err2 := th.CreateWebSocketClient()
	require.Nil(t, err2, err2)

	time.Sleep(1000 * time.Millisecond)

	WebSocketClient.GetStatuses()
	resp = <-WebSocketClient.ResponseChannel
	require.Nil(t, resp.Error, resp.Error)

	require.Equal(t, resp.SeqReply, WebSocketClient.Sequence-1, "bad sequence number")

	allowedValues := [4]string{model.STATUS_OFFLINE, model.STATUS_AWAY, model.STATUS_ONLINE, model.STATUS_DND}
	for _, status := range resp.Data {
		require.Containsf(t, allowedValues, status, "one of the statuses had an invalid value status=%v", status)
	}

	status, ok := resp.Data[th.BasicUser2.Id]
	require.True(t, ok, "should have had user status")

	require.Equal(t, status, model.STATUS_ONLINE, "status should have been online status=%v", status)

	WebSocketClient.GetStatusesByIds([]string{th.BasicUser2.Id})
	resp = <-WebSocketClient.ResponseChannel
	require.Nil(t, resp.Error, resp.Error)

	require.Equal(t, resp.SeqReply, WebSocketClient.Sequence-1, "bad sequence number")

	allowedValues = [4]string{model.STATUS_OFFLINE, model.STATUS_AWAY, model.STATUS_ONLINE}
	for _, status := range resp.Data {
		require.Containsf(t, allowedValues, status, "one of the statuses had an invalid value status")
	}

	status, ok = resp.Data[th.BasicUser2.Id]
	require.True(t, ok, "should have had user status")

	require.Equal(t, status, model.STATUS_ONLINE, "status should have been online status=%v", status)
	require.Equal(t, len(resp.Data), 1, "only 1 status should be returned")

	WebSocketClient.GetStatusesByIds([]string{ruser2.Id, "junk"})
	resp = <-WebSocketClient.ResponseChannel
	require.Nil(t, resp.Error, resp.Error)
	require.Equal(t, resp.SeqReply, WebSocketClient.Sequence-1, "bad sequence number")
	require.Equal(t, len(resp.Data), 2, "2 statuses should be returned")

	WebSocketClient.GetStatusesByIds([]string{})
	if resp2 := <-WebSocketClient.ResponseChannel; resp2.Error == nil {
		require.Equal(t, resp2.SeqReply, WebSocketClient.Sequence-1, "bad sequence number")
		require.NotNil(t, resp2.Error, "should have errored - empty user ids")
	}

	WebSocketClient2.Close()

	th.App.SetStatusAwayIfNeeded(th.BasicUser.Id, false)

	awayTimeout := *th.App.Config().TeamSettings.UserStatusAwayTimeout
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.UserStatusAwayTimeout = awayTimeout })
	}()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.UserStatusAwayTimeout = 1 })

	time.Sleep(1500 * time.Millisecond)

	th.App.SetStatusAwayIfNeeded(th.BasicUser.Id, false)
	th.App.SetStatusOnline(th.BasicUser.Id, false)

	time.Sleep(1500 * time.Millisecond)

	WebSocketClient.GetStatuses()
	resp = <-WebSocketClient.ResponseChannel
	require.Nil(t, resp.Error)

	require.Equal(t, resp.SeqReply, WebSocketClient.Sequence-1, "bad sequence number")
	_, ok = resp.Data[th.BasicUser2.Id]
	require.False(t, ok, "should not have had user status")

	stop := make(chan bool)
	onlineHit := false
	awayHit := false

	go func() {
		for {
			select {
			case resp := <-WebSocketClient.EventChannel:
				if resp.EventType() == model.WEBSOCKET_EVENT_STATUS_CHANGE && resp.GetData()["user_id"].(string) == th.BasicUser.Id {
					status := resp.GetData()["status"].(string)
					if status == model.STATUS_ONLINE {
						onlineHit = true
					} else if status == model.STATUS_AWAY {
						awayHit = true
					}
				}
			case <-stop:
				return
			}
		}
	}()

	time.Sleep(500 * time.Millisecond)

	stop <- true

	require.True(t, onlineHit, "didn't get online event")
	require.True(t, awayHit, "didn't get away event")

	time.Sleep(500 * time.Millisecond)

	WebSocketClient.Close()
}

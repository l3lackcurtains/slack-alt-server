// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mattermost/mattermost-server/v5/mlog"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPing(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("basic ping", func(t *testing.T) {

		t.Run("healthy", func(t *testing.T) {
			status, resp := th.Client.GetPing()
			CheckNoError(t, resp)
			assert.Equal(t, model.STATUS_OK, status)
		})

		t.Run("unhealthy", func(t *testing.T) {
			goRoutineHealthThreshold := *th.App.Config().ServiceSettings.GoroutineHealthThreshold
			defer func() {
				th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.GoroutineHealthThreshold = goRoutineHealthThreshold })
			}()

			th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.GoroutineHealthThreshold = 10 })
			status, resp := th.Client.GetPing()
			CheckInternalErrorStatus(t, resp)
			assert.Equal(t, model.STATUS_UNHEALTHY, status)
		})

	})

	t.Run("with server status", func(t *testing.T) {

		t.Run("healthy", func(t *testing.T) {
			status, resp := th.Client.GetPingWithServerStatus()
			CheckNoError(t, resp)
			assert.Equal(t, model.STATUS_OK, status)
		})

		t.Run("unhealthy", func(t *testing.T) {
			badDriver := "badDriverName"
			th.App.Config().FileSettings.DriverName = &badDriver

			status, resp := th.Client.GetPingWithServerStatus()
			CheckInternalErrorStatus(t, resp)
			assert.Equal(t, model.STATUS_UNHEALTHY, status)
		})

	})
}

func TestGetAudits(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	audits, resp := th.SystemAdminClient.GetAudits(0, 100, "")
	CheckNoError(t, resp)
	require.NotEmpty(t, audits, "should not be empty")

	audits, resp = th.SystemAdminClient.GetAudits(0, 1, "")
	CheckNoError(t, resp)
	require.Len(t, audits, 1, "should only be 1")

	audits, resp = th.SystemAdminClient.GetAudits(1, 1, "")
	CheckNoError(t, resp)
	require.Len(t, audits, 1, "should only be 1")

	_, resp = th.SystemAdminClient.GetAudits(-1, -1, "")
	CheckNoError(t, resp)

	_, resp = Client.GetAudits(0, 100, "")
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetAudits(0, 100, "")
	CheckUnauthorizedStatus(t, resp)
}

func TestEmailTest(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	config := model.Config{
		ServiceSettings: model.ServiceSettings{
			SiteURL: model.NewString(""),
		},
		EmailSettings: model.EmailSettings{
			SMTPServer:             model.NewString(""),
			SMTPPort:               model.NewString(""),
			SMTPPassword:           model.NewString(""),
			FeedbackName:           model.NewString(""),
			FeedbackEmail:          model.NewString(""),
			ReplyToAddress:         model.NewString(""),
			SendEmailNotifications: model.NewBool(false),
		},
	}

	t.Run("as system user", func(t *testing.T) {
		_, resp := Client.TestEmail(&config)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("as system admin", func(t *testing.T) {
		_, resp := th.SystemAdminClient.TestEmail(&config)
		CheckErrorMessage(t, resp, "api.admin.test_email.missing_server")
		CheckBadRequestStatus(t, resp)

		inbucket_host := os.Getenv("CI_INBUCKET_HOST")
		if inbucket_host == "" {
			inbucket_host = "localhost"
		}

		inbucket_port := os.Getenv("CI_INBUCKET_PORT")
		if inbucket_port == "" {
			inbucket_port = "9000"
		}

		*config.EmailSettings.SMTPServer = inbucket_host
		*config.EmailSettings.SMTPPort = inbucket_port
		_, resp = th.SystemAdminClient.TestEmail(&config)
		CheckOKStatus(t, resp)
	})

	t.Run("as restricted system admin", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ExperimentalSettings.RestrictSystemAdmin = true })

		_, resp := th.SystemAdminClient.TestEmail(&config)
		CheckForbiddenStatus(t, resp)
	})
}

func TestSiteURLTest(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/valid/api/v4/system/ping") {
			w.WriteHeader(200)
		} else {
			w.WriteHeader(400)
		}
	}))
	defer ts.Close()

	validSiteURL := ts.URL + "/valid"
	invalidSiteURL := ts.URL + "/invalid"

	t.Run("as system admin", func(t *testing.T) {
		_, resp := th.SystemAdminClient.TestSiteURL("")
		CheckBadRequestStatus(t, resp)

		_, resp = th.SystemAdminClient.TestSiteURL(invalidSiteURL)
		CheckBadRequestStatus(t, resp)

		_, resp = th.SystemAdminClient.TestSiteURL(validSiteURL)
		CheckOKStatus(t, resp)
	})

	t.Run("as system user", func(t *testing.T) {
		_, resp := Client.TestSiteURL(validSiteURL)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("as restricted system admin", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ExperimentalSettings.RestrictSystemAdmin = true })

		_, resp := Client.TestSiteURL(validSiteURL)
		CheckForbiddenStatus(t, resp)
	})
}

func TestDatabaseRecycle(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	t.Run("as system user", func(t *testing.T) {
		_, resp := Client.DatabaseRecycle()
		CheckForbiddenStatus(t, resp)
	})

	t.Run("as system admin", func(t *testing.T) {
		_, resp := th.SystemAdminClient.DatabaseRecycle()
		CheckNoError(t, resp)
	})

	t.Run("as restricted system admin", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ExperimentalSettings.RestrictSystemAdmin = true })

		_, resp := th.SystemAdminClient.DatabaseRecycle()
		CheckForbiddenStatus(t, resp)
	})
}

func TestInvalidateCaches(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	t.Run("as system user", func(t *testing.T) {
		ok, resp := Client.InvalidateCaches()
		CheckForbiddenStatus(t, resp)
		require.False(t, ok, "should not clean the cache due to no permission.")
	})

	t.Run("as system admin", func(t *testing.T) {
		ok, resp := th.SystemAdminClient.InvalidateCaches()
		CheckNoError(t, resp)
		require.True(t, ok, "should clean the cache")
	})

	t.Run("as restricted system admin", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ExperimentalSettings.RestrictSystemAdmin = true })

		ok, resp := th.SystemAdminClient.InvalidateCaches()
		CheckForbiddenStatus(t, resp)
		require.False(t, ok, "should not clean the cache due to no permission.")
	})
}

func TestGetLogs(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	for i := 0; i < 20; i++ {
		mlog.Info(strconv.Itoa(i))
	}

	logs, resp := th.SystemAdminClient.GetLogs(0, 10)
	CheckNoError(t, resp)
	require.Len(t, logs, 10)

	for i := 10; i < 20; i++ {
		assert.Containsf(t, logs[i-10], fmt.Sprintf(`"msg":"%d"`, i), "Log line doesn't contain correct message")
	}

	logs, resp = th.SystemAdminClient.GetLogs(1, 10)
	CheckNoError(t, resp)
	require.Len(t, logs, 10)

	logs, resp = th.SystemAdminClient.GetLogs(-1, -1)
	CheckNoError(t, resp)
	require.NotEmpty(t, logs, "should not be empty")

	_, resp = Client.GetLogs(0, 10)
	CheckForbiddenStatus(t, resp)

	Client.Logout()
	_, resp = Client.GetLogs(0, 10)
	CheckUnauthorizedStatus(t, resp)
}

func TestPostLog(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	enableDev := *th.App.Config().ServiceSettings.EnableDeveloper
	defer func() {
		*th.App.Config().ServiceSettings.EnableDeveloper = enableDev
	}()
	*th.App.Config().ServiceSettings.EnableDeveloper = true

	message := make(map[string]string)
	message["level"] = "ERROR"
	message["message"] = "this is a test"

	_, resp := Client.PostLog(message)
	CheckNoError(t, resp)

	*th.App.Config().ServiceSettings.EnableDeveloper = false

	_, resp = Client.PostLog(message)
	CheckNoError(t, resp)

	*th.App.Config().ServiceSettings.EnableDeveloper = true

	Client.Logout()

	_, resp = Client.PostLog(message)
	CheckNoError(t, resp)

	*th.App.Config().ServiceSettings.EnableDeveloper = false

	_, resp = Client.PostLog(message)
	CheckForbiddenStatus(t, resp)

	logMessage, resp := th.SystemAdminClient.PostLog(message)
	CheckNoError(t, resp)
	require.NotEmpty(t, logMessage, "should return the log message")

}

func TestGetAnalyticsOld(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	rows, resp := Client.GetAnalyticsOld("", "")
	CheckForbiddenStatus(t, resp)
	require.Nil(t, rows, "should be nil")
	rows, resp = th.SystemAdminClient.GetAnalyticsOld("", "")
	CheckNoError(t, resp)

	found := false
	found2 := false
	for _, row := range rows {
		if row.Name == "unique_user_count" {
			found = true
		} else if row.Name == "inactive_user_count" {
			found2 = true
			assert.True(t, row.Value >= 0)
		}
	}

	assert.True(t, found, "should return unique user count")
	assert.True(t, found2, "should return inactive user count")

	_, resp = th.SystemAdminClient.GetAnalyticsOld("post_counts_day", "")
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.GetAnalyticsOld("user_counts_with_posts_day", "")
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.GetAnalyticsOld("extra_counts", "")
	CheckNoError(t, resp)

	rows, resp = th.SystemAdminClient.GetAnalyticsOld("", th.BasicTeam.Id)
	CheckNoError(t, resp)

	for _, row := range rows {
		if row.Name == "inactive_user_count" {
			assert.Equal(t, float64(-1), row.Value, "inactive user count should be -1 when team specified")
		}
	}

	rows2, resp2 := th.SystemAdminClient.GetAnalyticsOld("standard", "")
	CheckNoError(t, resp2)
	assert.Equal(t, "total_websocket_connections", rows2[5].Name)
	assert.Equal(t, float64(0), rows2[5].Value)

	WebSocketClient, err := th.CreateWebSocketClient()
	require.Nil(t, err)
	time.Sleep(100 * time.Millisecond)
	rows2, resp2 = th.SystemAdminClient.GetAnalyticsOld("standard", "")
	CheckNoError(t, resp2)
	assert.Equal(t, "total_websocket_connections", rows2[5].Name)
	assert.Equal(t, float64(1), rows2[5].Value)

	WebSocketClient.Close()

	rows2, resp2 = th.SystemAdminClient.GetAnalyticsOld("standard", "")
	CheckNoError(t, resp2)
	assert.Equal(t, "total_websocket_connections", rows2[5].Name)
	assert.Equal(t, float64(0), rows2[5].Value)

	Client.Logout()
	_, resp = Client.GetAnalyticsOld("", th.BasicTeam.Id)
	CheckUnauthorizedStatus(t, resp)
}

func TestS3TestConnection(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	s3Host := os.Getenv("CI_MINIO_HOST")
	if s3Host == "" {
		s3Host = "localhost"
	}

	s3Port := os.Getenv("CI_MINIO_PORT")
	if s3Port == "" {
		s3Port = "9000"
	}

	s3Endpoint := fmt.Sprintf("%s:%s", s3Host, s3Port)
	config := model.Config{
		FileSettings: model.FileSettings{
			DriverName:              model.NewString(model.IMAGE_DRIVER_S3),
			AmazonS3AccessKeyId:     model.NewString(model.MINIO_ACCESS_KEY),
			AmazonS3SecretAccessKey: model.NewString(model.MINIO_SECRET_KEY),
			AmazonS3Bucket:          model.NewString(""),
			AmazonS3Endpoint:        model.NewString(s3Endpoint),
			AmazonS3Region:          model.NewString(""),
			AmazonS3SSL:             model.NewBool(false),
		},
	}

	t.Run("as system user", func(t *testing.T) {
		_, resp := Client.TestS3Connection(&config)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("as system admin", func(t *testing.T) {
		_, resp := th.SystemAdminClient.TestS3Connection(&config)
		CheckBadRequestStatus(t, resp)
		require.Equal(t, resp.Error.Message, "S3 Bucket is required", "should return error - missing s3 bucket")
		// If this fails, check the test configuration to ensure minio is setup with the
		// `mattermost-test` bucket defined by model.MINIO_BUCKET.
		*config.FileSettings.AmazonS3Bucket = model.MINIO_BUCKET
		*config.FileSettings.AmazonS3Region = "us-east-1"
		_, resp = th.SystemAdminClient.TestS3Connection(&config)
		CheckOKStatus(t, resp)

		config.FileSettings.AmazonS3Region = model.NewString("")
		_, resp = th.SystemAdminClient.TestS3Connection(&config)
		CheckOKStatus(t, resp)

		config.FileSettings.AmazonS3Bucket = model.NewString("Wrong_bucket")
		_, resp = th.SystemAdminClient.TestS3Connection(&config)
		CheckInternalErrorStatus(t, resp)
		assert.Equal(t, "Unable to create bucket.", resp.Error.Message)

		*config.FileSettings.AmazonS3Bucket = "shouldcreatenewbucket"
		_, resp = th.SystemAdminClient.TestS3Connection(&config)
		CheckOKStatus(t, resp)
	})

	t.Run("as restricted system admin", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ExperimentalSettings.RestrictSystemAdmin = true })

		_, resp := th.SystemAdminClient.TestS3Connection(&config)
		CheckForbiddenStatus(t, resp)
	})

}

func TestSupportedTimezones(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client

	supportedTimezonesFromConfig := th.App.Timezones().GetSupported()
	supportedTimezones, resp := Client.GetSupportedTimezone()

	CheckNoError(t, resp)
	assert.Equal(t, supportedTimezonesFromConfig, supportedTimezones)
}

func TestRedirectLocation(t *testing.T) {
	expected := "https://mattermost.com/wp-content/themes/mattermostv2/img/logo-light.svg"

	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Location", expected)
		res.WriteHeader(http.StatusFound)
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()

	mockBitlyLink := testServer.URL

	th := Setup(t).InitBasic()
	defer th.TearDown()
	Client := th.Client
	enableLinkPreviews := *th.App.Config().ServiceSettings.EnableLinkPreviews
	defer func() {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableLinkPreviews = enableLinkPreviews })
	}()

	*th.App.Config().ServiceSettings.EnableLinkPreviews = true
	*th.App.Config().ServiceSettings.AllowedUntrustedInternalConnections = "127.0.0.1"

	_, resp := th.SystemAdminClient.GetRedirectLocation("https://mattermost.com/", "")
	CheckNoError(t, resp)

	_, resp = th.SystemAdminClient.GetRedirectLocation("", "")
	CheckBadRequestStatus(t, resp)

	actual, resp := th.SystemAdminClient.GetRedirectLocation(mockBitlyLink, "")
	CheckNoError(t, resp)
	assert.Equal(t, expected, actual)

	// Check cached value
	actual, resp = th.SystemAdminClient.GetRedirectLocation(mockBitlyLink, "")
	CheckNoError(t, resp)
	assert.Equal(t, expected, actual)

	*th.App.Config().ServiceSettings.EnableLinkPreviews = false
	actual, resp = th.SystemAdminClient.GetRedirectLocation("https://mattermost.com/", "")
	CheckNoError(t, resp)
	assert.Equal(t, actual, "")

	actual, resp = th.SystemAdminClient.GetRedirectLocation("", "")
	CheckNoError(t, resp)
	assert.Equal(t, actual, "")

	actual, resp = th.SystemAdminClient.GetRedirectLocation(mockBitlyLink, "")
	CheckNoError(t, resp)
	assert.Equal(t, actual, "")

	Client.Logout()
	_, resp = Client.GetRedirectLocation("", "")
	CheckUnauthorizedStatus(t, resp)
}

func TestSetServerBusy(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	const secs = 30

	t.Run("as system user", func(t *testing.T) {
		ok, resp := th.Client.SetServerBusy(secs)
		CheckForbiddenStatus(t, resp)
		require.False(t, ok, "should not set server busy due to no permission")
		require.False(t, th.App.Srv().Busy.IsBusy(), "server should not be marked busy")
	})

	t.Run("as system admin", func(t *testing.T) {
		ok, resp := th.SystemAdminClient.SetServerBusy(secs)
		CheckNoError(t, resp)
		require.True(t, ok, "should set server busy successfully")
		require.True(t, th.App.Srv().Busy.IsBusy(), "server should be marked busy")
	})
}

func TestSetServerBusyInvalidParam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("as system admin, invalid param", func(t *testing.T) {
		params := []int{-1, 0, MAX_SERVER_BUSY_SECONDS + 1}
		for _, p := range params {
			ok, resp := th.SystemAdminClient.SetServerBusy(p)
			CheckBadRequestStatus(t, resp)
			require.False(t, ok, "should not set server busy due to invalid param ", p)
			require.False(t, th.App.Srv().Busy.IsBusy(), "server should not be marked busy due to invalid param ", p)
		}
	})
}

func TestClearServerBusy(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.Srv().Busy.Set(time.Second * 30)
	t.Run("as system user", func(t *testing.T) {
		ok, resp := th.Client.ClearServerBusy()
		CheckForbiddenStatus(t, resp)
		require.False(t, ok, "should not clear server busy flag due to no permission.")
		require.True(t, th.App.Srv().Busy.IsBusy(), "server should be marked busy")
	})

	th.App.Srv().Busy.Set(time.Second * 30)
	t.Run("as system admin", func(t *testing.T) {
		ok, resp := th.SystemAdminClient.ClearServerBusy()
		CheckNoError(t, resp)
		require.True(t, ok, "should clear server busy flag successfully")
		require.False(t, th.App.Srv().Busy.IsBusy(), "server should not be marked busy")
	})
}

func TestGetServerBusy(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.Srv().Busy.Set(time.Second * 30)

	t.Run("as system user", func(t *testing.T) {
		_, resp := th.Client.GetServerBusy()
		CheckForbiddenStatus(t, resp)
	})

	t.Run("as system admin", func(t *testing.T) {
		sbs, resp := th.SystemAdminClient.GetServerBusy()
		expires := time.Unix(sbs.Expires, 0)
		CheckNoError(t, resp)
		require.Greater(t, expires.Unix(), time.Now().Unix())
	})
}

func TestGetServerBusyExpires(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.Srv().Busy.Set(time.Second * 30)

	t.Run("as system user", func(t *testing.T) {
		_, resp := th.Client.GetServerBusyExpires()
		CheckForbiddenStatus(t, resp)
	})

	t.Run("as system admin", func(t *testing.T) {
		expires, resp := th.SystemAdminClient.GetServerBusyExpires()
		CheckNoError(t, resp)
		require.Greater(t, expires.Unix(), time.Now().Unix())
	})
}

func TestServerBusy503(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.Srv().Busy.Set(time.Second * 30)

	t.Run("search users while busy", func(t *testing.T) {
		us := &model.UserSearch{Term: "test"}
		_, resp := th.SystemAdminClient.SearchUsers(us)
		CheckServiceUnavailableStatus(t, resp)
	})

	t.Run("search teams while busy", func(t *testing.T) {
		ts := &model.TeamSearch{}
		_, resp := th.SystemAdminClient.SearchTeams(ts)
		CheckServiceUnavailableStatus(t, resp)
	})

	t.Run("search channels while busy", func(t *testing.T) {
		cs := &model.ChannelSearch{}
		_, resp := th.SystemAdminClient.SearchChannels("foo", cs)
		CheckServiceUnavailableStatus(t, resp)
	})

	t.Run("search archived channels while busy", func(t *testing.T) {
		cs := &model.ChannelSearch{}
		_, resp := th.SystemAdminClient.SearchArchivedChannels("foo", cs)
		CheckServiceUnavailableStatus(t, resp)
	})

	th.App.Srv().Busy.Clear()

	t.Run("search users while not busy", func(t *testing.T) {
		us := &model.UserSearch{Term: "test"}
		_, resp := th.SystemAdminClient.SearchUsers(us)
		CheckNoError(t, resp)
	})
}

func TestPushNotificationAck(t *testing.T) {
	th := Setup(t).InitBasic()
	api := Init(th.Server, th.Server.AppOptions, th.Server.Router)
	session, _ := th.App.GetSession(th.Client.AuthToken)
	defer th.TearDown()
	t.Run("should return error when the ack body is not passed", func(t *testing.T) {
		handler := api.ApiHandler(pushNotificationAck)
		resp := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v4/notifications/ack", nil)
		req.Header.Set(model.HEADER_AUTH, "Bearer "+session.Token)

		handler.ServeHTTP(resp, req)
		assert.Equal(t, http.StatusBadRequest, resp.Code)
		assert.NotNil(t, resp.Body)
	})
}

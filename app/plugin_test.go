// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/mattermost/mattermost-server/v5/utils"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
)

func getHashedKey(key string) string {
	hash := sha256.New()
	hash.Write([]byte(key))
	return base64.StdEncoding.EncodeToString(hash.Sum(nil))
}

func TestPluginKeyValueStore(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	pluginId := "testpluginid"

	defer func() {
		assert.Nil(t, th.App.DeletePluginKey(pluginId, "key"))
		assert.Nil(t, th.App.DeletePluginKey(pluginId, "key2"))
		assert.Nil(t, th.App.DeletePluginKey(pluginId, "key3"))
		assert.Nil(t, th.App.DeletePluginKey(pluginId, "key4"))
	}()

	assert.Nil(t, th.App.SetPluginKey(pluginId, "key", []byte("test")))
	ret, err := th.App.GetPluginKey(pluginId, "key")
	assert.Nil(t, err)
	assert.Equal(t, []byte("test"), ret)

	// Test inserting over existing entries
	assert.Nil(t, th.App.SetPluginKey(pluginId, "key", []byte("test2")))
	ret, err = th.App.GetPluginKey(pluginId, "key")
	assert.Nil(t, err)
	assert.Equal(t, []byte("test2"), ret)

	// Test getting non-existent key
	ret, err = th.App.GetPluginKey(pluginId, "notakey")
	assert.Nil(t, err)
	assert.Nil(t, ret)

	// Test deleting non-existent keys.
	assert.Nil(t, th.App.DeletePluginKey(pluginId, "notrealkey"))

	// Verify behaviour for the old approach that involved storing the hashed keys.
	hashedKey2 := getHashedKey("key2")
	kv := &model.PluginKeyValue{
		PluginId: pluginId,
		Key:      hashedKey2,
		Value:    []byte("test"),
		ExpireAt: 0,
	}

	_, err = th.App.Srv().Store.Plugin().SaveOrUpdate(kv)
	assert.Nil(t, err)

	// Test fetch by keyname (this key does not exist but hashed key will be used for lookup)
	ret, err = th.App.GetPluginKey(pluginId, "key2")
	assert.Nil(t, err)
	assert.Equal(t, kv.Value, ret)

	// Test fetch by hashed keyname
	ret, err = th.App.GetPluginKey(pluginId, hashedKey2)
	assert.Nil(t, err)
	assert.Equal(t, kv.Value, ret)

	// Test ListKeys
	assert.Nil(t, th.App.SetPluginKey(pluginId, "key3", []byte("test3")))
	assert.Nil(t, th.App.SetPluginKey(pluginId, "key4", []byte("test4")))

	list, err := th.App.ListPluginKeys(pluginId, 0, 1)
	assert.Nil(t, err)
	assert.Equal(t, []string{"key"}, list)

	list, err = th.App.ListPluginKeys(pluginId, 1, 1)
	assert.Nil(t, err)
	assert.Equal(t, []string{"key3"}, list)

	list, err = th.App.ListPluginKeys(pluginId, 0, 4)
	assert.Nil(t, err)
	assert.Equal(t, []string{"key", "key3", "key4", hashedKey2}, list)

	list, err = th.App.ListPluginKeys(pluginId, 0, 2)
	assert.Nil(t, err)
	assert.Equal(t, []string{"key", "key3"}, list)

	list, err = th.App.ListPluginKeys(pluginId, 1, 2)
	assert.Nil(t, err)
	assert.Equal(t, []string{"key4", hashedKey2}, list)

	list, err = th.App.ListPluginKeys(pluginId, 2, 2)
	assert.Nil(t, err)
	assert.Equal(t, []string{}, list)

	// List Keys bad input
	list, err = th.App.ListPluginKeys(pluginId, 0, 0)
	assert.Nil(t, err)
	assert.Equal(t, []string{"key", "key3", "key4", hashedKey2}, list)

	list, err = th.App.ListPluginKeys(pluginId, 0, -1)
	assert.Nil(t, err)
	assert.Equal(t, []string{"key", "key3", "key4", hashedKey2}, list)

	list, err = th.App.ListPluginKeys(pluginId, -1, 1)
	assert.Nil(t, err)
	assert.Equal(t, []string{"key"}, list)

	list, err = th.App.ListPluginKeys(pluginId, -1, 0)
	assert.Nil(t, err)
	assert.Equal(t, []string{"key", "key3", "key4", hashedKey2}, list)
}

func TestPluginKeyValueStoreCompareAndSet(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	pluginId := "testpluginid"

	defer func() {
		assert.Nil(t, th.App.DeletePluginKey(pluginId, "key"))
	}()

	// Set using Set api for key2
	assert.Nil(t, th.App.SetPluginKey(pluginId, "key2", []byte("test")))
	ret, err := th.App.GetPluginKey(pluginId, "key2")
	assert.Nil(t, err)
	assert.Equal(t, []byte("test"), ret)

	// Attempt to insert value for key2
	updated, err := th.App.CompareAndSetPluginKey(pluginId, "key2", nil, []byte("test2"))
	assert.Nil(t, err)
	assert.False(t, updated)
	ret, err = th.App.GetPluginKey(pluginId, "key2")
	assert.Nil(t, err)
	assert.Equal(t, []byte("test"), ret)

	// Insert new value for key
	updated, err = th.App.CompareAndSetPluginKey(pluginId, "key", nil, []byte("test"))
	assert.Nil(t, err)
	assert.True(t, updated)
	ret, err = th.App.GetPluginKey(pluginId, "key")
	assert.Nil(t, err)
	assert.Equal(t, []byte("test"), ret)

	// Should fail to insert again
	updated, err = th.App.CompareAndSetPluginKey(pluginId, "key", nil, []byte("test3"))
	assert.Nil(t, err)
	assert.False(t, updated)
	ret, err = th.App.GetPluginKey(pluginId, "key")
	assert.Nil(t, err)
	assert.Equal(t, []byte("test"), ret)

	// Test updating using incorrect old value
	updated, err = th.App.CompareAndSetPluginKey(pluginId, "key", []byte("oldvalue"), []byte("test3"))
	assert.Nil(t, err)
	assert.False(t, updated)
	ret, err = th.App.GetPluginKey(pluginId, "key")
	assert.Nil(t, err)
	assert.Equal(t, []byte("test"), ret)

	// Test updating using correct old value
	updated, err = th.App.CompareAndSetPluginKey(pluginId, "key", []byte("test"), []byte("test2"))
	assert.Nil(t, err)
	assert.True(t, updated)
	ret, err = th.App.GetPluginKey(pluginId, "key")
	assert.Nil(t, err)
	assert.Equal(t, []byte("test2"), ret)
}

func TestPluginKeyValueStoreSetWithOptionsJSON(t *testing.T) {
	pluginId := "testpluginid"

	t.Run("storing a value without providing options works", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		result, err := th.App.SetPluginKeyWithOptions(pluginId, "key", []byte("value-1"), model.PluginKVSetOptions{})
		assert.True(t, result)
		assert.Nil(t, err)

		// and I can get it back!
		ret, err := th.App.GetPluginKey(pluginId, "key")
		assert.Nil(t, err)
		assert.Equal(t, []byte(`value-1`), ret)
	})

	t.Run("test that setting it atomic when it doesn't match doesn't change anything", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		err := th.App.SetPluginKey(pluginId, "key", []byte("value-1"))
		require.Nil(t, err)

		result, err := th.App.SetPluginKeyWithOptions(pluginId, "key", []byte("value-3"), model.PluginKVSetOptions{
			Atomic:   true,
			OldValue: []byte("value-2"),
		})
		assert.False(t, result)
		assert.Nil(t, err)

		// test that the value didn't change
		ret, err := th.App.GetPluginKey(pluginId, "key")
		assert.Nil(t, err)
		assert.Equal(t, []byte(`value-1`), ret)
	})

	t.Run("test the atomic change with the proper old value", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		err := th.App.SetPluginKey(pluginId, "key", []byte("value-2"))
		require.Nil(t, err)

		result, err := th.App.SetPluginKeyWithOptions(pluginId, "key", []byte("value-3"), model.PluginKVSetOptions{
			Atomic:   true,
			OldValue: []byte("value-2"),
		})
		assert.True(t, result)
		assert.Nil(t, err)

		// test that the value did change
		ret, err := th.App.GetPluginKey(pluginId, "key")
		assert.Nil(t, err)
		assert.Equal(t, []byte(`value-3`), ret)
	})

	t.Run("when new value is nil and old value matches with the current, it should delete the currently set value", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		// first set a value.
		result, err := th.App.SetPluginKeyWithOptions(pluginId, "nil-test-key-2", []byte("value-1"), model.PluginKVSetOptions{})
		require.Nil(t, err)
		require.True(t, result)

		// now it should delete the set value.
		result, err = th.App.SetPluginKeyWithOptions(pluginId, "nil-test-key-2", nil, model.PluginKVSetOptions{
			Atomic:   true,
			OldValue: []byte("value-1"),
		})
		assert.Nil(t, err)
		assert.True(t, result)

		ret, err := th.App.GetPluginKey(pluginId, "nil-test-key-2")
		assert.Nil(t, err)
		assert.Nil(t, ret)
	})

	t.Run("when new value is nil and there is a value set for the key already, it should delete the currently set value", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		// first set a value.
		result, err := th.App.SetPluginKeyWithOptions(pluginId, "nil-test-key-3", []byte("value-1"), model.PluginKVSetOptions{})
		require.Nil(t, err)
		require.True(t, result)

		// now it should delete the set value.
		result, err = th.App.SetPluginKeyWithOptions(pluginId, "nil-test-key-3", nil, model.PluginKVSetOptions{})
		assert.Nil(t, err)
		assert.True(t, result)

		// verify a nil value is returned
		ret, err := th.App.GetPluginKey(pluginId, "nil-test-key-3")
		assert.Nil(t, err)
		assert.Nil(t, ret)

		// verify the row is actually gone
		list, err := th.App.ListPluginKeys(pluginId, 0, 1)
		assert.Nil(t, err)
		assert.Empty(t, list)
	})

	t.Run("when old value is nil and there is no value set for the key before, it should set the new value", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		result, err := th.App.SetPluginKeyWithOptions(pluginId, "nil-test-key-4", []byte("value-1"), model.PluginKVSetOptions{
			Atomic:   true,
			OldValue: nil,
		})
		assert.Nil(t, err)
		assert.True(t, result)

		ret, err := th.App.GetPluginKey(pluginId, "nil-test-key-4")
		assert.Nil(t, err)
		assert.Equal(t, []byte("value-1"), ret)
	})

	t.Run("test that value is set and unset with ExpireInSeconds", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()

		result, err := th.App.SetPluginKeyWithOptions(pluginId, "key", []byte("value-1"), model.PluginKVSetOptions{
			ExpireInSeconds: 1,
		})
		assert.True(t, result)
		assert.Nil(t, err)

		// test that the value is set
		ret, err := th.App.GetPluginKey(pluginId, "key")
		assert.Nil(t, err)
		assert.Equal(t, []byte(`value-1`), ret)

		// test that the value is not longer
		time.Sleep(1500 * time.Millisecond)

		ret, err = th.App.GetPluginKey(pluginId, "key")
		assert.Nil(t, err)
		assert.Nil(t, ret)
	})
}

func TestServePluginRequest(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PluginSettings.Enable = false })

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/plugins/foo/bar", nil)
	th.App.ServePluginRequest(w, r)
	assert.Equal(t, http.StatusNotImplemented, w.Result().StatusCode)
}

func TestPrivateServePluginRequest(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	testCases := []struct {
		Description string
		ConfigFunc  func(cfg *model.Config)
		URL         string
		ExpectedURL string
	}{
		{
			"no subpath",
			func(cfg *model.Config) {},
			"/plugins/id/endpoint",
			"/endpoint",
		},
		{
			"subpath",
			func(cfg *model.Config) { *cfg.ServiceSettings.SiteURL += "/subpath" },
			"/subpath/plugins/id/endpoint",
			"/endpoint",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			th.App.UpdateConfig(testCase.ConfigFunc)
			expectedBody := []byte("body")
			request := httptest.NewRequest(http.MethodGet, testCase.URL, bytes.NewReader(expectedBody))
			recorder := httptest.NewRecorder()

			handler := func(context *plugin.Context, w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, testCase.ExpectedURL, r.URL.Path)

				body, _ := ioutil.ReadAll(r.Body)
				assert.Equal(t, expectedBody, body)
			}

			request = mux.SetURLVars(request, map[string]string{"plugin_id": "id"})

			th.App.servePluginRequest(recorder, request, handler)
		})
	}

}

func TestHandlePluginRequest(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.PluginSettings.Enable = false
		*cfg.ServiceSettings.EnableUserAccessTokens = true
	})

	token, err := th.App.CreateUserAccessToken(&model.UserAccessToken{
		UserId: th.BasicUser.Id,
	})
	require.Nil(t, err)

	var assertions func(*http.Request)
	router := mux.NewRouter()
	router.HandleFunc("/plugins/{plugin_id:[A-Za-z0-9\\_\\-\\.]+}/{anything:.*}", func(_ http.ResponseWriter, r *http.Request) {
		th.App.servePluginRequest(nil, r, func(_ *plugin.Context, _ http.ResponseWriter, r *http.Request) {
			assertions(r)
		})
	})

	r := httptest.NewRequest("GET", "/plugins/foo/bar", nil)
	r.Header.Add("Authorization", "Bearer "+token.Token)
	assertions = func(r *http.Request) {
		assert.Equal(t, "/bar", r.URL.Path)
		assert.Equal(t, th.BasicUser.Id, r.Header.Get("Mattermost-User-Id"))
	}
	router.ServeHTTP(nil, r)

	r = httptest.NewRequest("GET", "/plugins/foo/bar?a=b&access_token="+token.Token+"&c=d", nil)
	assertions = func(r *http.Request) {
		assert.Equal(t, "/bar", r.URL.Path)
		assert.Equal(t, "a=b&c=d", r.URL.RawQuery)
		assert.Equal(t, th.BasicUser.Id, r.Header.Get("Mattermost-User-Id"))
	}
	router.ServeHTTP(nil, r)

	r = httptest.NewRequest("GET", "/plugins/foo/bar?a=b&access_token=asdf&c=d", nil)
	assertions = func(r *http.Request) {
		assert.Equal(t, "/bar", r.URL.Path)
		assert.Equal(t, "a=b&c=d", r.URL.RawQuery)
		assert.Empty(t, r.Header.Get("Mattermost-User-Id"))
	}
	router.ServeHTTP(nil, r)
}

func TestGetPluginStatusesDisabled(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.PluginSettings.Enable = false
	})

	_, err := th.App.GetPluginStatuses()
	require.NotNil(t, err)
	require.EqualError(t, err, "GetPluginStatuses: Plugins have been disabled. Please check your logs for details., ")
}

func TestGetPluginStatuses(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.PluginSettings.Enable = true
	})

	pluginStatuses, err := th.App.GetPluginStatuses()
	require.Nil(t, err)
	require.NotNil(t, pluginStatuses)
}

func TestPluginSync(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	testCases := []struct {
		Description string
		ConfigFunc  func(cfg *model.Config)
	}{
		{
			"local",
			func(cfg *model.Config) {
				cfg.FileSettings.DriverName = model.NewString(model.IMAGE_DRIVER_LOCAL)
			},
		},
		{
			"s3",
			func(cfg *model.Config) {
				s3Host := os.Getenv("CI_MINIO_HOST")
				if s3Host == "" {
					s3Host = "localhost"
				}

				s3Port := os.Getenv("CI_MINIO_PORT")
				if s3Port == "" {
					s3Port = "9000"
				}

				s3Endpoint := fmt.Sprintf("%s:%s", s3Host, s3Port)
				cfg.FileSettings.DriverName = model.NewString(model.IMAGE_DRIVER_S3)
				cfg.FileSettings.AmazonS3AccessKeyId = model.NewString(model.MINIO_ACCESS_KEY)
				cfg.FileSettings.AmazonS3SecretAccessKey = model.NewString(model.MINIO_SECRET_KEY)
				cfg.FileSettings.AmazonS3Bucket = model.NewString(model.MINIO_BUCKET)
				cfg.FileSettings.AmazonS3Endpoint = model.NewString(s3Endpoint)
				cfg.FileSettings.AmazonS3Region = model.NewString("")
				cfg.FileSettings.AmazonS3SSL = model.NewBool(false)

			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Description, func(t *testing.T) {
			th.App.UpdateConfig(func(cfg *model.Config) {
				*cfg.PluginSettings.Enable = true
				testCase.ConfigFunc(cfg)
			})

			env := th.App.GetPluginsEnvironment()
			require.NotNil(t, env)

			path, _ := fileutils.FindDir("tests")

			t.Run("new bundle in the file store", func(t *testing.T) {
				th.App.UpdateConfig(func(cfg *model.Config) {
					*cfg.PluginSettings.RequirePluginSignature = false
				})

				fileReader, err := os.Open(filepath.Join(path, "testplugin.tar.gz"))
				require.NoError(t, err)
				defer fileReader.Close()

				_, appErr := th.App.WriteFile(fileReader, th.App.getBundleStorePath("testplugin"))
				checkNoError(t, appErr)

				appErr = th.App.SyncPlugins()
				checkNoError(t, appErr)

				// Check if installed
				pluginStatus, err := env.Statuses()
				require.Nil(t, err)
				require.Len(t, pluginStatus, 1)
				require.Equal(t, pluginStatus[0].PluginId, "testplugin")
			})

			t.Run("bundle removed from the file store", func(t *testing.T) {
				th.App.UpdateConfig(func(cfg *model.Config) {
					*cfg.PluginSettings.RequirePluginSignature = false
				})

				appErr := th.App.RemoveFile(th.App.getBundleStorePath("testplugin"))
				checkNoError(t, appErr)

				appErr = th.App.SyncPlugins()
				checkNoError(t, appErr)

				// Check if removed
				pluginStatus, err := env.Statuses()
				require.Nil(t, err)
				require.Empty(t, pluginStatus)
			})

			t.Run("plugin signatures required, no signature", func(t *testing.T) {
				th.App.UpdateConfig(func(cfg *model.Config) {
					*cfg.PluginSettings.RequirePluginSignature = true
				})

				pluginFileReader, err := os.Open(filepath.Join(path, "testplugin.tar.gz"))
				require.NoError(t, err)
				defer pluginFileReader.Close()
				_, appErr := th.App.WriteFile(pluginFileReader, th.App.getBundleStorePath("testplugin"))
				checkNoError(t, appErr)

				appErr = th.App.SyncPlugins()
				checkNoError(t, appErr)
				pluginStatus, err := env.Statuses()
				require.Nil(t, err)
				require.Len(t, pluginStatus, 0)
			})

			t.Run("plugin signatures required, wrong signature", func(t *testing.T) {
				th.App.UpdateConfig(func(cfg *model.Config) {
					*cfg.PluginSettings.RequirePluginSignature = true
				})

				signatureFileReader, err := os.Open(filepath.Join(path, "testplugin2.tar.gz.sig"))
				require.NoError(t, err)
				defer signatureFileReader.Close()
				_, appErr := th.App.WriteFile(signatureFileReader, th.App.getSignatureStorePath("testplugin"))
				checkNoError(t, appErr)

				appErr = th.App.SyncPlugins()
				checkNoError(t, appErr)

				pluginStatus, err := env.Statuses()
				require.Nil(t, err)
				require.Len(t, pluginStatus, 0)
			})

			t.Run("plugin signatures required, correct signature", func(t *testing.T) {
				th.App.UpdateConfig(func(cfg *model.Config) {
					*cfg.PluginSettings.RequirePluginSignature = true
				})

				key, err := os.Open(filepath.Join(path, "development-private-key.asc"))
				require.NoError(t, err)
				appErr := th.App.AddPublicKey("pub_key", key)
				checkNoError(t, appErr)

				signatureFileReader, err := os.Open(filepath.Join(path, "testplugin.tar.gz.sig"))
				require.NoError(t, err)
				defer signatureFileReader.Close()
				_, appErr = th.App.WriteFile(signatureFileReader, th.App.getSignatureStorePath("testplugin"))
				checkNoError(t, appErr)

				appErr = th.App.SyncPlugins()
				checkNoError(t, appErr)

				pluginStatus, err := env.Statuses()
				require.Nil(t, err)
				require.Len(t, pluginStatus, 1)
				require.Equal(t, pluginStatus[0].PluginId, "testplugin")

				appErr = th.App.DeletePublicKey("pub_key")
				checkNoError(t, appErr)

				appErr = th.App.RemovePlugin("testplugin")
				checkNoError(t, appErr)
			})
		})
	}
}

func TestPluginPanicLogs(t *testing.T) {
	t.Run("should panic", func(t *testing.T) {
		th := Setup(t).InitBasic()
		defer th.TearDown()
		tearDown, _, _ := SetAppEnvironmentWithPlugins(t, []string{
			`
		package main

		import (
			"github.com/mattermost/mattermost-server/v5/plugin"
			"github.com/mattermost/mattermost-server/v5/model"
		)

		type MyPlugin struct {
			plugin.MattermostPlugin
		}

		func (p *MyPlugin) MessageWillBePosted(c *plugin.Context, post *model.Post) (*model.Post, string) {
			panic("some text from panic")
			return nil, ""
		}

		func main() {
			plugin.ClientMain(&MyPlugin{})
		}
		`,
		}, th.App, th.App.NewPluginAPI)
		defer tearDown()

		post := &model.Post{
			UserId:    th.BasicUser.Id,
			ChannelId: th.BasicChannel.Id,
			Message:   "message_",
			CreateAt:  model.GetMillis() - 10000,
		}
		_, err := th.App.CreatePost(post, th.BasicChannel, false)
		assert.Nil(t, err)

		logs := th.LogBuffer.String()
		assert.True(t, strings.Contains(logs, "some text from panic"))
	})
}

func TestProcessPrepackagedPlugins(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	testsPath, _ := fileutils.FindDir("tests")
	prepackagedPluginsPath := filepath.Join(testsPath, prepackagedPluginsDir)
	fileErr := os.Mkdir(prepackagedPluginsPath, os.ModePerm)
	require.NoError(t, fileErr)
	defer os.RemoveAll(prepackagedPluginsPath)

	prepackagedPluginsDir, found := fileutils.FindDir(prepackagedPluginsPath)
	require.True(t, found, "failed to find prepackaged plugins directory")

	testPluginPath := filepath.Join(testsPath, "testplugin.tar.gz")
	fileErr = utils.CopyFile(testPluginPath, filepath.Join(prepackagedPluginsDir, "testplugin.tar.gz"))
	require.NoError(t, fileErr)

	t.Run("automatic, enabled plugin, no signature", func(t *testing.T) {
		// Install the plugin and enable
		pluginBytes, err := ioutil.ReadFile(testPluginPath)
		require.NoError(t, err)
		require.NotNil(t, pluginBytes)

		manifest, appErr := th.App.installPluginLocally(bytes.NewReader(pluginBytes), nil, installPluginLocallyAlways)
		require.Nil(t, appErr)
		require.Equal(t, "testplugin", manifest.Id)

		env := th.App.GetPluginsEnvironment()

		activatedManifest, activated, err := env.Activate(manifest.Id)
		require.NoError(t, err)
		require.True(t, activated)
		require.Equal(t, manifest, activatedManifest)

		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.PluginSettings.Enable = true
			*cfg.PluginSettings.AutomaticPrepackagedPlugins = true
		})

		plugins := th.App.processPrepackagedPlugins(prepackagedPluginsDir)
		require.Len(t, plugins, 1)
		require.Equal(t, plugins[0].Manifest.Id, "testplugin")
		require.Empty(t, plugins[0].Signature, 0)

		pluginStatus, err := env.Statuses()
		require.NoError(t, err)
		require.Len(t, pluginStatus, 1)
		require.Equal(t, pluginStatus[0].PluginId, "testplugin")

		appErr = th.App.RemovePlugin("testplugin")
		checkNoError(t, appErr)

		pluginStatus, err = env.Statuses()
		require.NoError(t, err)
		require.Len(t, pluginStatus, 0)
	})

	t.Run("automatic, not enabled plugin", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.PluginSettings.Enable = true
			*cfg.PluginSettings.AutomaticPrepackagedPlugins = true
		})

		env := th.App.GetPluginsEnvironment()

		plugins := th.App.processPrepackagedPlugins(prepackagedPluginsDir)
		require.Len(t, plugins, 1)
		require.Equal(t, plugins[0].Manifest.Id, "testplugin")
		require.Empty(t, plugins[0].Signature, 0)

		pluginStatus, err := env.Statuses()
		require.NoError(t, err)
		require.Empty(t, pluginStatus, 0)
	})

	t.Run("automatic, multiple plugins with signatures, not enabled", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.PluginSettings.Enable = true
			*cfg.PluginSettings.AutomaticPrepackagedPlugins = true
		})

		env := th.App.GetPluginsEnvironment()

		// Add signature
		testPluginSignaturePath := filepath.Join(testsPath, "testplugin.tar.gz.sig")
		err := utils.CopyFile(testPluginSignaturePath, filepath.Join(prepackagedPluginsDir, "testplugin.tar.gz.sig"))
		require.NoError(t, err)

		// Add second plugin
		testPlugin2Path := filepath.Join(testsPath, "testplugin2.tar.gz")
		err = utils.CopyFile(testPlugin2Path, filepath.Join(prepackagedPluginsDir, "testplugin2.tar.gz"))
		require.NoError(t, err)

		testPlugin2SignaturePath := filepath.Join(testsPath, "testplugin2.tar.gz.sig")
		err = utils.CopyFile(testPlugin2SignaturePath, filepath.Join(prepackagedPluginsDir, "testplugin2.tar.gz.sig"))
		require.NoError(t, err)

		plugins := th.App.processPrepackagedPlugins(prepackagedPluginsDir)
		require.Len(t, plugins, 2)
		require.Contains(t, []string{"testplugin", "testplugin2"}, plugins[0].Manifest.Id)
		require.NotEmpty(t, plugins[0].Signature)
		require.Contains(t, []string{"testplugin", "testplugin2"}, plugins[1].Manifest.Id)
		require.NotEmpty(t, plugins[1].Signature)

		pluginStatus, err := env.Statuses()
		require.NoError(t, err)
		require.Len(t, pluginStatus, 0)
	})

	t.Run("automatic, multiple plugins with signatures, one enabled", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.PluginSettings.Enable = true
			*cfg.PluginSettings.AutomaticPrepackagedPlugins = true
		})

		env := th.App.GetPluginsEnvironment()

		// Add signature
		testPluginSignaturePath := filepath.Join(testsPath, "testplugin.tar.gz.sig")
		err := utils.CopyFile(testPluginSignaturePath, filepath.Join(prepackagedPluginsDir, "testplugin.tar.gz.sig"))
		require.NoError(t, err)

		// Install first plugin and enable
		pluginBytes, err := ioutil.ReadFile(testPluginPath)
		require.NoError(t, err)
		require.NotNil(t, pluginBytes)

		manifest, appErr := th.App.installPluginLocally(bytes.NewReader(pluginBytes), nil, installPluginLocallyAlways)
		require.Nil(t, appErr)
		require.Equal(t, "testplugin", manifest.Id)

		activatedManifest, activated, err := env.Activate(manifest.Id)
		require.NoError(t, err)
		require.True(t, activated)
		require.Equal(t, manifest, activatedManifest)

		// Add second plugin
		testPlugin2Path := filepath.Join(testsPath, "testplugin2.tar.gz")
		err = utils.CopyFile(testPlugin2Path, filepath.Join(prepackagedPluginsDir, "testplugin2.tar.gz"))
		require.NoError(t, err)

		testPlugin2SignaturePath := filepath.Join(testsPath, "testplugin2.tar.gz.sig")
		err = utils.CopyFile(testPlugin2SignaturePath, filepath.Join(prepackagedPluginsDir, "testplugin2.tar.gz.sig"))
		require.NoError(t, err)

		plugins := th.App.processPrepackagedPlugins(prepackagedPluginsDir)
		require.Len(t, plugins, 2)
		require.Contains(t, []string{"testplugin", "testplugin2"}, plugins[0].Manifest.Id)
		require.NotEmpty(t, plugins[0].Signature)
		require.Contains(t, []string{"testplugin", "testplugin2"}, plugins[1].Manifest.Id)
		require.NotEmpty(t, plugins[1].Signature)

		pluginStatus, err := env.Statuses()
		require.NoError(t, err)
		require.Len(t, pluginStatus, 1)
		require.Equal(t, pluginStatus[0].PluginId, "testplugin")

		appErr = th.App.RemovePlugin("testplugin")
		checkNoError(t, appErr)

		pluginStatus, err = env.Statuses()
		require.NoError(t, err)
		require.Len(t, pluginStatus, 0)
	})

	t.Run("non-automatic, multiple plugins", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.PluginSettings.Enable = true
			*cfg.PluginSettings.AutomaticPrepackagedPlugins = false
		})

		env := th.App.GetPluginsEnvironment()

		testPlugin2Path := filepath.Join(testsPath, "testplugin2.tar.gz")
		err := utils.CopyFile(testPlugin2Path, filepath.Join(prepackagedPluginsDir, "testplugin2.tar.gz"))
		require.NoError(t, err)

		testPlugin2SignaturePath := filepath.Join(testsPath, "testplugin2.tar.gz.sig")
		err = utils.CopyFile(testPlugin2SignaturePath, filepath.Join(prepackagedPluginsDir, "testplugin2.tar.gz.sig"))
		require.NoError(t, err)

		plugins := th.App.processPrepackagedPlugins(prepackagedPluginsDir)
		require.Len(t, plugins, 2)
		require.Contains(t, []string{"testplugin", "testplugin2"}, plugins[0].Manifest.Id)
		require.NotEmpty(t, plugins[0].Signature)
		require.Contains(t, []string{"testplugin", "testplugin2"}, plugins[1].Manifest.Id)
		require.NotEmpty(t, plugins[1].Signature)

		pluginStatus, err := env.Statuses()
		require.NoError(t, err)
		require.Len(t, pluginStatus, 0)
	})
}

// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package plugin_test

import (
	"strconv"
	"testing"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/mattermost/mattermost-server/v5/plugin/plugintest"
	"github.com/stretchr/testify/assert"
)

func TestKVGetJSON(t *testing.T) {
	t.Run("incompatible server version", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.1.0")

		p := &plugin.HelpersImpl{API: api}
		var dat map[string]interface{}

		ok, err := p.KVGetJSON("test-key", dat)

		api.AssertExpectations(t)
		assert.False(t, ok)
		assert.Error(t, err)
		assert.Equal(t, "incompatible server version for plugin, minimum required version: 5.2.0, current version: 5.1.0", err.Error())
	})

	t.Run("KVGet error", func(t *testing.T) {
		p := &plugin.HelpersImpl{}

		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.2.0")
		api.On("KVGet", "test-key").Return(nil, &model.AppError{})
		p.API = api

		var dat map[string]interface{}

		ok, err := p.KVGetJSON("test-key", dat)
		api.AssertExpectations(t)
		assert.False(t, ok)
		assert.Error(t, err)
		assert.Nil(t, dat)
	})

	t.Run("unknown key", func(t *testing.T) {
		p := &plugin.HelpersImpl{}

		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.2.0")
		api.On("KVGet", "test-key").Return(nil, nil)
		p.API = api

		var dat map[string]interface{}

		ok, err := p.KVGetJSON("test-key", dat)
		api.AssertExpectations(t)
		assert.False(t, ok)
		assert.NoError(t, err)
		assert.Nil(t, dat)
	})

	t.Run("malformed JSON", func(t *testing.T) {
		p := &plugin.HelpersImpl{}

		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.2.0")
		api.On("KVGet", "test-key").Return([]byte(`{{:}"val-a": 10}`), nil)
		p.API = api

		var dat map[string]interface{}

		ok, err := p.KVGetJSON("test-key", &dat)
		api.AssertExpectations(t)
		assert.False(t, ok)
		assert.Error(t, err)
		assert.Nil(t, dat)
	})

	t.Run("wellformed JSON", func(t *testing.T) {
		p := &plugin.HelpersImpl{}

		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.2.0")
		api.On("KVGet", "test-key").Return([]byte(`{"val-a": 10}`), nil)
		p.API = api

		var dat map[string]interface{}

		ok, err := p.KVGetJSON("test-key", &dat)
		assert.True(t, ok)
		api.AssertExpectations(t)
		assert.NoError(t, err)
		assert.Equal(t, map[string]interface{}{
			"val-a": float64(10),
		}, dat)
	})
}

func TestKVSetJSON(t *testing.T) {
	t.Run("incompatible server version", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.1.0")

		p := &plugin.HelpersImpl{API: api}

		err := p.KVSetJSON("test-key", map[string]interface{}{
			"val-a": float64(10),
		})

		api.AssertExpectations(t)
		assert.Error(t, err)
		assert.Equal(t, "incompatible server version for plugin, minimum required version: 5.2.0, current version: 5.1.0", err.Error())
	})

	t.Run("JSON marshal error", func(t *testing.T) {
		api := &plugintest.API{}
		api.AssertNotCalled(t, "KVSet")
		api.On("GetServerVersion").Return("5.2.0")

		p := &plugin.HelpersImpl{API: api}

		err := p.KVSetJSON("test-key", func() {})
		api.AssertExpectations(t)
		assert.Error(t, err)
	})

	t.Run("KVSet error", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("KVSet", "test-key", []byte(`{"val-a":10}`)).Return(&model.AppError{})
		api.On("GetServerVersion").Return("5.2.0")

		p := &plugin.HelpersImpl{API: api}

		err := p.KVSetJSON("test-key", map[string]interface{}{
			"val-a": float64(10),
		})

		api.AssertExpectations(t)
		assert.Error(t, err)
	})

	t.Run("marshallable struct", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("KVSet", "test-key", []byte(`{"val-a":10}`)).Return(nil)
		api.On("GetServerVersion").Return("5.2.0")

		p := &plugin.HelpersImpl{API: api}

		err := p.KVSetJSON("test-key", map[string]interface{}{
			"val-a": float64(10),
		})

		api.AssertExpectations(t)
		assert.NoError(t, err)
	})
}

func TestKVCompareAndSetJSON(t *testing.T) {
	t.Run("incompatible server version", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.10.0")
		p := &plugin.HelpersImpl{API: api}

		ok, err := p.KVCompareAndSetJSON("test-key", nil, map[string]interface{}{
			"val-b": 20,
		})

		assert.Equal(t, false, ok)
		assert.Error(t, err)
		assert.Equal(t, "incompatible server version for plugin, minimum required version: 5.12.0, current version: 5.10.0", err.Error())
	})

	t.Run("old value JSON marshal error", func(t *testing.T) {
		api := &plugintest.API{}
		api.AssertNotCalled(t, "KVCompareAndSet")
		api.On("GetServerVersion").Return("5.12.0")
		p := &plugin.HelpersImpl{API: api}

		ok, err := p.KVCompareAndSetJSON("test-key", func() {}, map[string]interface{}{})

		api.AssertExpectations(t)
		assert.Equal(t, false, ok)
		assert.Error(t, err)
	})

	t.Run("new value JSON marshal error", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.12.0")
		api.AssertNotCalled(t, "KVCompareAndSet")

		p := &plugin.HelpersImpl{API: api}

		ok, err := p.KVCompareAndSetJSON("test-key", map[string]interface{}{}, func() {})

		api.AssertExpectations(t)
		assert.False(t, ok)
		assert.Error(t, err)
	})

	t.Run("KVCompareAndSet error", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.12.0")
		api.On("KVCompareAndSet", "test-key", []byte(`{"val-a":10}`), []byte(`{"val-b":20}`)).Return(false, &model.AppError{})
		p := &plugin.HelpersImpl{API: api}

		ok, err := p.KVCompareAndSetJSON("test-key", map[string]interface{}{
			"val-a": 10,
		}, map[string]interface{}{
			"val-b": 20,
		})

		api.AssertExpectations(t)
		assert.False(t, ok)
		assert.Error(t, err)
	})

	t.Run("old value nil", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.12.0")
		api.On("KVCompareAndSet", "test-key", []byte(nil), []byte(`{"val-b":20}`)).Return(true, nil)
		p := &plugin.HelpersImpl{API: api}

		ok, err := p.KVCompareAndSetJSON("test-key", nil, map[string]interface{}{
			"val-b": 20,
		})

		api.AssertExpectations(t)
		assert.True(t, ok)
		assert.NoError(t, err)
	})

	t.Run("old value non-nil", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.12.0")
		api.On("KVCompareAndSet", "test-key", []byte(`{"val-a":10}`), []byte(`{"val-b":20}`)).Return(true, nil)
		p := &plugin.HelpersImpl{API: api}

		ok, err := p.KVCompareAndSetJSON("test-key", map[string]interface{}{
			"val-a": 10,
		}, map[string]interface{}{
			"val-b": 20,
		})

		api.AssertExpectations(t)
		assert.True(t, ok)
		assert.NoError(t, err)
	})

	t.Run("new value nil", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.12.0")
		api.On("KVCompareAndSet", "test-key", []byte(`{"val-a":10}`), []byte(nil)).Return(true, nil)
		p := &plugin.HelpersImpl{API: api}

		ok, err := p.KVCompareAndSetJSON("test-key", map[string]interface{}{
			"val-a": 10,
		}, nil)

		api.AssertExpectations(t)
		assert.True(t, ok)
		assert.NoError(t, err)
	})
}

func TestKVCompareAndDeleteJSON(t *testing.T) {
	t.Run("incompatible server version", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.10.0")
		p := &plugin.HelpersImpl{API: api}

		ok, err := p.KVCompareAndDeleteJSON("test-key", map[string]interface{}{
			"val-a": 10,
		})

		assert.Equal(t, false, ok)
		assert.Error(t, err)
		assert.Equal(t, "incompatible server version for plugin, minimum required version: 5.16.0, current version: 5.10.0", err.Error())
	})

	t.Run("old value JSON marshal error", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.16.0")
		api.AssertNotCalled(t, "KVCompareAndDelete")
		p := &plugin.HelpersImpl{API: api}

		ok, err := p.KVCompareAndDeleteJSON("test-key", func() {})

		api.AssertExpectations(t)
		assert.Equal(t, false, ok)
		assert.Error(t, err)
	})

	t.Run("KVCompareAndDelete error", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.16.0")
		api.On("KVCompareAndDelete", "test-key", []byte(`{"val-a":10}`)).Return(false, &model.AppError{})
		p := &plugin.HelpersImpl{API: api}

		ok, err := p.KVCompareAndDeleteJSON("test-key", map[string]interface{}{
			"val-a": 10,
		})

		api.AssertExpectations(t)
		assert.False(t, ok)
		assert.Error(t, err)
	})

	t.Run("old value nil", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.16.0")
		api.On("KVCompareAndDelete", "test-key", []byte(nil)).Return(true, nil)
		p := &plugin.HelpersImpl{API: api}

		ok, err := p.KVCompareAndDeleteJSON("test-key", nil)

		api.AssertExpectations(t)
		assert.True(t, ok)
		assert.NoError(t, err)
	})

	t.Run("old value non-nil", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.16.0")
		api.On("KVCompareAndDelete", "test-key", []byte(`{"val-a":10}`)).Return(true, nil)
		p := &plugin.HelpersImpl{API: api}

		ok, err := p.KVCompareAndDeleteJSON("test-key", map[string]interface{}{
			"val-a": 10,
		})

		api.AssertExpectations(t)
		assert.True(t, ok)
		assert.NoError(t, err)
	})
}

func TestKVSetWithExpiryJSON(t *testing.T) {
	t.Run("incompatible server version", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.4.0")

		p := &plugin.HelpersImpl{API: api}

		err := p.KVSetWithExpiryJSON("test-key", map[string]interface{}{
			"val-a": float64(10),
		}, 100)

		api.AssertExpectations(t)
		assert.Error(t, err)
		assert.Equal(t, "incompatible server version for plugin, minimum required version: 5.6.0, current version: 5.4.0", err.Error())
	})

	t.Run("JSON marshal error", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.6.0")
		api.AssertNotCalled(t, "KVSetWithExpiry")

		p := &plugin.HelpersImpl{API: api}

		err := p.KVSetWithExpiryJSON("test-key", func() {}, 100)

		api.AssertExpectations(t)
		assert.Error(t, err)
	})

	t.Run("KVSetWithExpiry error", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.6.0")
		api.On("KVSetWithExpiry", "test-key", []byte(`{"val-a":10}`), int64(100)).Return(&model.AppError{})
		p := &plugin.HelpersImpl{API: api}

		err := p.KVSetWithExpiryJSON("test-key", map[string]interface{}{
			"val-a": float64(10),
		}, 100)

		api.AssertExpectations(t)
		assert.Error(t, err)
	})

	t.Run("wellformed JSON", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.6.0")
		api.On("KVSetWithExpiry", "test-key", []byte(`{"val-a":10}`), int64(100)).Return(nil)

		p := &plugin.HelpersImpl{API: api}

		err := p.KVSetWithExpiryJSON("test-key", map[string]interface{}{
			"val-a": float64(10),
		}, 100)

		api.AssertExpectations(t)
		assert.NoError(t, err)
	})
}

func TestKVListWithOptions(t *testing.T) {
	t.Run("incompatible server version", func(t *testing.T) {
		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.1.0")

		p := &plugin.HelpersImpl{API: api}

		keys, err := p.KVListWithOptions()

		api.AssertExpectations(t)
		assert.Nil(t, keys)
		assert.Error(t, err)
		assert.Equal(t, "incompatible server version for plugin, minimum required version: 5.6.0, current version: 5.1.0", err.Error())
	})

	t.Run("KVList error", func(t *testing.T) {
		p := &plugin.HelpersImpl{}

		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.6.0")
		api.On("KVList", 0, 100).Return([]string{}, &model.AppError{})
		p.API = api

		keys, err := p.KVListWithOptions()
		api.AssertExpectations(t)
		assert.Empty(t, keys)
		assert.Error(t, err)
	})

	t.Run("No keys", func(t *testing.T) {
		p := &plugin.HelpersImpl{}

		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.6.0")
		api.On("KVList", 0, 100).Return(nil, nil)
		p.API = api

		keys, err := p.KVListWithOptions()
		api.AssertExpectations(t)
		assert.Empty(t, keys)
		assert.Nil(t, err)
	})

	t.Run("Basic Success, one page", func(t *testing.T) {
		p := &plugin.HelpersImpl{}

		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.6.0")
		api.On("KVList", 0, 100).Return([]string{"key1", "key2"}, nil)
		p.API = api

		keys, err := p.KVListWithOptions()
		api.AssertExpectations(t)
		assert.ElementsMatch(t, keys, []string{"key1", "key2"})
		assert.Nil(t, err)
	})

	t.Run("Basic Success, two page", func(t *testing.T) {
		p := &plugin.HelpersImpl{}

		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.6.0")
		api.On("KVList", 0, 100).Return(getKeys(100), nil)
		api.On("KVList", 1, 100).Return([]string{"key100"}, nil)
		p.API = api

		keys, err := p.KVListWithOptions()
		api.AssertExpectations(t)
		assert.ElementsMatch(t, keys, getKeys(101))
		assert.Nil(t, err)
	})

	t.Run("error on second page", func(t *testing.T) {
		p := &plugin.HelpersImpl{}

		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.6.0")
		api.On("KVList", 0, 100).Return(getKeys(100), nil)
		api.On("KVList", 1, 100).Return([]string{"key100"}, &model.AppError{})
		p.API = api

		keys, err := p.KVListWithOptions()
		api.AssertExpectations(t)
		assert.Empty(t, keys)
		assert.Error(t, err)
	})

	t.Run("success, two page, filter prefix, one", func(t *testing.T) {
		p := &plugin.HelpersImpl{}

		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.6.0")
		api.On("KVList", 0, 100).Return(getKeys(100), nil)
		api.On("KVList", 1, 100).Return([]string{"key100"}, nil)
		p.API = api

		keys, err := p.KVListWithOptions(plugin.WithPrefix("key99"))
		api.AssertExpectations(t)
		assert.ElementsMatch(t, keys, []string{"key99"})
		assert.Nil(t, err)
	})

	t.Run("success, two page, filter prefix, all", func(t *testing.T) {
		p := &plugin.HelpersImpl{}

		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.6.0")
		api.On("KVList", 0, 100).Return(getKeys(100), nil)
		api.On("KVList", 1, 100).Return([]string{"key100"}, nil)
		p.API = api

		keys, err := p.KVListWithOptions(plugin.WithPrefix("notkey"))
		api.AssertExpectations(t)
		assert.Empty(t, keys)
		assert.Nil(t, err)
	})

	t.Run("success, two page, filter prefix, none", func(t *testing.T) {
		p := &plugin.HelpersImpl{}

		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.6.0")
		api.On("KVList", 0, 100).Return(getKeys(100), nil)
		api.On("KVList", 1, 100).Return([]string{"key100"}, nil)
		p.API = api

		keys, err := p.KVListWithOptions(plugin.WithPrefix("key"))
		api.AssertExpectations(t)
		assert.ElementsMatch(t, keys, getKeys(101))
		assert.Nil(t, err)
	})

	t.Run("success, two page, checker func, one", func(t *testing.T) {
		p := &plugin.HelpersImpl{}

		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.6.0")
		api.On("KVList", 0, 100).Return(getKeys(100), nil)
		api.On("KVList", 1, 100).Return([]string{"key100"}, nil)
		p.API = api

		check := func(key string) (bool, error) {
			if key == "key1" {
				return true, nil
			}
			return false, nil
		}

		keys, err := p.KVListWithOptions(plugin.WithChecker(check))
		api.AssertExpectations(t)
		assert.ElementsMatch(t, keys, []string{"key1"})
		assert.Nil(t, err)
	})

	t.Run("success, two page, checker func, all", func(t *testing.T) {
		p := &plugin.HelpersImpl{}

		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.6.0")
		api.On("KVList", 0, 100).Return(getKeys(100), nil)
		api.On("KVList", 1, 100).Return([]string{"key100"}, nil)
		p.API = api

		check := func(key string) (bool, error) {
			return false, nil
		}

		keys, err := p.KVListWithOptions(plugin.WithChecker(check))
		api.AssertExpectations(t)
		assert.Empty(t, keys)
		assert.Nil(t, err)
	})

	t.Run("success, two page, checker func, none", func(t *testing.T) {
		p := &plugin.HelpersImpl{}

		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.6.0")
		api.On("KVList", 0, 100).Return(getKeys(100), nil)
		api.On("KVList", 1, 100).Return([]string{"key100"}, nil)
		p.API = api

		check := func(key string) (bool, error) {
			return true, nil
		}

		keys, err := p.KVListWithOptions(plugin.WithChecker(check))
		api.AssertExpectations(t)
		assert.ElementsMatch(t, keys, getKeys(101))
		assert.Nil(t, err)
	})

	t.Run("error, checker func", func(t *testing.T) {
		p := &plugin.HelpersImpl{}

		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.6.0")
		api.On("KVList", 0, 100).Return([]string{"key1"}, nil)
		p.API = api

		check := func(key string) (bool, error) {
			return true, &model.AppError{}
		}

		keys, err := p.KVListWithOptions(plugin.WithChecker(check))
		api.AssertExpectations(t)
		assert.Empty(t, keys)
		assert.Error(t, err)
	})

	t.Run("success, filter and checker func, partial on both", func(t *testing.T) {
		p := &plugin.HelpersImpl{}

		api := &plugintest.API{}
		api.On("GetServerVersion").Return("5.6.0")
		api.On("KVList", 0, 100).Return([]string{"key1", "key2", "notkey3", "key4", "key5"}, nil)
		p.API = api

		check := func(key string) (bool, error) {
			if key == "key1" || key == "key5" {
				return false, nil
			}
			return true, nil
		}

		keys, err := p.KVListWithOptions(plugin.WithPrefix("key"), plugin.WithChecker(check))
		api.AssertExpectations(t)
		assert.ElementsMatch(t, keys, []string{"key2", "key4"})
		assert.Nil(t, err)
	})
}

func getKeys(count int) []string {
	ret := make([]string, count)
	for i := 0; i < count; i++ {
		ret[i] = "key" + strconv.Itoa(i)
	}
	return ret
}

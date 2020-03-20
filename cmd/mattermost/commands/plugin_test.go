// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.
package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mattermost/mattermost-server/v5/config"
	"github.com/mattermost/mattermost-server/v5/utils/fileutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlugin(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	cfg := th.Config()
	*cfg.PluginSettings.EnableUploads = true
	*cfg.PluginSettings.Directory = "./test-plugins"
	*cfg.PluginSettings.ClientDirectory = "./test-client-plugins"
	th.SetConfig(cfg)

	err := os.MkdirAll("./test-plugins", os.ModePerm)
	require.Nil(t, err)
	err = os.MkdirAll("./test-client-plugins", os.ModePerm)
	require.Nil(t, err)

	path, _ := fileutils.FindDir("tests")

	output := th.CheckCommand(t, "plugin", "add", filepath.Join(path, "testplugin.tar.gz"))
	assert.Contains(t, output, "Added plugin:")
	output = th.CheckCommand(t, "plugin", "enable", "testplugin")
	assert.Contains(t, output, "Enabled plugin: testplugin")

	fs, err := config.NewFileStore(th.ConfigPath(), false)
	require.Nil(t, err)
	require.NotNil(t, fs.Get().PluginSettings.PluginStates["testplugin"])
	assert.True(t, fs.Get().PluginSettings.PluginStates["testplugin"].Enable)
	fs.Close()

	output = th.CheckCommand(t, "plugin", "disable", "testplugin")
	assert.Contains(t, output, "Disabled plugin: testplugin")
	fs, err = config.NewFileStore(th.ConfigPath(), false)
	require.Nil(t, err)
	require.NotNil(t, fs.Get().PluginSettings.PluginStates["testplugin"])
	assert.False(t, fs.Get().PluginSettings.PluginStates["testplugin"].Enable)
	fs.Close()

	th.CheckCommand(t, "plugin", "list")

	th.CheckCommand(t, "plugin", "delete", "testplugin")
}

func TestPluginPublicKeys(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	cfg := th.Config()
	cfg.PluginSettings.SignaturePublicKeyFiles = []string{"public-key"}
	th.SetConfig(cfg)

	output := th.CheckCommand(t, "plugin", "keys")
	assert.Contains(t, output, "public-key")
	assert.NotContains(t, output, "Plugin name:")
}

func TestPluginPublicKeyDetails(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	cfg := th.Config()
	cfg.PluginSettings.SignaturePublicKeyFiles = []string{"public-key"}

	th.SetConfig(cfg)

	output := th.CheckCommand(t, "plugin", "keys", "--verbose", "true")
	assert.Contains(t, output, "Plugin name: public-key")
	output = th.CheckCommand(t, "plugin", "keys", "--verbose")
	assert.Contains(t, output, "Plugin name: public-key")
}

func TestAddPluginPublicKeys(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	cfg := th.Config()
	cfg.PluginSettings.SignaturePublicKeyFiles = []string{"public-key"}
	th.SetConfig(cfg)

	err := th.RunCommand(t, "plugin", "keys", "add", "pk1")
	assert.NotNil(t, err)
}

func TestDeletePluginPublicKeys(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	cfg := th.Config()
	cfg.PluginSettings.SignaturePublicKeyFiles = []string{"pk1"}
	th.SetConfig(cfg)

	output := th.CheckCommand(t, "plugin", "keys", "delete", "pk1")
	assert.Contains(t, output, "Deleted public key: pk1")
}

func TestPluginPublicKeysFlow(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	path, _ := fileutils.FindDir("tests")
	name := "test-public-key.plugin.gpg"
	output := th.CheckCommand(t, "plugin", "keys", "add", filepath.Join(path, name))
	assert.Contains(t, output, "Added public key: "+filepath.Join(path, name))

	output = th.CheckCommand(t, "plugin", "keys")
	assert.Contains(t, output, name)
	assert.NotContains(t, output, "Plugin name:")

	output = th.CheckCommand(t, "plugin", "keys", "--verbose")
	assert.Contains(t, output, "Plugin name: "+name)

	output = th.CheckCommand(t, "plugin", "keys", "delete", name)
	assert.Contains(t, output, "Deleted public key: "+name)
}

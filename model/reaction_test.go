// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReactionIsValid(t *testing.T) {
	tests := []struct {
		// reaction
		reaction Reaction
		// error message to print
		errMsg string
		// should there be an error
		shouldErr bool
	}{
		{
			reaction: Reaction{
				UserId:    NewId(),
				PostId:    NewId(),
				EmojiName: "emoji",
				CreateAt:  GetMillis(),
			},
			errMsg:    "",
			shouldErr: false,
		},
		{
			reaction: Reaction{
				UserId:    "",
				PostId:    NewId(),
				EmojiName: "emoji",
				CreateAt:  GetMillis(),
			},
			errMsg:    "user id should be invalid",
			shouldErr: true,
		},
		{
			reaction: Reaction{
				UserId:    "1234garbage",
				PostId:    NewId(),
				EmojiName: "emoji",
				CreateAt:  GetMillis(),
			},
			errMsg:    "user id should be invalid",
			shouldErr: true,
		},
		{
			reaction: Reaction{
				UserId:    NewId(),
				PostId:    "",
				EmojiName: "emoji",
				CreateAt:  GetMillis(),
			},
			errMsg:    "post id should be invalid",
			shouldErr: true,
		},
		{
			reaction: Reaction{
				UserId:    NewId(),
				PostId:    "1234garbage",
				EmojiName: "emoji",
				CreateAt:  GetMillis(),
			},
			errMsg:    "post id should be invalid",
			shouldErr: true,
		},
		{
			reaction: Reaction{
				UserId:    NewId(),
				PostId:    NewId(),
				EmojiName: strings.Repeat("a", 64),
				CreateAt:  GetMillis(),
			},
			errMsg:    "",
			shouldErr: false,
		},
		{
			reaction: Reaction{
				UserId:    NewId(),
				PostId:    NewId(),
				EmojiName: "emoji-",
				CreateAt:  GetMillis(),
			},
			errMsg:    "",
			shouldErr: false,
		},
		{
			reaction: Reaction{
				UserId:    NewId(),
				PostId:    NewId(),
				EmojiName: "emoji_",
				CreateAt:  GetMillis(),
			},
			errMsg:    "",
			shouldErr: false,
		},
		{
			reaction: Reaction{
				UserId:    NewId(),
				PostId:    NewId(),
				EmojiName: "+1",
				CreateAt:  GetMillis(),
			},
			errMsg:    "",
			shouldErr: false,
		},
		{
			reaction: Reaction{
				UserId:    NewId(),
				PostId:    NewId(),
				EmojiName: "emoji:",
				CreateAt:  GetMillis(),
			},
			errMsg:    "",
			shouldErr: true,
		},
		{
			reaction: Reaction{
				UserId:    NewId(),
				PostId:    NewId(),
				EmojiName: "",
				CreateAt:  GetMillis(),
			},
			errMsg:    "emoji name should be invalid",
			shouldErr: true,
		},
		{
			reaction: Reaction{
				UserId:    NewId(),
				PostId:    NewId(),
				EmojiName: strings.Repeat("a", 65),
				CreateAt:  GetMillis(),
			},
			errMsg:    "emoji name should be invalid",
			shouldErr: true,
		},
		{
			reaction: Reaction{
				UserId:    NewId(),
				PostId:    NewId(),
				EmojiName: "emoji",
				CreateAt:  0,
			},
			errMsg:    "create at should be invalid",
			shouldErr: true,
		},
	}

	for _, test := range tests {
		err := test.reaction.IsValid()
		if test.shouldErr {
			// there should be an error here
			require.NotNil(t, err, test.errMsg)
		} else {
			// err should be nil here
			require.Nil(t, err, test.errMsg)
		}
	}
}

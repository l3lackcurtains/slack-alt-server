// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package migrations

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
)

type AdvancedPermissionsPhase2Progress struct {
	CurrentTable  string `json:"current_table"`
	LastTeamId    string `json:"last_team_id"`
	LastChannelId string `json:"last_channel_id"`
	LastUserId    string `json:"last_user"`
}

func (p *AdvancedPermissionsPhase2Progress) ToJson() string {
	b, _ := json.Marshal(p)
	return string(b)
}

func AdvancedPermissionsPhase2ProgressFromJson(data io.Reader) *AdvancedPermissionsPhase2Progress {
	var o *AdvancedPermissionsPhase2Progress
	json.NewDecoder(data).Decode(&o)
	return o
}

func (p *AdvancedPermissionsPhase2Progress) IsValid() bool {
	if len(p.LastChannelId) != 26 {
		return false
	}

	if len(p.LastTeamId) != 26 {
		return false
	}

	if len(p.LastUserId) != 26 {
		return false
	}

	switch p.CurrentTable {
	case "TeamMembers":
	case "ChannelMembers":
	default:
		return false
	}

	return true
}

func (worker *Worker) runAdvancedPermissionsPhase2Migration(lastDone string) (bool, string, *model.AppError) {
	var progress *AdvancedPermissionsPhase2Progress
	if len(lastDone) == 0 {
		// Haven't started the migration yet.
		progress = new(AdvancedPermissionsPhase2Progress)
		progress.CurrentTable = "TeamMembers"
		progress.LastChannelId = strings.Repeat("0", 26)
		progress.LastTeamId = strings.Repeat("0", 26)
		progress.LastUserId = strings.Repeat("0", 26)
	} else {
		progress = AdvancedPermissionsPhase2ProgressFromJson(strings.NewReader(lastDone))
		if !progress.IsValid() {
			return false, "", model.NewAppError("MigrationsWorker.runAdvancedPermissionsPhase2Migration", "migrations.worker.run_advanced_permissions_phase_2_migration.invalid_progress", map[string]interface{}{"progress": progress.ToJson()}, "", http.StatusInternalServerError)
		}
	}

	if progress.CurrentTable == "TeamMembers" {
		// Run a TeamMembers migration batch.
		if result, err := worker.app.Srv().Store.Team().MigrateTeamMembers(progress.LastTeamId, progress.LastUserId); err != nil {
			return false, progress.ToJson(), err
		} else {
			if result == nil {
				// We haven't progressed. That means that we've reached the end of this stage of the migration, and should now advance to the next stage.
				progress.LastUserId = strings.Repeat("0", 26)
				progress.CurrentTable = "ChannelMembers"
				return false, progress.ToJson(), nil
			}

			progress.LastTeamId = result["TeamId"]
			progress.LastUserId = result["UserId"]
		}
	} else if progress.CurrentTable == "ChannelMembers" {
		// Run a ChannelMembers migration batch.
		if data, err := worker.app.Srv().Store.Channel().MigrateChannelMembers(progress.LastChannelId, progress.LastUserId); err != nil {
			return false, progress.ToJson(), err
		} else {
			if data == nil {
				// We haven't progressed. That means we've reached the end of this final stage of the migration.

				return true, progress.ToJson(), nil
			}

			progress.LastChannelId = data["ChannelId"]
			progress.LastUserId = data["UserId"]
		}
	}

	return false, progress.ToJson(), nil
}

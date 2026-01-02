package dropbox

import (
	"time"
)

type Error struct {
	Summary string `json:"error_summary"`
}

type Cursor struct {
	Cursor string
}

type (
	Update struct {
		ListFolder struct {
			Accounts []Account `json:"accounts"`
		} `json:"list_folder"`
	}

	Account string
)

type (
	Folder struct {
		Cursor  string `json:"cursor"`
		Entries []File `json:"entries"`
	}

	File struct {
		Tag                      string          `json:".tag"`
		ClientModified           time.Time       `json:"client_modified"`
		ContentHash              string          `json:"content_hash"`
		FileLockInfo             FileLockInfo    `json:"file_lock_info"`
		HasExplicitSharedMembers bool            `json:"has_explicit_shared_members"`
		ID                       string          `json:"id"`
		IsDownloadable           bool            `json:"is_downloadable"`
		Name                     string          `json:"name"`
		PathDisplay              string          `json:"path_display"`
		PathLower                string          `json:"path_lower"`
		PropertyGroups           []PropertyGroup `json:"property_groups"`
		Rev                      string          `json:"rev"`
		ServerModified           time.Time       `json:"server_modified"`
		SharingInfo              SharingInfo     `json:"sharing_info"`
		Size                     int             `json:"size"`
	}

	SharingInfo struct {
		ModifiedBy           string `json:"modified_by"`
		ParentSharedFolderID string `json:"parent_shared_folder_id"`
		ReadOnly             bool   `json:"read_only"`
	}

	FileLockInfo struct {
		// TODO
	}

	PropertyGroup struct {
		// TODO
	}
)

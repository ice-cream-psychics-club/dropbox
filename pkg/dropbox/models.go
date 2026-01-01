package dropbox

import "time"

type Error struct {
	Summary string `json:"error_summary"`
}

//         {
//             ".tag": "folder",
//             "id": "id:a4ayc_80_OEAAAAAAAAAXz",
//             "name": "math",
//             "path_display": "/Homework/math",
//             "path_lower": "/homework/math",
//             "property_groups": [
//                 {
//                     "fields": [
//                         {
//                             "name": "Security Policy",
//                             "value": "Confidential"
//                         }
//                     ],
//                     "template_id": "ptid:1a5n2i6d3OYEAAAAAAAAAYa"
//                 }
//             ],
//             "sharing_info": {
//                 "no_access": false,
//                 "parent_shared_folder_id": "84528192421",
//                 "read_only": false,
//                 "traverse_only": false
//             }
//         }
//     ],
//     "has_more": false
// }

type Folder struct {
	Cursor  string `json:"cursor"`
	Entries []File `json:"entries"`
}

type Entry struct {
	Tag            string    `json:".tag"`
	ClientModified time.Time `json:"client_modified"`
}

// TODO: decide how to handle Folder vs Folder

type File struct {
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

type SharingInfo struct {
	ModifiedBy           string `json:"modified_by"`
	ParentSharedFolderID string `json:"parent_shared_folder_id"`
	ReadOnly             bool   `json:"read_only"`
}

type FileLockInfo struct {
	// TODO
}

type PropertyGroup struct {
	// TODO
}

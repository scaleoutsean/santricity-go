// Copyright 2026 NetApp, Inc. All Rights Reserved.

package santricity

// SnapshotGroupCreateRequest is the payload for creating a Snapshot Group
type SnapshotGroupCreateRequest struct {
	BaseMappableObjectId string  `json:"baseMappableObjectId"` // The Ref of the volume/resource to snapshot
	Name                 string  `json:"name"`
	RepositoryPercentage float64 `json:"repositoryPercentage"`
	WarningThreshold     int     `json:"warningThreshold"`
	AutoDeleteLimit      int     `json:"autoDeleteLimit"`
	FullPolicy           string  `json:"fullPolicy"`              // "purgepit" (auto-delete oldest) or "failbasewrites"
	StoragePoolId        string  `json:"storagePoolId,omitempty"` // Optional: Pool to create repository on
}

// SnapshotImageCreateRequest is the payload for creating a Snapshot Image (Instant Snapshot)
type SnapshotImageCreateRequest struct {
	GroupId string `json:"groupId"` // The Ref of the Snapshot Group
}

// SnapshotVolumeCreateRequest is the payload for creating a Snapshot Volume (Linked Clone)
type SnapshotVolumeCreateRequest struct {
	SnapshotImageId      string  `json:"snapshotImageId"`      // The ID of the snapshot image to clone
	Name                 string  `json:"name"`                 // The name of the new volume
	ViewMode             string  `json:"viewMode,omitempty"`   // "readOnly" (default) or "readWrite"
	RepositoryPercentage float64 `json:"repositoryPercentage"` // Size relative to base volume
	RepositoryPoolId     string  `json:"repositoryPoolId,omitempty"`
	FullThreshold        int     `json:"fullThreshold,omitempty"`
}

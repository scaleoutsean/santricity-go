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

// ConsistencyGroupCreateRequest represents the request to create a new Consistency Group (Snapshot Group container)
type ConsistencyGroupCreateRequest struct {
	Name                     string `json:"name"`
	FullWarnThresholdPercent int    `json:"fullWarnThresholdPercent,omitempty"`
	AutoDeleteThreshold      int    `json:"autoDeleteThreshold,omitempty"`
	RepositoryFullPolicy     string `json:"repositoryFullPolicy,omitempty"` // "purgepit" etc
	RollbackPriority         string `json:"rollbackPriority,omitempty"`
}

// ConsistencyGroupMemberAddRequest represents the request to add a volume to a CG
type ConsistencyGroupMemberAddRequest struct {
	VolumeId          string  `json:"volumeId"`
	RepositoryPoolId  string  `json:"repositoryPoolId,omitempty"`
	RepositoryPercent float64 `json:"repositoryPercent,omitempty"`
	ScanMedia         bool    `json:"scanMedia,omitempty"`
	ValidateParity    bool    `json:"validateParity,omitempty"`
}

// ConsistencyGroupViewCreateRequest represents the request to create a View/Clone of a CG
type ConsistencyGroupViewCreateRequest struct {
	Name              string  `json:"name"`
	RepositoryPoolId  string  `json:"repositoryPoolId,omitempty"`
	PitId             string  `json:"pitId,omitempty"`
	PitSequenceNumber int64   `json:"pitSequenceNumber,omitempty"` // Note: int64 to match format "int64" in swagger
	AccessMode        string  `json:"accessMode,omitempty"`        // "readOnly" or "readWrite"
	RepositoryPercent float64 `json:"repositoryPercent,omitempty"`
	ScanMedia         bool    `json:"scanMedia,omitempty"`
	ValidateParity    bool    `json:"validateParity,omitempty"`
}

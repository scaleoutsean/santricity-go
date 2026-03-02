// Copyright 2026 NetApp, Inc. All Rights Reserved.

package santricity

import (
	"context"
	"encoding/json"
	"fmt"
)

// CreateSnapshotGroup creates a new snapshot group for a volume.
func (c *Client) CreateSnapshotGroup(ctx context.Context, request SnapshotGroupCreateRequest) (*SnapshotGroup, error) {
	// Endpoint: /storage-systems/{system-id}/snapshot-groups
	// Swagger ID: newSnapshotGroup

	systemID, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/storage-systems/%s/snapshot-groups", systemID)

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	_, responseBody, err := c.InvokeAPI(ctx, jsonBody, "POST", path)
	if err != nil {
		return nil, err
	}

	var group SnapshotGroup
	err = json.Unmarshal(responseBody, &group)
	if err != nil {
		return nil, err
	}
	return &group, nil
}

// CreateSnapshotImage creates a new snapshot (PiT) in an existing Snapshot Group.
func (c *Client) CreateSnapshotImage(ctx context.Context, request SnapshotImageCreateRequest) (*SnapshotImage, error) {
	// Endpoint: /storage-systems/{system-id}/snapshot-images

	systemID, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/storage-systems/%s/snapshot-images", systemID)

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	_, responseBody, err := c.InvokeAPI(ctx, jsonBody, "POST", path)
	if err != nil {
		return nil, err
	}

	var image SnapshotImage
	err = json.Unmarshal(responseBody, &image)
	if err != nil {
		return nil, err
	}
	return &image, nil
}

// CreateSnapshotVolume creates a new Snapshot Volume (Linked Clone).
func (c *Client) CreateSnapshotVolume(ctx context.Context, request SnapshotVolumeCreateRequest) (*SnapshotVolume, error) {
	// Endpoint: /storage-systems/{system-id}/snapshot-volumes

	systemID, err := c.Connect(ctx)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/storage-systems/%s/snapshot-volumes", systemID)

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	_, responseBody, err := c.InvokeAPI(ctx, jsonBody, "POST", path)
	if err != nil {
		return nil, err
	}

	var volume SnapshotVolume
	err = json.Unmarshal(responseBody, &volume)
	if err != nil {
		return nil, err
	}

	return &volume, nil
}

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

	if _, err := c.Connect(ctx); err != nil {
		return nil, err
	}
	path := "/snapshot-groups"

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	resp, responseBody, err := c.InvokeAPI(ctx, jsonBody, "POST", path)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("failed to create snapshot group: status %d, body: %s", resp.StatusCode, string(responseBody))
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

	if _, err := c.Connect(ctx); err != nil {
		return nil, err
	}
	path := "/snapshot-images"

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	resp, responseBody, err := c.InvokeAPI(ctx, jsonBody, "POST", path)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("failed to create snapshot image: status %d, body: %s", resp.StatusCode, string(responseBody))
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

	if _, err := c.Connect(ctx); err != nil {
		return nil, err
	}
	path := "/snapshot-volumes"

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	resp, responseBody, err := c.InvokeAPI(ctx, jsonBody, "POST", path)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("failed to create snapshot volume: status %d, body: %s", resp.StatusCode, string(responseBody))
	}

	var volume SnapshotVolume
	err = json.Unmarshal(responseBody, &volume)
	if err != nil {
		return nil, err
	}

	return &volume, nil
}

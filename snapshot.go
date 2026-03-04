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

// RollbackSnapshotImage initiates a rollback of a volume to a specific Snapshot Image (PiT).
// CAUTION: This overwrites the base volume with the snapshot data.
func (c *Client) RollbackSnapshotImage(ctx context.Context, imageRef string) error {
	// Endpoint: /storage-systems/{system-id}/symbol/startPITRollback
	// This uses the legacy Symbol API proxy endpoint.

	if _, err := c.Connect(ctx); err != nil {
		return err
	}
	path := "/symbol/startPITRollback"

	req := SnapshotRollbackRequest{
		PitRef: []string{imageRef},
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, responseBody, err := c.InvokeAPI(ctx, jsonBody, "POST", path)
	if err != nil {
		return err
	}

	// The Symbol API often returns "ok" as a raw string or JSON "ok"
	// But standard REST semantic is 200/204.
	// We need to check if response code is success.
	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("failed to start rollback: status %d, body: %s", resp.StatusCode, string(responseBody))
	}

	// We can trust the user to monitor progress via GetVolume -> underlying Action status
	return nil
}

// DeleteSnapshotGroup deletes a snapshot group.
func (c *Client) DeleteSnapshotGroup(ctx context.Context, id string) error {
	// Endpoint: /storage-systems/{system-id}/snapshot-groups/{id}

	if _, err := c.Connect(ctx); err != nil {
		return err
	}
	path := fmt.Sprintf("/snapshot-groups/%s", id)

	resp, responseBody, err := c.InvokeAPI(ctx, nil, "DELETE", path)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("failed to delete snapshot group: status %d, body: %s", resp.StatusCode, string(responseBody))
	}
	return nil
}

// DeleteSnapshotImage deletes a snapshot image (PiT).
func (c *Client) DeleteSnapshotImage(ctx context.Context, id string) error {
	// Endpoint: /storage-systems/{system-id}/snapshot-images/{id}

	if _, err := c.Connect(ctx); err != nil {
		return err
	}
	path := fmt.Sprintf("/snapshot-images/%s", id)

	resp, responseBody, err := c.InvokeAPI(ctx, nil, "DELETE", path)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("failed to delete snapshot image: status %d, body: %s", resp.StatusCode, string(responseBody))
	}
	return nil
}

// DeleteSnapshotVolume deletes a snapshot volume (linked clone).
func (c *Client) DeleteSnapshotVolume(ctx context.Context, id string) error {
	// Endpoint: /storage-systems/{system-id}/snapshot-volumes/{id}

	if _, err := c.Connect(ctx); err != nil {
		return err
	}
	path := fmt.Sprintf("/snapshot-volumes/%s", id)

	resp, responseBody, err := c.InvokeAPI(ctx, nil, "DELETE", path)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("failed to delete snapshot volume: status %d, body: %s", resp.StatusCode, string(responseBody))
	}
	return nil
}

// GetSnapshotGroup returns a snapshot group by ID.
func (c *Client) GetSnapshotGroup(ctx context.Context, id string) (*SnapshotGroup, error) {
	// Endpoint: /storage-systems/{system-id}/snapshot-groups/{id}
	if _, err := c.Connect(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/snapshot-groups/%s", id)

	resp, responseBody, err := c.InvokeAPI(ctx, nil, "GET", path)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 404 {
		return nil, nil // Not found
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get snapshot group: status %d, body: %s", resp.StatusCode, string(responseBody))
	}

	var group SnapshotGroup
	err = json.Unmarshal(responseBody, &group)
	if err != nil {
		return nil, err
	}
	return &group, nil
}

// GetSnapshotImage returns a snapshot image (PiT) by ID.
func (c *Client) GetSnapshotImage(ctx context.Context, id string) (*SnapshotImage, error) {
	// Endpoint: /storage-systems/{system-id}/snapshot-images/{id}
	if _, err := c.Connect(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/snapshot-images/%s", id)

	resp, responseBody, err := c.InvokeAPI(ctx, nil, "GET", path)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 404 {
		return nil, nil // Not found
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get snapshot image: status %d, body: %s", resp.StatusCode, string(responseBody))
	}

	var image SnapshotImage
	err = json.Unmarshal(responseBody, &image)
	if err != nil {
		return nil, err
	}
	return &image, nil
}

// GetSnapshotVolume returns a snapshot volume (linked clone) by ID.
func (c *Client) GetSnapshotVolume(ctx context.Context, id string) (*SnapshotVolume, error) {
	// Endpoint: /storage-systems/{system-id}/snapshot-volumes/{id}
	if _, err := c.Connect(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/snapshot-volumes/%s", id)

	resp, responseBody, err := c.InvokeAPI(ctx, nil, "GET", path)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 404 {
		return nil, nil // Not found
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get snapshot volume: status %d, body: %s", resp.StatusCode, string(responseBody))
	}

	var volume SnapshotVolume
	err = json.Unmarshal(responseBody, &volume)
	if err != nil {
		return nil, err
	}
	return &volume, nil
}

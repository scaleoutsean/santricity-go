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

// CreateConsistencyGroup creates a new Consistency Group (Snapshot Group Container).
func (c *Client) CreateConsistencyGroup(ctx context.Context, request ConsistencyGroupCreateRequest) (*ConsistencyGroup, error) {
	// Endpoint: /storage-systems/{system-id}/consistency-groups
	if _, err := c.Connect(ctx); err != nil {
		return nil, err
	}
	path := "/consistency-groups"

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	resp, responseBody, err := c.InvokeAPI(ctx, jsonBody, "POST", path)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("failed to create consistency group: status %d, body: %s", resp.StatusCode, string(responseBody))
	}

	var group ConsistencyGroup
	err = json.Unmarshal(responseBody, &group)
	if err != nil {
		return nil, err
	}
	return &group, nil
}

// AddConsistencyGroupMember adds a volume member to a Consistency Group.
func (c *Client) AddConsistencyGroupMember(ctx context.Context, cgID string, request ConsistencyGroupMemberAddRequest) (*ConsistencyGroupMember, error) {
	// Endpoint: /storage-systems/{system-id}/consistency-groups/{cg-id}/member-volumes
	if _, err := c.Connect(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/consistency-groups/%s/member-volumes", cgID)

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	resp, responseBody, err := c.InvokeAPI(ctx, jsonBody, "POST", path)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("failed to add consistency group member: status %d, body: %s", resp.StatusCode, string(responseBody))
	}

	var member ConsistencyGroupMember
	err = json.Unmarshal(responseBody, &member)
	if err != nil {
		return nil, err
	}
	return &member, nil
}

// CreateConsistencyGroupSnapshot creates a new snapshot (PiT) for a Consistency Group.
func (c *Client) CreateConsistencyGroupSnapshot(ctx context.Context, cgID string) ([]SnapshotImage, error) {
	// Endpoint: /storage-systems/{system-id}/consistency-groups/{cg-id}/snapshots

	if _, err := c.Connect(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/consistency-groups/%s/snapshots", cgID)

	// No body needed for newConsistencyGroupSnapshot based on API analysis
	resp, responseBody, err := c.InvokeAPI(ctx, nil, "POST", path)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("failed to create consistency group snapshot: status %d, body: %s", resp.StatusCode, string(responseBody))
	}

	var images []SnapshotImage
	err = json.Unmarshal(responseBody, &images)
	if err != nil {
		return nil, err
	}
	return images, nil
}

// CreateConsistencyGroupView creates a new Consistency Group View (Linked Clone).
func (c *Client) CreateConsistencyGroupView(ctx context.Context, cgID string, request ConsistencyGroupViewCreateRequest) (*ConsistencyGroupView, error) {
	// Endpoint: /storage-systems/{system-id}/consistency-groups/{cg-id}/views

	if _, err := c.Connect(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/consistency-groups/%s/views", cgID)

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	resp, responseBody, err := c.InvokeAPI(ctx, jsonBody, "POST", path)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("failed to create consistency group view: status %d, body: %s", resp.StatusCode, string(responseBody))
	}

	var view ConsistencyGroupView
	err = json.Unmarshal(responseBody, &view)
	if err != nil {
		return nil, err
	}
	return &view, nil
}

// DeleteConsistencyGroup deletes a Consistency Group.
func (c *Client) DeleteConsistencyGroup(ctx context.Context, cgID string) error {
	// Endpoint: /storage-systems/{system-id}/consistency-groups/{id}
	if _, err := c.Connect(ctx); err != nil {
		return err
	}
	path := fmt.Sprintf("/consistency-groups/%s", cgID)

	resp, responseBody, err := c.InvokeAPI(ctx, nil, "DELETE", path)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("failed to delete consistency group: status %d, body: %s", resp.StatusCode, string(responseBody))
	}

	return nil
}

// RemoveConsistencyGroupMember removes a volume from a Consistency Group.
func (c *Client) RemoveConsistencyGroupMember(ctx context.Context, cgID string, memberVolumeID string) error {
	// Endpoint: /storage-systems/{system-id}/consistency-groups/{id}/members/{memberId}
	if _, err := c.Connect(ctx); err != nil {
		return err
	}
	path := fmt.Sprintf("/consistency-groups/%s/members/%s", cgID, memberVolumeID)

	resp, responseBody, err := c.InvokeAPI(ctx, nil, "DELETE", path)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("failed to remove consistency group member: status %d, body: %s", resp.StatusCode, string(responseBody))
	}

	return nil
}

// GetConcatRepositoryVolumes returns a list of all concatenated repository volumes.
func (c *Client) GetConcatRepositoryVolumes(ctx context.Context) ([]ConcatRepositoryVolume, error) {
	// Endpoint: /storage-systems/{system-id}/repositories/concat
	if _, err := c.Connect(ctx); err != nil {
		return nil, err
	}
	path := "/repositories/concat"

	resp, responseBody, err := c.InvokeAPI(ctx, nil, "GET", path)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get concat repository volumes: status %d, body: %s", resp.StatusCode, string(responseBody))
	}

	var concatVols []ConcatRepositoryVolume
	err = json.Unmarshal(responseBody, &concatVols)
	if err != nil {
		return nil, err
	}

	return concatVols, nil
}

// GetConcatRepositoryVolume returns a specific concatenated repository volume by ID.
func (c *Client) GetConcatRepositoryVolume(ctx context.Context, id string) (*ConcatRepositoryVolume, error) {
	// Endpoint: /storage-systems/{system-id}/repositories/concat/{id}
	if _, err := c.Connect(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/repositories/concat/%s", id)

	resp, responseBody, err := c.InvokeAPI(ctx, nil, "GET", path)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get concat repository volume: status %d, body: %s", resp.StatusCode, string(responseBody))
	}

	var concatVol ConcatRepositoryVolume
	err = json.Unmarshal(responseBody, &concatVol)
	if err != nil {
		return nil, err
	}

	return &concatVol, nil
}

// DeleteConsistencyGroupView deletes a Consistency Group View.
func (c *Client) DeleteConsistencyGroupView(ctx context.Context, cgID string, viewID string) error {
	// Endpoint: /storage-systems/{system-id}/consistency-groups/{cg-id}/views/{viewId}
	if _, err := c.Connect(ctx); err != nil {
		return err
	}
	path := fmt.Sprintf("/consistency-groups/%s/views/%s", cgID, viewID)

	resp, responseBody, err := c.InvokeAPI(ctx, nil, "DELETE", path)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("failed to delete consistency group view: status %d, body: %s", resp.StatusCode, string(responseBody))
	}

	return nil
}

// GetConsistencyGroup returns a specific Consistency Group by ID.
func (c *Client) GetConsistencyGroup(ctx context.Context, id string) (*ConsistencyGroup, error) {
	// Endpoint: /storage-systems/{system-id}/consistency-groups/{id}
	if _, err := c.Connect(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/consistency-groups/%s", id)

	resp, responseBody, err := c.InvokeAPI(ctx, nil, "GET", path)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 404 {
		return nil, nil // Return nil, nil for Terraform ReadContext missing check
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get consistency group: status %d, body: %s", resp.StatusCode, string(responseBody))
	}

	var group ConsistencyGroup
	err = json.Unmarshal(responseBody, &group)
	if err != nil {
		return nil, err
	}

	return &group, nil
}

// GetConsistencyGroupMember returns a specific volume member of a Consistency Group.
func (c *Client) GetConsistencyGroupMember(ctx context.Context, cgID string, volumeID string) (*ConsistencyGroupMember, error) {
	// Endpoint: /storage-systems/{system-id}/consistency-groups/{cg-id}/member-volumes/{volumeRef}
	if _, err := c.Connect(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/consistency-groups/%s/member-volumes/%s", cgID, volumeID)

	resp, responseBody, err := c.InvokeAPI(ctx, nil, "GET", path)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 404 {
		return nil, nil
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get consistency group member: status %d, body: %s", resp.StatusCode, string(responseBody))
	}

	var member ConsistencyGroupMember
	err = json.Unmarshal(responseBody, &member)
	if err != nil {
		return nil, err
	}

	return &member, nil
}

// GetConsistencyGroupSnapshot returns snapshots of a given sequence number for a Consistency Group.
func (c *Client) GetConsistencyGroupSnapshot(ctx context.Context, cgID string, sequenceNumber string) ([]SnapshotImage, error) {
	// Endpoint: /storage-systems/{system-id}/consistency-groups/{cg-id}/snapshots/{sequenceNumber}
	if _, err := c.Connect(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/consistency-groups/%s/snapshots/%s", cgID, sequenceNumber)

	resp, responseBody, err := c.InvokeAPI(ctx, nil, "GET", path)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 404 {
		return nil, nil
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get consistency group snapshot: status %d, body: %s", resp.StatusCode, string(responseBody))
	}

	var images []SnapshotImage
	err = json.Unmarshal(responseBody, &images)
	if err != nil {
		return nil, err
	}

	return images, nil
}

// DeleteConsistencyGroupSnapshot deletes consistency group snapshots associated with a sequence number.
func (c *Client) DeleteConsistencyGroupSnapshot(ctx context.Context, cgID string, sequenceNumber string) error {
	if _, err := c.Connect(ctx); err != nil {
		return err
	}
	path := fmt.Sprintf("/consistency-groups/%s/snapshots/%s", cgID, sequenceNumber)

	resp, responseBody, err := c.InvokeAPI(ctx, nil, "DELETE", path)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("failed to delete consistency group snapshot: status %d, body: %s", resp.StatusCode, string(responseBody))
	}

	return nil
}

// GetConsistencyGroupView returns a specific Consistency Group View by ID.
func (c *Client) GetConsistencyGroupView(ctx context.Context, cgID string, viewID string) (*ConsistencyGroupView, error) {
	// Endpoint: /storage-systems/{system-id}/consistency-groups/{cg-id}/views/{viewId}
	if _, err := c.Connect(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/consistency-groups/%s/views/%s", cgID, viewID)

	resp, responseBody, err := c.InvokeAPI(ctx, nil, "GET", path)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 404 {
		return nil, nil
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get consistency group view: status %d, body: %s", resp.StatusCode, string(responseBody))
	}

	var view ConsistencyGroupView
	err = json.Unmarshal(responseBody, &view)
	if err != nil {
		return nil, err
	}

	return &view, nil
}

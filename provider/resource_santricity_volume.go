package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	santricity "github.com/scaleoutsean/santricity-go"
)

func resourceVolume() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceVolumeCreate,
		ReadContext:   resourceVolumeRead,
		UpdateContext: resourceVolumeUpdate,
		DeleteContext: resourceVolumeDelete,
		Schema: map[string]*schema.Schema{
			"pool_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The ID (Ref) of the Storage Pool (DDP) to create the volume in.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the volume.",
			},
			"size_gb": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "The size of the volume in GB (will be rounded up to nearest multiple of 4 for DDP compatibility).",
			},
			"raid_level": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "raid6",
				ForceNew:    true,
				Description: "The RAID level for the volume (e.g. raid1, raid6). Defaults to raid6.",
			},
			"block_size": {
				Type:        schema.TypeInt,
				Optional:    true,
				Computed:    true,
				ForceNew:    true,
				Description: "The block size of the volume (e.g. 512, 4096). Defaults to pool recommended size if not specified.",
			},
			"volume_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The ID (Ref) of the created volume.",
			},
			"wwn": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The WWN of the created volume.",
			},
		},
	}
}

func resourceVolumeCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)

	name := d.Get("name").(string)
	poolID := d.Get("pool_id").(string)
	sizeGB := d.Get("size_gb").(int)
	raidLevel := d.Get("raid_level").(string)
	blockSize := d.Get("block_size").(int)

	// Round up size_gb to multiple of 4
	if remainder := sizeGB % 4; remainder != 0 {
		sizeGB += 4 - remainder
	}
	// Update state so Terraform knows about the rounding
	d.Set("size_gb", sizeGB)

	// Convert GB to bytes for the API (library expects bytes or handles conversion? Library CreateVolume expects bytes in uint64)
	// Check library: CreateVolume takes size uint64 (bytes? no, library checks).
	// Library: CreateVolume(..., size uint64, ...). Internal: size/1024 -> converts to KB for request. So input must be bytes.
	// 1 GB = 1024 * 1024 * 1024 bytes.
	sizeBytes := uint64(sizeGB) * 1024 * 1024 * 1024

	// mediaType and fstype are optional/legacy.
	// Defaults for a "dumb" provider:
	// mediaType="hdd" (or whatever valid default, DDP handles this mostly), fstype="xfs" (irrelevant for array).

	// API call
	// Note: protocol is configured in client config implicitly or unnecessary for volume creation itself unless mapping.
	// Library CreateVolume signature:
	// func (d Client) CreateVolume(ctx context.Context, name string, volumeGroupRef string, size uint64, mediaType, fstype string, raidLevel string, blockSize int, segmentSize int, extraTags map[string]string) (VolumeEx, error)

	vol, err := client.CreateVolume(ctx, name, poolID, sizeBytes, "hdd", "xfs", raidLevel, blockSize, 0, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(vol.VolumeRef)
	d.Set("volume_id", vol.VolumeRef)
	d.Set("wwn", vol.WorldWideName)
	d.Set("block_size", vol.BlockSize)

	return resourceVolumeRead(ctx, d, m)
}

func resourceVolumeRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)
	volID := d.Id()

	vol, err := client.GetVolumeByRef(ctx, volID)
	if err != nil {
		// Verify if 404/not found to update state
		if apiErr, ok := err.(santricity.Error); ok && apiErr.Code == 404 {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	d.Set("name", vol.Label)
	d.Set("pool_id", vol.VolumeGroupRef)
	d.Set("block_size", vol.BlockSize)

	// Size conversion
	// VolumeSize in struct is string representing bytes
	capInt, _ := strconv.ParseUint(vol.VolumeSize, 10, 64)
	d.Set("size_gb", int(capInt/(1024*1024*1024)))
	d.Set("wwn", vol.WorldWideName)

	return nil
}

func resourceVolumeUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)
	volID := d.Id()

	if d.HasChange("name") {
		newName := d.Get("name").(string)
		_, err := client.UpdateVolume(ctx, volID, santricity.VolumeUpdateRequest{Name: newName})
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("size_gb") {
		newSizeGB := d.Get("size_gb").(int)

		// Round up size_gb to multiple of 4 (DDP requirement) to avoid drift
		if remainder := newSizeGB % 4; remainder != 0 {
			newSizeGB += 4 - remainder
			d.Set("size_gb", newSizeGB)
		}

		// Use ExpandVolume which takes ref and size directly
		newSizeBytes := int64(newSizeGB) * 1024 * 1024 * 1024

		err := client.ExpandVolume(ctx, volID, newSizeBytes)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	return resourceVolumeRead(ctx, d, m)
}

func resourceVolumeDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)
	volID := d.Id()

	volObj := santricity.VolumeEx{
		VolumeRef: volID,
		Label:     d.Get("name").(string),
	}

	err := client.DeleteVolume(ctx, volObj)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

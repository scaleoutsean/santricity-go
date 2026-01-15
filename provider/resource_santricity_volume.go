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
				ForceNew:    true, // Renaming usually supported but complicated, keeping simpler for now
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
				Description: "The RAID level for the volume (e.g. raid1, raid6). Defaults to raid6.",
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
	// func (d Client) CreateVolume(ctx context.Context, name string, volumeGroupRef string, size uint64, mediaType, fstype string, raidLevel string, blockSize int, segmentSize int) (VolumeEx, error)

	vol, err := client.CreateVolume(ctx, name, poolID, sizeBytes, "hdd", "xfs", raidLevel, 0, 0)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(vol.VolumeRef)
	d.Set("volume_id", vol.VolumeRef)
	d.Set("wwn", vol.WorldWideName)

	return resourceVolumeRead(ctx, d, m)
}

func resourceVolumeRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)
	volID := d.Id()

	// Need a GetVolume method by Ref or ID.
	// client.GetVolume(ctx, volID) ?
	// Library has GetVolumes() which list all.
	// Library has InvokeAPI.
	// Library might have GetVolume(ref). Let's check `client.go`.

	// If specific get not available, we have to list all and find or add method.
	// Assuming GetVolume behavior for now or I'll implement it strictly within provider if needed using InvokeAPI directly to avoid modifying library too much if not desired, but cleaner to fix library.
	// Actually, I'll use list for now if single get is missing, but efficient way is `GetVolume(id)`.
	// For now, I'll assume I can implement `getVolume` helper here or use library.

	// Let's assume we implement a helper `getVolume` in provider or check library.
	// Based on earlier searches, there was `GetVolumes` returning `[]VolumeEx`.
	// I'll stick to listing for safety if specific Get unavailable, but for performance `InvokeAPI` path `/volumes/{id}` is better.
	// I'll implement a local helper if needed.

	// Using the library's internal-ish `InvokeAPI`? No, it's public.
	// But `VolumeEx` struct parsing is needed.

	// Let's try to find the volume in `GetVolumes` list for "v1" MVP.

	vols, err := client.GetVolumes(ctx)
	if err != nil {
		return diag.FromErr(err)
	}

	var foundVol *santricity.VolumeEx
	for _, v := range vols {
		if v.VolumeRef == volID {
			foundVol = &v
			break
		}
	}

	if foundVol == nil {
		d.SetId("")
		return nil
	}

	d.Set("name", foundVol.Label)
	d.Set("pool_id", foundVol.VolumeGroupRef)
	// Size conversion
	// VolumeSize in struct is string? Check types.go.
	// types.go: VolumeSize string `json:"capacity"`
	// It refers to bytes usually as string.
	capInt, _ := strconv.ParseUint(foundVol.VolumeSize, 10, 64)
	d.Set("size_gb", int(capInt/(1024*1024*1024)))
	d.Set("wwn", foundVol.WorldWideName)

	return nil
}

func resourceVolumeUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)
	volID := d.Id()

	if d.HasChange("size_gb") {
		newSizeGB := d.Get("size_gb").(int)
		// We can only expand
		// client.ResizeVolume(ctx, volumeObj, newSizeBytes)
		// We need the volume object first.

		// This requires retrieving the volume struct again.
		// Construct a dummy VolumeEx with Ref is dangerous if method relies on other fields?
		// ResizeVolume signature: func (d Client) ResizeVolume(ctx context.Context, volume VolumeEx, size uint64) error
		// It uses volume.VolumeRef and volume.Label (logging).

		volObj := santricity.VolumeEx{
			VolumeRef: volID,
			Label:     d.Get("name").(string),
		}

		newSizeBytes := uint64(newSizeGB) * 1024 * 1024 * 1024

		err := client.ResizeVolume(ctx, volObj, newSizeBytes)
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

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	santricity "github.com/scaleoutsean/santricity-go"
)

func resourceSnapshotVolume() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceSnapshotVolumeCreate,
		ReadContext:   resourceSnapshotVolumeRead,
		DeleteContext: resourceSnapshotVolumeDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"snapshot_image_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The ID (Ref) of the snapshot image to create the volume from.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of the new snapshot volume.",
			},
			"view_mode": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "readOnly",
				ForceNew:    true,
				Description: "The view mode of the snapshot volume (readOnly or readWrite).",
			},
			"repository_percentage": {
				Type:        schema.TypeFloat,
				Optional:    true,
				Default:     20.0,
				ForceNew:    true,
				Description: "The size of the repository volume as a percentage of the base volume size.",
			},
			"repository_pool_id": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Optional: The ID (Ref) of the storage pool to create the repository in.",
			},
			"full_threshold": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     80,
				ForceNew:    true,
				Description: "The repository utilization warning threshold percentage.",
			},
			"base_volume_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The ID (Ref) of the base volume.",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The status of the snapshot volume.",
			},
		},
	}
}

func resourceSnapshotVolumeCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)

	req := santricity.SnapshotVolumeCreateRequest{
		SnapshotImageId:      d.Get("snapshot_image_id").(string),
		Name:                 d.Get("name").(string),
		ViewMode:             d.Get("view_mode").(string),
		RepositoryPercentage: d.Get("repository_percentage").(float64),
		FullThreshold:        d.Get("full_threshold").(int),
	}

	if v, ok := d.GetOk("repository_pool_id"); ok {
		req.RepositoryPoolId = v.(string)
	}

	volume, err := client.CreateSnapshotVolume(ctx, req)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error creating snapshot volume: %s", err))
	}

	d.SetId(volume.SnapshotRef)

	return resourceSnapshotVolumeRead(ctx, d, m)
}

func resourceSnapshotVolumeRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)

	id := d.Id()

	volume, err := client.GetSnapshotVolume(ctx, id)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error reading snapshot volume %s: %s", id, err))
	}

	if volume == nil {
		d.SetId("")
		return nil
	}

	d.Set("snapshot_image_id", volume.BasePIT)
	d.Set("name", volume.Label)
	d.Set("base_volume_id", volume.BaseVolume)
	d.Set("status", volume.Status)
	// view_mode, repository_percentage, full_threshold are not typically returned in the basic View object
	// or might require extra API calls, but for now we set the state from what we found.
	// Since forceNew=true, drift is less critical for immutable parameters.

	return nil
}

func resourceSnapshotVolumeDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)
	id := d.Id()

	err := client.DeleteSnapshotVolume(ctx, id)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error deleting snapshot volume %s: %s", id, err))
	}

	d.SetId("")
	return nil
}

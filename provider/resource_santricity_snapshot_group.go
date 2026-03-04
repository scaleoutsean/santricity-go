package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	santricity "github.com/scaleoutsean/santricity-go"
)

func resourceSnapshotGroup() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceSnapshotGroupCreate,
		ReadContext:   resourceSnapshotGroupRead,
		DeleteContext: resourceSnapshotGroupDelete,
		Schema: map[string]*schema.Schema{
			"base_volume_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The ID (Ref) of the base volume to create the snapshot group for.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of the snapshot group.",
			},
			"repository_percentage": {
				Type:        schema.TypeFloat,
				Optional:    true,
				Default:     20.0,
				ForceNew:    true,
				Description: "The size of the repository volume as a percentage of the base volume size.",
			},
			"warning_threshold": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     80,
				ForceNew:    true,
				Description: "The repository utilization warning threshold percentage.",
			},
			"auto_delete_limit": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     30,
				ForceNew:    true,
				Description: "The automatic deletion limit for snapshot images.",
			},
			"full_policy": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "purgepit",
				ForceNew:    true,
				Description: "The repository full policy (purgepit or failbasewrites).",
			},
			"storage_pool_id": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Optional: The ID (Ref) of the storage pool to create the repository in.",
			},
			"snapshot_group_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The ID (Ref) of the created snapshot group.",
			},
		},
	}
}

func resourceSnapshotGroupCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)

	req := santricity.SnapshotGroupCreateRequest{
		BaseMappableObjectId: d.Get("base_volume_id").(string),
		Name:                 d.Get("name").(string),
		RepositoryPercentage: d.Get("repository_percentage").(float64),
		WarningThreshold:     d.Get("warning_threshold").(int),
		AutoDeleteLimit:      d.Get("auto_delete_limit").(int),
		FullPolicy:           d.Get("full_policy").(string),
	}

	if v, ok := d.GetOk("storage_pool_id"); ok {
		req.StoragePoolId = v.(string)
	}

	group, err := client.CreateSnapshotGroup(ctx, req)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(group.PitGroupRef)
	d.Set("snapshot_group_id", group.PitGroupRef)

	return resourceSnapshotGroupRead(ctx, d, m)
}

func resourceSnapshotGroupRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)

	id := d.Id()
	group, err := client.GetSnapshotGroup(ctx, id)
	if err != nil {
		return diag.FromErr(err)
	}
	if group == nil {
		d.SetId("")
		return nil
	}

	d.Set("base_volume_id", group.BaseVolume)
	d.Set("name", group.Label)
	d.Set("snapshot_group_id", group.PitGroupRef)
	d.Set("warning_threshold", group.FullWarnThreshold)
	d.Set("auto_delete_limit", group.AutoDeleteLimit)
	d.Set("full_policy", group.RepFullPolicy)

	return nil
}

func resourceSnapshotGroupDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)
	id := d.Id()

	err := client.DeleteSnapshotGroup(ctx, id)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

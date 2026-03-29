package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	santricity "github.com/scaleoutsean/santricity-go"
)

func resourceConsistencyGroupMember() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceConsistencyGroupMemberCreate,
		ReadContext:   resourceConsistencyGroupMemberRead,
		DeleteContext: resourceConsistencyGroupMemberDelete,
		Schema: map[string]*schema.Schema{
			"consistency_group_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The ID (Ref) of the consistency group.",
			},
			"volume_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The ID (Ref) of the volume to add as a member.",
			},
			"repository_percent": {
				Type:        schema.TypeFloat,
				Optional:    true,
				Default:     20.0,
				ForceNew:    true,
				Description: "The repository utilization percentage.",
			},
			"repository_pool_id": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Optional: The ID (Ref) of the expected storage pool.",
			},
			"scan_media": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				ForceNew:    true,
				Description: "Scan media for errors.",
			},
			"validate_parity": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				ForceNew:    true,
				Description: "Validate parity.",
			},
		},
	}
}

func resourceConsistencyGroupMemberCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)

	cgID := d.Get("consistency_group_id").(string)
	volID := d.Get("volume_id").(string)

	req := santricity.ConsistencyGroupMemberAddRequest{
		VolumeId:          volID,
		RepositoryPercent: d.Get("repository_percent").(float64),
		ScanMedia:         d.Get("scan_media").(bool),
		ValidateParity:    d.Get("validate_parity").(bool),
	}

	if v, ok := d.GetOk("repository_pool_id"); ok {
		req.RepositoryPoolId = v.(string)
	}

	_, err := client.AddConsistencyGroupMember(ctx, cgID, req)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%s:%s", cgID, volID))

	return resourceConsistencyGroupMemberRead(ctx, d, m)
}

func resourceConsistencyGroupMemberRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)

	id := d.Id()
	parts := strings.Split(id, ":")
	if len(parts) != 2 {
		return diag.Errorf("Invalid consistency group member ID: %s", id)
	}
	cgID := parts[0]
	volID := parts[1]

	member, err := client.GetConsistencyGroupMember(ctx, cgID, volID)
	if err != nil {
		return diag.FromErr(err)
	}
	if member == nil {
		d.SetId("")
		return nil
	}

	d.Set("consistency_group_id", member.ConsistencyGroupId)
	d.Set("volume_id", member.VolumeId)

	return nil
}

func resourceConsistencyGroupMemberDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)

	id := d.Id()
	parts := strings.Split(id, ":")
	if len(parts) != 2 {
		return diag.Errorf("Invalid consistency group member ID: %s", id)
	}
	cgID := parts[0]
	volID := parts[1]

	err := client.RemoveConsistencyGroupMember(ctx, cgID, volID)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	santricity "github.com/scaleoutsean/santricity-go"
)

func resourceConsistencyGroup() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceConsistencyGroupCreate,
		ReadContext:   resourceConsistencyGroupRead,
		DeleteContext: resourceConsistencyGroupDelete,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of the consistency group.",
			},
			"full_warn_threshold_percent": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     80,
				ForceNew:    true,
				Description: "The repository utilization warning threshold percentage.",
			},
			"auto_delete_threshold": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     30,
				ForceNew:    true,
				Description: "The automatic deletion limit for snapshot images.",
			},
			"repository_full_policy": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "purgepit",
				ForceNew:    true,
				Description: "The repository full policy (e.g., purgepit or failbasewrites).",
			},
			"rollback_priority": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "medium",
				ForceNew:    true,
				Description: "The rollback priority (e.g., lowest, low, medium, high, highest).",
			},
			"consistency_group_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The ID (Ref) of the created consistency group.",
			},
		},
	}
}

func resourceConsistencyGroupCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)

	req := santricity.ConsistencyGroupCreateRequest{
		Name:                     d.Get("name").(string),
		FullWarnThresholdPercent: d.Get("full_warn_threshold_percent").(int),
		AutoDeleteThreshold:      d.Get("auto_delete_threshold").(int),
		RepositoryFullPolicy:     d.Get("repository_full_policy").(string),
		RollbackPriority:         d.Get("rollback_priority").(string),
	}

	group, err := client.CreateConsistencyGroup(ctx, req)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(group.ConsistencyGroupRef)
	d.Set("consistency_group_id", group.ConsistencyGroupRef)

	return resourceConsistencyGroupRead(ctx, d, m)
}

func resourceConsistencyGroupRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)
	id := d.Id()

	group, err := client.GetConsistencyGroup(ctx, id)
	if err != nil {
		return diag.FromErr(err)
	}
	if group == nil {
		d.SetId("")
		return nil
	}

	d.Set("name", group.Label)
	d.Set("consistency_group_id", group.ConsistencyGroupRef)
	d.Set("full_warn_threshold_percent", group.FullWarnThreshold)
	d.Set("auto_delete_threshold", group.AutoDeleteLimit)
	d.Set("repository_full_policy", group.RepFullPolicy)
	d.Set("rollback_priority", group.RollbackPriority)

	return nil
}

func resourceConsistencyGroupDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)
	id := d.Id()

	err := client.DeleteConsistencyGroup(ctx, id)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

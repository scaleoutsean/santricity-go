package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	santricity "github.com/scaleoutsean/santricity-go"
)

func resourceConsistencyGroupView() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceConsistencyGroupViewCreate,
		ReadContext:   resourceConsistencyGroupViewRead,
		DeleteContext: resourceConsistencyGroupViewDelete,
		Schema: map[string]*schema.Schema{
			"consistency_group_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The ID (Ref) of the consistency group.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of the view.",
			},
			"snapshot_id": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "The snapshot image ID or sequence number (optional).",
			},
			"sequence_number": {
				Type:        schema.TypeInt,
				Optional:    true,
				ForceNew:    true,
				Description: "The sequence number (optional).",
			},
			"access_mode": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "readOnly",
				ForceNew:    true,
				Description: "Access mode for the view (readOnly, readWrite).",
			},
			"repository_percent": {
				Type:        schema.TypeFloat,
				Optional:    true,
				Default:     20.0,
				ForceNew:    true,
				Description: "Repository percentage.",
			},
			"repository_pool_id": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Optional: Storage pool ID for the repository.",
			},
			"view_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The ID (Ref) of the created view.",
			},
		},
	}
}

func resourceConsistencyGroupViewCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)

	cgID := d.Get("consistency_group_id").(string)

	req := santricity.ConsistencyGroupViewCreateRequest{
		Name:              d.Get("name").(string),
		AccessMode:        d.Get("access_mode").(string),
		RepositoryPercent: d.Get("repository_percent").(float64),
	}

	if v, ok := d.GetOk("snapshot_id"); ok {
		req.PitId = v.(string)
	}

	// Or seq number mapping depending on the precise field they pass
	if v, ok := d.GetOk("sequence_number"); ok {
		req.PitSequenceNumber = int64(v.(int))
	} else if req.PitId == "" {
		// Provide logic to extract if possible? Usually one or the other is fine.
	}

	if v, ok := d.GetOk("repository_pool_id"); ok {
		req.RepositoryPoolId = v.(string)
	}

	view, err := client.CreateConsistencyGroupView(ctx, cgID, req)
	if err != nil {
		return diag.FromErr(err)
	}

	// Format id as cgID:viewID
	d.SetId(fmt.Sprintf("%s:%s", cgID, view.ConsistencyGroupViewRef))
	d.Set("view_id", view.ConsistencyGroupViewRef)

	return resourceConsistencyGroupViewRead(ctx, d, m)
}

func resourceConsistencyGroupViewRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)

	id := d.Id()
	parts := strings.Split(id, ":")
	if len(parts) != 2 {
		return diag.Errorf("Invalid consistency group view ID format: %s", id)
	}
	cgID := parts[0]
	viewID := parts[1]

	view, err := client.GetConsistencyGroupView(ctx, cgID, viewID)
	if err != nil {
		return diag.FromErr(err)
	}
	if view == nil {
		d.SetId("")
		return nil
	}

	d.Set("consistency_group_id", view.GroupRef) // Adjust field if needed
	d.Set("name", view.Name)
	d.Set("view_id", view.ConsistencyGroupViewRef)

	// Since seq number can be int or string, if API returns a string:
	if seq, err := strconv.Atoi(view.ViewSequenceNumber); err == nil {
		d.Set("sequence_number", seq)
	}

	return nil
}

func resourceConsistencyGroupViewDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)
	id := d.Id()

	parts := strings.Split(id, ":")
	if len(parts) != 2 {
		return diag.Errorf("Invalid consistency group view ID format: %s", id)
	}
	cgID := parts[0]
	viewID := parts[1]

	err := client.DeleteConsistencyGroupView(ctx, cgID, viewID)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

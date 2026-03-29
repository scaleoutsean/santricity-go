package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	santricity "github.com/scaleoutsean/santricity-go"
)

func resourceConsistencyGroupSnapshot() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceConsistencyGroupSnapshotCreate,
		ReadContext:   resourceConsistencyGroupSnapshotRead,
		DeleteContext: resourceConsistencyGroupSnapshotDelete,
		Schema: map[string]*schema.Schema{
			"consistency_group_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The ID (Ref) of the consistency group to snapshot.",
			},
			"sequence_number": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The sequence number of the created consistency group snapshot.",
			},
		},
	}
}

func resourceConsistencyGroupSnapshotCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)

	cgID := d.Get("consistency_group_id").(string)

	images, err := client.CreateConsistencyGroupSnapshot(ctx, cgID)
	if err != nil {
		return diag.FromErr(err)
	}

	if len(images) == 0 {
		return diag.Errorf("Failed to create consistency group snapshot: no images returned")
	}

	seqNumber := images[0].PitSequenceNumber

	d.SetId(fmt.Sprintf("%s:%s", cgID, seqNumber))
	d.Set("sequence_number", seqNumber)

	return resourceConsistencyGroupSnapshotRead(ctx, d, m)
}

func resourceConsistencyGroupSnapshotRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)

	id := d.Id()
	parts := strings.Split(id, ":")
	if len(parts) != 2 {
		return diag.Errorf("Invalid consistency group snapshot ID format: %s", id)
	}
	cgID := parts[0]
	seqNumber := parts[1]

	images, err := client.GetConsistencyGroupSnapshot(ctx, cgID, seqNumber)
	if err != nil {
		return diag.FromErr(err)
	}
	if images == nil || len(images) == 0 {
		d.SetId("")
		return nil
	}

	d.Set("consistency_group_id", cgID)
	d.Set("sequence_number", seqNumber)

	return nil
}

func resourceConsistencyGroupSnapshotDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)
	id := d.Id()

	parts := strings.Split(id, ":")
	if len(parts) != 2 {
		return diag.Errorf("Invalid consistency group snapshot ID format: %s", id)
	}
	cgID := parts[0]
	seqNumber := parts[1]

	err := client.DeleteConsistencyGroupSnapshot(ctx, cgID, seqNumber)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

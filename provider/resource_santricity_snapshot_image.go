package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	santricity "github.com/scaleoutsean/santricity-go"
)

func resourceSnapshotImage() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceSnapshotImageCreate,
		ReadContext:   resourceSnapshotImageRead,
		DeleteContext: resourceSnapshotImageDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"group_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"status": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"timestamp": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"sequence_number": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"base_vol": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceSnapshotImageCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)

	groupID := d.Get("group_id").(string)

	req := santricity.SnapshotImageCreateRequest{
		GroupId: groupID,
	}

	image, err := client.CreateSnapshotImage(ctx, req)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error creating snapshot image: %s", err))
	}

	d.SetId(image.PitRef)

	return resourceSnapshotImageRead(ctx, d, m)
}

func resourceSnapshotImageRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)

	id := d.Id()

	image, err := client.GetSnapshotImage(ctx, id)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error reading snapshot image %s: %s", id, err))
	}

	if image == nil {
		d.SetId("")
		return nil
	}

	d.Set("group_id", image.PitGroupRef)
	d.Set("status", image.Status)
	d.Set("timestamp", image.PitTimestamp)
	d.Set("sequence_number", image.PitSequenceNumber)
	d.Set("base_vol", image.BaseVol)

	return nil
}

func resourceSnapshotImageDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)
	id := d.Id()

	err := client.DeleteSnapshotImage(ctx, id)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error deleting snapshot image %s: %s", id, err))
	}

	d.SetId("")
	return nil
}

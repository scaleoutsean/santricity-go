package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	santricity "github.com/scaleoutsean/santricity-go"
)

func resourceMapping() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceMappingCreate,
		ReadContext:   resourceMappingRead,
		DeleteContext: resourceMappingDelete,
		Schema: map[string]*schema.Schema{
			"volume_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The ID (Ref) of the volume to map.",
			},
			"host_id": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				Description:   "The ID (Ref) of the Host to map to.",
				ConflictsWith: []string{"host_group_id"},
			},
			"host_group_id": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				Description:   "The ID (Ref) of the Host Group to map to.",
				ConflictsWith: []string{"host_id"},
			},
			"lun": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "The LUN number to assign.",
			},
			"mapping_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The ID (Ref) of the mapping.",
			},
		},
	}
}

func resourceMappingCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)

	volID := d.Get("volume_id").(string)
	lun := d.Get("lun").(int)

	var hostObj santricity.HostEx
	var err error

	if v, ok := d.GetOk("host_id"); ok {
		hostID := v.(string)
		// Fetch real host object to get ClusterRef if it exists, ensuring we map to cluster if needed
		hostObj, err = client.GetHostByRef(ctx, hostID)
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to fetch host info for mapping (ID: %s): %v", hostID, err))
		}
	} else if v, ok := d.GetOk("host_group_id"); ok {
		hgID := v.(string)
		// If mapping to a group directly, we spoof a Host object with ClusterRef set.
		// maintainer: this relies on client.MapVolume preferring ClusterRef over HostRef.
		hostObj = santricity.HostEx{
			HostRef:    "virtual-host-for-group-mapping",
			ClusterRef: hgID,
			Label:      "tf-group-map-" + hgID,
		}
	} else {
		return diag.Errorf("One of host_id or host_group_id must be specified.")
	}

	volObj := santricity.VolumeEx{
		VolumeRef: volID,
		Label:     "tf-volume-ref-" + volID,
	}

	result, err := client.MapVolume(ctx, volObj, hostObj, lun)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(result.LunMappingRef)
	d.Set("mapping_id", result.LunMappingRef)

	return resourceMappingRead(ctx, d, m)
}

func resourceMappingRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	// TODO: implement read verifying mapping exists.
	// For now assume existence.
	return nil
}

func resourceMappingDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)
	volID := d.Get("volume_id").(string)

	volObj := santricity.VolumeEx{
		VolumeRef: volID,
		Label:     "tf-volume-ref-" + volID,
	}

	// UnmapVolume(ctx, volume)
	err := client.UnmapVolume(ctx, volObj)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

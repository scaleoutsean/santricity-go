package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	santricity "github.com/scaleoutsean/santricity-go"
)

func resourceHost() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceHostCreate,
		ReadContext:   resourceHostRead,
		UpdateContext: resourceHostUpdate,
		DeleteContext: resourceHostDelete,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the Host.",
			},
			"type": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "linux_dm_mp",
				Description: "The Host Type (e.g. linux_dm_mp, vmware, windows).",
			},
			"ports": {
				Type:     schema.TypeList,
				Required: true,
				ForceNew: true, // Changing ports (identity) requires replacement
				MinItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"type": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Port type (iscsi, fc, nvme, nvmeof, ib)",
						},
						"port": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Port identifier (IQN, NQN or WWN)",
						},
						"label": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "User label for port",
						},
						"chap_secret": {
							Type:        schema.TypeString,
							Optional:    true,
							Sensitive:   true,
							Description: "CHAP secret for the port (iSCSI only).",
						},
					},
				},
				Description: "List of host ports.",
			},
			"host_group_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The ID (Ref) of the Host Group.",
			},
			"host_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The ID (Ref) of the created Host.",
			},
		},
	}
}

func resourceHostCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)

	name := d.Get("name").(string)
	hostType := d.Get("type").(string)
	portsList := d.Get("ports").([]interface{})

	// Library limitation? CreateHost takes 1 IQN.
	// For MVP: We take the first port from the list.
	// Future: Iterate and add ports. (Library addPort method needed).

	if len(portsList) == 0 {
		return diag.Errorf("At least one port must be specified")
	}

	firstPort := portsList[0].(map[string]interface{})
	portType := firstPort["type"].(string)
	portID := firstPort["port"].(string)
	authSecret := ""
	if v, ok := firstPort["chap_secret"]; ok {
		authSecret = v.(string)
	}

	if portType != "iscsi" && portType != "fc" && portType != "nvme" && portType != "nvmeof" && portType != "ib" {
		return diag.Errorf("Supported port types: 'iscsi', 'fc', 'nvme', 'nvmeof', 'ib'")
	}

	// CreateHost(ctx, name, portID, portType, hostType, authSecret, hostGroup)
	hg := santricity.HostGroup{}
	if v, ok := d.GetOk("host_group_id"); ok {
		hg.ClusterRef = v.(string)
		hg.Label = "tf-group-ref-" + v.(string)
	}

	host, err := client.CreateHost(ctx, name, portID, portType, hostType, authSecret, hg)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(host.HostRef)
	d.Set("host_id", host.HostRef)

	// If more ports? Warn user.
	if len(portsList) > 1 {
		return diag.Errorf("Only single port currently supported by this provider implementation")
	}

	return resourceHostRead(ctx, d, m)
}

func resourceHostRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)
	hostID := d.Id()

	host, err := client.GetHostByRef(ctx, hostID)
	if err != nil {
		// Verify if 404/not found. The library returns an error if not 200/OK.
		if apiErr, ok := err.(santricity.Error); ok && apiErr.Code == 404 {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	d.Set("name", host.Label)
	// We could import port details here but that might cause diffs if they are reordered.
	// For now trust Terraform state on ports unless drifted.
	// However, we should verify at least name.

	return nil
}

func resourceHostUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)
	hostID := d.Id()

	updateReq := santricity.HostUpdateRequest{}
	updateNeeded := false

	if d.HasChange("name") {
		updateReq.Name = d.Get("name").(string)
		updateNeeded = true
	}

	if d.HasChange("host_group_id") {
		updateReq.GroupID = d.Get("host_group_id").(string)
		updateNeeded = true
	}

	if d.HasChange("type") {
		hostTypeStr := d.Get("type").(string)
		idx := client.GetBestIndexForHostType(ctx, hostTypeStr)
		updateReq.HostType = &santricity.HostType{Index: idx}
		updateNeeded = true
	}

	if updateNeeded {
		_, err := client.UpdateHost(ctx, hostID, updateReq)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	return resourceHostRead(ctx, d, m)
}

func resourceHostDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)
	hostID := d.Id()

	err := client.DeleteHost(ctx, hostID)
	if err != nil {
		if apiErr, ok := err.(santricity.Error); ok && apiErr.Code == 404 {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

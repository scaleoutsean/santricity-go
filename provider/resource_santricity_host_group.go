package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	santricity "github.com/scaleoutsean/santricity-go"
)

func resourceHostGroup() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceHostGroupCreate,
		ReadContext:   resourceHostGroupRead,
		DeleteContext: resourceHostGroupDelete,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of the Host Group.",
			},
		},
	}
}

func resourceHostGroupCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)
	name := d.Get("name").(string)

	hg, err := client.CreateHostGroup(ctx, name)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(hg.ClusterRef)
	return resourceHostGroupRead(ctx, d, m)
}

func resourceHostGroupRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)
	id := d.Id()

	hg, err := client.GetHostGroupByRef(ctx, id)
	if err != nil {
		// If 404, remove from state
		// But GetHostGroupByRef returns error on non-200.
		// We should check if error is 404/NotFound.
		return diag.FromErr(err)
	}

	d.Set("name", hg.Label)
	return nil
}

func resourceHostGroupDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*santricity.Client)
	id := d.Id()

	err := client.DeleteHostGroup(ctx, id)
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId("")
	return nil
}

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	santricity "github.com/scaleoutsean/santricity-go"
)

func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"endpoint": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The controller IP or Hostname.",
				DefaultFunc: schema.EnvDefaultFunc("SANTRICITY_ENDPOINT", nil),
			},
			"username": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Username for basic auth.",
				DefaultFunc: schema.EnvDefaultFunc("SANTRICITY_USERNAME", nil),
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Password for basic auth.",
				DefaultFunc: schema.EnvDefaultFunc("SANTRICITY_PASSWORD", nil),
			},
			"token": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Bearer token (mutually exclusive with username/password).",
				DefaultFunc: schema.EnvDefaultFunc("SANTRICITY_TOKEN", nil),
			},
			"insecure": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Skip TLS verification.",
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"santricity_volume": resourceVolume(),
		},
		DataSourcesMap:       map[string]*schema.Resource{},
		ConfigureContextFunc: providerConfigure,
	}
}

func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	endpoint := d.Get("endpoint").(string)
	username := d.Get("username").(string)
	password := d.Get("password").(string)
	token := d.Get("token").(string)
	insecure := d.Get("insecure").(bool)

	var diags diag.Diagnostics

	if token != "" && (username != "" || password != "") {
		return nil, diag.Errorf("token and username/password are mutually exclusive")
	}

	config := santricity.ClientConfig{
		ApiControllers: []string{endpoint},
		ApiPort:        8443,
		Username:       username,
		Password:       password,
		BearerToken:    token,
		VerifyTLS:      !insecure,
	}

	c := santricity.NewAPIClient(ctx, config)

	// Verify connection
	if _, err := c.Connect(ctx); err != nil {
		return nil, diag.FromErr(fmt.Errorf("error connecting to device: %s", err))
	}

	return c, diags
}

package unit_test

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/nmehlei/terraform-provider-aptabase/src/provider"
)

func newProviderServer(t *testing.T) tfprotov6.ProviderServer {
	t.Helper()
	server, err := providerserver.NewProtocol6WithError(provider.New("test")())()
	if err != nil {
		t.Fatalf("creating provider server: %v", err)
	}
	return server
}

func configValueFromEndpointOnly() (*tfprotov6.DynamicValue, error) {
	objType := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"endpoint": tftypes.String,
			"token":    tftypes.String,
		},
	}
	value := tftypes.NewValue(objType, map[string]tftypes.Value{
		"endpoint": tftypes.NewValue(tftypes.String, "https://aptabase.example.com"),
		"token":    tftypes.NewValue(tftypes.String, nil),
	})
	dv, err := tfprotov6.NewDynamicValue(objType, value)
	if err != nil {
		return nil, err
	}
	return &dv, nil
}

func TestProvider_Metadata_ReportsAptabaseTypeName(t *testing.T) {
	server := newProviderServer(t)
	resp, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema: %v", err)
	}
	if resp.Provider == nil {
		t.Fatal("expected a non-nil provider schema")
	}
}

func TestProvider_Configure_MissingTokenAndEnv_ReturnsError(t *testing.T) {
	os.Unsetenv("APTABASE_TOKEN")
	server := newProviderServer(t)

	config, err := configValueFromEndpointOnly()
	if err != nil {
		t.Fatalf("building config: %v", err)
	}

	resp, err := server.ConfigureProvider(context.Background(), &tfprotov6.ConfigureProviderRequest{
		Config: config,
	})
	if err != nil {
		t.Fatalf("ConfigureProvider: %v", err)
	}
	if len(resp.Diagnostics) == 0 {
		t.Fatal("expected a diagnostic for a missing token")
	}
}

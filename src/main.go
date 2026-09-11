package main

//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name aptabase --examples-dir ../examples --rendered-website-dir ../docs --website-source-dir ../templates

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/nmehlei/terraform-provider-aptabase/src/provider"
)

// version is injected at build time via -ldflags (see .goreleaser.yml).
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/nmehlei/aptabase",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}

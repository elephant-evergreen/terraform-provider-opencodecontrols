package main

import (
	"context"
	"log"

	"github.com/elephant-evergreen/terraform-provider-opencodecontrols/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var version = "dev"

func main() {
	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/elephant-evergreen/opencodecontrols",
	})
	if err != nil {
		log.Fatal(err)
	}
}

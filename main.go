package main

//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate -provider-name vengtoo

import (
	"context"
	"log"

	"github.com/vengtoo/terraform-provider-vengtoo/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

func main() {
	err := providerserver.Serve(context.Background(), provider.New, providerserver.ServeOpts{
		Address: "registry.terraform.io/vengtoo/vengtoo",
	})
	if err != nil {
		log.Fatal(err)
	}
}

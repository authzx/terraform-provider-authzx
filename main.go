package main

//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate -provider-name authzx

import (
	"context"
	"log"

	"github.com/authzx/terraform-provider-authzx/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

func main() {
	err := providerserver.Serve(context.Background(), provider.New, providerserver.ServeOpts{
		Address: "registry.terraform.io/authzx/authzx",
	})
	if err != nil {
		log.Fatal(err)
	}
}

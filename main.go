package main

import (
	"flag"

	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"

	"github.com/himanhsugusain/terraform-provider-spinnaker/spinnaker"
)

func main() {
	var debugMode bool

	flag.BoolVar(&debugMode, "debuggable", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: spinnaker.Provider,
		Debug:        debugMode,
	})
}

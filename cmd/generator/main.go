/*
Copyright 2023 SAP SE
*/

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/crossplane/upjet/pkg/pipeline"

	"github.tools.sap/cloud-orchestration/crossplane-provider-cloudfoundry/config"
)

func main() {

	fmt.Println(os.Args)
	if len(os.Args) < 2 || os.Args[1] == "" {
		panic("root directory is required to be given as argument")
	}

	rootDir := os.Args[1]

	fmt.Println(rootDir)

	absRootDir, err := filepath.Abs(rootDir)

	fmt.Println(absRootDir)
	if err != nil {
		panic(fmt.Sprintf("cannot calculate the absolute path with %s", rootDir))
	}

	// need to overide the rootgroup as we as want to control the name of the CRD groups

	// todo(mirza): should this be move inside the method GetProvider?
	provider := config.GetProvider()
	//	provider.RootGroup = "cloudfoundry.btp.orchestrate.cloud.sap"
	rg := provider.RootGroup
	fmt.Println(rg)

	pipeline.Run(provider, absRootDir)
}

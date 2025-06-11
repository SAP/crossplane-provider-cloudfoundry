package main

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/importer/cmd"
)

func main() {
	// Set up a new logger
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	cmd.Execute()
}

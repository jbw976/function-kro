package main

import (
	"context"

	"github.com/upbound/function-kro/input/v1beta1"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/response"
)

// Function returns whatever response you ask it to.
type Function struct {
	fnv1.UnimplementedFunctionRunnerServiceServer

	log logging.Logger
}

// RunFunction runs the Function.
func (f *Function) RunFunction(_ context.Context, req *fnv1.RunFunctionRequest) (*fnv1.RunFunctionResponse, error) {
	f.log.Info("Running function", "tag", req.GetMeta().GetTag())

	rsp := response.To(req, response.DefaultTTL)

	rg := &v1beta1.ResourceGraph{}
	if err := request.GetInput(req, rg); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get Function input from %T", req))
		return rsp, nil
	}

	// TODO(negz): This takes a RESTClient to make a discovery client. It uses
	// the discovery client to load the OAPI schema for all resource kinds that
	// appear in the template. It also generates a dummy schema.
	//
	// Process:
	// 1. Create map of all namespaced GroupKinds in the cluster
	// 2. Create map of resource (template) by id
	//    a. Use schema resolver to get a CEL schema - https://pkg.go.dev/k8s.io/apiserver/pkg/cel/openapi/resolver#DefinitionsSchemaResolver.ResolveSchema
	// gb, err := graph.NewBuilder(nil)
	// if err != nil {
	// 	//
	// }
	// g, err := gb.NewResourceGraphDefinition(nil) // TODO(negz): Takes RGD as input.
	// if err != nil {
	// 	//
	// }

	return rsp, nil
}

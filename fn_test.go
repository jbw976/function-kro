package main

import (
	"context"
	"errors"
	"testing"

	"github.com/go-openapi/jsonreference"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"k8s.io/utils/ptr"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/response"
)

func TestRunFunction(t *testing.T) {
	type args struct {
		ctx context.Context
		req *fnv1.RunFunctionRequest
	}
	type want struct {
		rsp *fnv1.RunFunctionResponse
		err error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"MissingCRDs": {
			reason: "The function should return requirements when CRDs are not yet available",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "test"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "kro.fn.crossplane.io/v1beta1",
						"kind": "ResourceGraph",
						"resources": [{
							"id": "bucket",
							"template": {
								"apiVersion": "s3.aws.upbound.io/v1beta1",
								"kind": "Bucket",
								"metadata": {},
								"spec": {
									"forProvider": {
										"region": "us-west-2"
									}
								}
							}
						}],
						"status": {
							"bucketName": "${bucket.status.atProvider.id}"
						}
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.crossplane.io/v1",
								"kind": "XBucket",
								"metadata": {"name": "test-bucket"},
								"spec": {}
							}`),
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "test", Ttl: durationpb.New(response.DefaultTTL)},
					Requirements: &fnv1.Requirements{
						Schemas: map[string]*fnv1.SchemaSelector{
							"example.crossplane.io/v1, Kind=XBucket": {
								ApiVersion: "example.crossplane.io/v1",
								Kind:       "XBucket",
							},
							"s3.aws.upbound.io/v1beta1, Kind=Bucket": {
								ApiVersion: "s3.aws.upbound.io/v1beta1",
								Kind:       "Bucket",
							},
						},
						Resources: map[string]*fnv1.ResourceSelector{
							"example.crossplane.io/v1, Kind=XBucket": {
								ApiVersion: "apiextensions.k8s.io/v1",
								Kind:       "CustomResourceDefinition",
								Match:      &fnv1.ResourceSelector_MatchName{MatchName: "xbuckets.example.crossplane.io"},
							},
							"s3.aws.upbound.io/v1beta1, Kind=Bucket": {
								ApiVersion: "apiextensions.k8s.io/v1",
								Kind:       "CustomResourceDefinition",
								Match:      &fnv1.ResourceSelector_MatchName{MatchName: "buckets.s3.aws.upbound.io"},
							},
						},
					},
				},
			},
		},
		"DesiredXROnlyContainsDeclaredStatus": {
			reason: "The desired XR should only contain status fields declared in the ResourceGraph, not the full observed XR",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "test"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "kro.fn.crossplane.io/v1beta1",
						"kind": "ResourceGraph",
						"resources": [{
							"id": "bucket",
							"template": {
								"apiVersion": "s3.aws.upbound.io/v1beta1",
								"kind": "Bucket",
								"metadata": {},
								"spec": {
									"forProvider": {
										"region": "us-west-2"
									}
								}
							}
						}],
						"status": {
							"bucketName": "${bucket.status.atProvider.id}"
						}
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.crossplane.io/v1",
								"kind": "XBucket",
								"metadata": {
									"name": "test-bucket",
									"uid": "abc-123",
									"resourceVersion": "12345",
									"generation": 2
								},
								"spec": {"bucketName": "my-bucket"},
								"status": {
									"conditions": [{"type": "Ready", "status": "True"}]
								}
							}`),
						},
					},
					RequiredResources: map[string]*fnv1.Resources{
						"example.crossplane.io/v1, Kind=XBucket": {
							Items: []*fnv1.Resource{{
								Resource: resource.MustStructJSON(`{
									"apiVersion": "apiextensions.k8s.io/v1",
									"kind": "CustomResourceDefinition",
									"metadata": {"name": "xbuckets.example.crossplane.io"},
									"spec": {
										"group": "example.crossplane.io",
										"names": {"kind": "XBucket", "plural": "xbuckets"},
										"scope": "Cluster",
										"versions": [{
											"name": "v1",
											"served": true,
											"storage": true,
											"schema": {
												"openAPIV3Schema": {
													"type": "object",
													"properties": {
														"apiVersion": {"type": "string"},
														"kind": {"type": "string"},
														"metadata": {"type": "object"},
														"spec": {
															"type": "object",
															"properties": {
																"bucketName": {"type": "string"}
															}
														},
														"status": {
															"type": "object",
															"properties": {
																"bucketName": {"type": "string"}
															}
														}
													}
												}
											}
										}]
									}
								}`),
							}},
						},
						"s3.aws.upbound.io/v1beta1, Kind=Bucket": {
							Items: []*fnv1.Resource{{
								Resource: resource.MustStructJSON(`{
									"apiVersion": "apiextensions.k8s.io/v1",
									"kind": "CustomResourceDefinition",
									"metadata": {"name": "buckets.s3.aws.upbound.io"},
									"spec": {
										"group": "s3.aws.upbound.io",
										"names": {"kind": "Bucket", "plural": "buckets"},
										"scope": "Cluster",
										"versions": [{
											"name": "v1beta1",
											"served": true,
											"storage": true,
											"schema": {
												"openAPIV3Schema": {
													"type": "object",
													"properties": {
														"apiVersion": {"type": "string"},
														"kind": {"type": "string"},
														"metadata": {"type": "object"},
														"spec": {
															"type": "object",
															"properties": {
																"forProvider": {
																	"type": "object",
																	"properties": {
																		"region": {"type": "string"}
																	}
																}
															}
														},
														"status": {
															"type": "object",
															"properties": {
																"atProvider": {
																	"type": "object",
																	"properties": {
																		"id": {"type": "string"}
																	}
																}
															}
														}
													}
												}
											}
										}]
									}
								}`),
							}},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "test", Ttl: durationpb.New(response.DefaultTTL)},
					Requirements: &fnv1.Requirements{
						Schemas: map[string]*fnv1.SchemaSelector{
							"example.crossplane.io/v1, Kind=XBucket": {
								ApiVersion: "example.crossplane.io/v1",
								Kind:       "XBucket",
							},
							"s3.aws.upbound.io/v1beta1, Kind=Bucket": {
								ApiVersion: "s3.aws.upbound.io/v1beta1",
								Kind:       "Bucket",
							},
						},
						Resources: map[string]*fnv1.ResourceSelector{
							"example.crossplane.io/v1, Kind=XBucket": {
								ApiVersion: "apiextensions.k8s.io/v1",
								Kind:       "CustomResourceDefinition",
								Match:      &fnv1.ResourceSelector_MatchName{MatchName: "xbuckets.example.crossplane.io"},
							},
							"s3.aws.upbound.io/v1beta1, Kind=Bucket": {
								ApiVersion: "apiextensions.k8s.io/v1",
								Kind:       "CustomResourceDefinition",
								Match:      &fnv1.ResourceSelector_MatchName{MatchName: "buckets.s3.aws.upbound.io"},
							},
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.crossplane.io/v1",
								"kind": "XBucket"
							}`),
						},
						Resources: map[string]*fnv1.Resource{
							"bucket": {
								Resource: resource.MustStructJSON(`{
									"apiVersion": "s3.aws.upbound.io/v1beta1",
									"kind": "Bucket",
									"metadata": {},
									"spec": {
										"forProvider": {
											"region": "us-west-2"
										}
									}
								}`),
								Ready: fnv1.Ready_READY_FALSE,
							},
						},
					},
				},
			},
		},
		"DesiredComposedResourceExcludesObservedFields": {
			reason: "Desired composed resources should only contain template fields, not fields from observed state like provider defaults",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "test"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "kro.fn.crossplane.io/v1beta1",
						"kind": "ResourceGraph",
						"resources": [{
							"id": "bucket",
							"template": {
								"apiVersion": "s3.aws.upbound.io/v1beta1",
								"kind": "Bucket",
								"metadata": {},
								"spec": {
									"forProvider": {
										"region": "us-west-2"
									}
								}
							}
						}],
						"status": {
							"bucketARN": "${bucket.status.atProvider.arn}"
						}
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.crossplane.io/v1",
								"kind": "XBucket",
								"metadata": {"name": "test-bucket"},
								"spec": {}
							}`),
						},
						Resources: map[string]*fnv1.Resource{
							"bucket": {
								Resource: resource.MustStructJSON(`{
									"apiVersion": "s3.aws.upbound.io/v1beta1",
									"kind": "Bucket",
									"metadata": {"name": "test-bucket-abc123"},
									"spec": {
										"forProvider": {
											"region": "us-west-2",
											"objectLockEnabled": false
										},
										"managementPolicies": ["*"]
									},
									"status": {
										"atProvider": {
											"arn": "arn:aws:s3:::test-bucket-abc123",
											"id": "test-bucket-abc123"
										}
									}
								}`),
							},
						},
					},
					RequiredResources: map[string]*fnv1.Resources{
						"example.crossplane.io/v1, Kind=XBucket": {
							Items: []*fnv1.Resource{{
								Resource: resource.MustStructJSON(`{
									"apiVersion": "apiextensions.k8s.io/v1",
									"kind": "CustomResourceDefinition",
									"metadata": {"name": "xbuckets.example.crossplane.io"},
									"spec": {
										"group": "example.crossplane.io",
										"names": {"kind": "XBucket", "plural": "xbuckets"},
										"scope": "Cluster",
										"versions": [{
											"name": "v1",
											"served": true,
											"storage": true,
											"schema": {
												"openAPIV3Schema": {
													"type": "object",
													"properties": {
														"apiVersion": {"type": "string"},
														"kind": {"type": "string"},
														"metadata": {"type": "object"},
														"spec": {"type": "object"},
														"status": {
															"type": "object",
															"properties": {
																"bucketARN": {"type": "string"}
															}
														}
													}
												}
											}
										}]
									}
								}`),
							}},
						},
						"s3.aws.upbound.io/v1beta1, Kind=Bucket": {
							Items: []*fnv1.Resource{{
								Resource: resource.MustStructJSON(`{
									"apiVersion": "apiextensions.k8s.io/v1",
									"kind": "CustomResourceDefinition",
									"metadata": {"name": "buckets.s3.aws.upbound.io"},
									"spec": {
										"group": "s3.aws.upbound.io",
										"names": {"kind": "Bucket", "plural": "buckets"},
										"scope": "Cluster",
										"versions": [{
											"name": "v1beta1",
											"served": true,
											"storage": true,
											"schema": {
												"openAPIV3Schema": {
													"type": "object",
													"properties": {
														"apiVersion": {"type": "string"},
														"kind": {"type": "string"},
														"metadata": {"type": "object"},
														"spec": {
															"type": "object",
															"properties": {
																"forProvider": {
																	"type": "object",
																	"properties": {
																		"region": {"type": "string"},
																		"objectLockEnabled": {"type": "boolean"}
																	}
																},
																"managementPolicies": {
																	"type": "array",
																	"items": {"type": "string"}
																}
															}
														},
														"status": {
															"type": "object",
															"properties": {
																"atProvider": {
																	"type": "object",
																	"properties": {
																		"arn": {"type": "string"},
																		"id": {"type": "string"}
																	}
																}
															}
														}
													}
												}
											}
										}]
									}
								}`),
							}},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "test", Ttl: durationpb.New(response.DefaultTTL)},
					Requirements: &fnv1.Requirements{
						Schemas: map[string]*fnv1.SchemaSelector{
							"example.crossplane.io/v1, Kind=XBucket": {
								ApiVersion: "example.crossplane.io/v1",
								Kind:       "XBucket",
							},
							"s3.aws.upbound.io/v1beta1, Kind=Bucket": {
								ApiVersion: "s3.aws.upbound.io/v1beta1",
								Kind:       "Bucket",
							},
						},
						Resources: map[string]*fnv1.ResourceSelector{
							"example.crossplane.io/v1, Kind=XBucket": {
								ApiVersion: "apiextensions.k8s.io/v1",
								Kind:       "CustomResourceDefinition",
								Match:      &fnv1.ResourceSelector_MatchName{MatchName: "xbuckets.example.crossplane.io"},
							},
							"s3.aws.upbound.io/v1beta1, Kind=Bucket": {
								ApiVersion: "apiextensions.k8s.io/v1",
								Kind:       "CustomResourceDefinition",
								Match:      &fnv1.ResourceSelector_MatchName{MatchName: "buckets.s3.aws.upbound.io"},
							},
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							// Only declared status field, CEL resolved from observed bucket
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.crossplane.io/v1",
								"kind": "XBucket",
								"status": {
									"bucketARN": "arn:aws:s3:::test-bucket-abc123"
								}
							}`),
						},
						Resources: map[string]*fnv1.Resource{
							"bucket": {
								// Only template fields - excludes observed objectLockEnabled and managementPolicies
								Resource: resource.MustStructJSON(`{
									"apiVersion": "s3.aws.upbound.io/v1beta1",
									"kind": "Bucket",
									"metadata": {},
									"spec": {
										"forProvider": {
											"region": "us-west-2"
										}
									}
								}`),
								Ready: fnv1.Ready_READY_TRUE, // Ready because observed resource exists with status
							},
						},
					},
				},
			},
		},
		"ExternalRefUsedInTemplate": {
			reason: "External refs should be fetched and their data available in CEL expressions, but not included in desired output",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "test"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "kro.fn.crossplane.io/v1beta1",
						"kind": "ResourceGraph",
						"resources": [{
							"id": "config",
							"externalRef": {
								"apiVersion": "v1",
								"kind": "ConfigMap",
								"metadata": {
									"name": "platform-config"
								}
							}
						}, {
							"id": "bucket",
							"template": {
								"apiVersion": "s3.aws.upbound.io/v1beta1",
								"kind": "Bucket",
								"metadata": {},
								"spec": {
									"forProvider": {
										"region": "${config.data.region}"
									}
								}
							}
						}],
						"status": {
							"region": "${config.data.region}"
						}
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.crossplane.io/v1",
								"kind": "XBucket",
								"metadata": {"name": "test-bucket", "namespace": "xr-ns"},
								"spec": {}
							}`),
						},
					},
					RequiredSchemas: map[string]*fnv1.Schema{
						"example.crossplane.io/v1, Kind=XBucket": {
							OpenapiV3: resource.MustStructJSON(`{
								"type": "object",
								"properties": {
									"apiVersion": {"type": "string"},
									"kind": {"type": "string"},
									"metadata": {"$ref": "#/components/schemas/ObjectMeta"},
									"spec": {"type": "object"},
									"status": {
										"type": "object",
										"properties": {
											"region": {"type": "string"}
										}
									}
								},
								"components": {
									"schemas": {
										"ObjectMeta": {
											"type": "object",
											"properties": {
												"name": {"type": "string"},
												"namespace": {"type": "string"},
												"labels": {"type": "object", "additionalProperties": {"type": "string"}},
												"annotations": {"type": "object", "additionalProperties": {"type": "string"}}
											}
										}
									}
								}
							}`),
						},
						"s3.aws.upbound.io/v1beta1, Kind=Bucket": {
							OpenapiV3: resource.MustStructJSON(`{
								"type": "object",
								"properties": {
									"apiVersion": {"type": "string"},
									"kind": {"type": "string"},
									"metadata": {"$ref": "#/components/schemas/ObjectMeta"},
									"spec": {
										"type": "object",
										"properties": {
											"forProvider": {
												"type": "object",
												"properties": {
													"region": {"type": "string"}
												}
											}
										}
									}
								},
								"components": {
									"schemas": {
										"ObjectMeta": {
											"type": "object",
											"properties": {
												"name": {"type": "string"},
												"namespace": {"type": "string"},
												"labels": {"type": "object", "additionalProperties": {"type": "string"}},
												"annotations": {"type": "object", "additionalProperties": {"type": "string"}}
											}
										}
									}
								}
							}`),
						},
						"/v1, Kind=ConfigMap": {
							OpenapiV3: resource.MustStructJSON(`{
								"type": "object",
								"properties": {
									"apiVersion": {"type": "string"},
									"kind": {"type": "string"},
									"metadata": {"$ref": "#/components/schemas/ObjectMeta"},
									"data": {
										"type": "object",
										"additionalProperties": {"type": "string"}
									}
								},
								"components": {
									"schemas": {
										"ObjectMeta": {
											"type": "object",
											"properties": {
												"name": {"type": "string"},
												"namespace": {"type": "string"},
												"labels": {"type": "object", "additionalProperties": {"type": "string"}},
												"annotations": {"type": "object", "additionalProperties": {"type": "string"}}
											}
										}
									}
								}
							}`),
						},
					},
					RequiredResources: map[string]*fnv1.Resources{
						"config": {
							Items: []*fnv1.Resource{{
								Resource: resource.MustStructJSON(`{
									"apiVersion": "v1",
									"kind": "ConfigMap",
									"metadata": {"name": "platform-config", "namespace": "xr-ns"},
									"data": {
										"region": "cool-region-2",
										"environment": "production"
									}
								}`),
							}},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "test", Ttl: durationpb.New(response.DefaultTTL)},
					Requirements: &fnv1.Requirements{
						Schemas: map[string]*fnv1.SchemaSelector{
							"example.crossplane.io/v1, Kind=XBucket": {
								ApiVersion: "example.crossplane.io/v1",
								Kind:       "XBucket",
							},
							"s3.aws.upbound.io/v1beta1, Kind=Bucket": {
								ApiVersion: "s3.aws.upbound.io/v1beta1",
								Kind:       "Bucket",
							},
							"/v1, Kind=ConfigMap": {
								ApiVersion: "v1",
								Kind:       "ConfigMap",
							},
						},
						Resources: map[string]*fnv1.ResourceSelector{
							"example.crossplane.io/v1, Kind=XBucket": {
								ApiVersion: "apiextensions.k8s.io/v1",
								Kind:       "CustomResourceDefinition",
								Match:      &fnv1.ResourceSelector_MatchName{MatchName: "xbuckets.example.crossplane.io"},
							},
							"s3.aws.upbound.io/v1beta1, Kind=Bucket": {
								ApiVersion: "apiextensions.k8s.io/v1",
								Kind:       "CustomResourceDefinition",
								Match:      &fnv1.ResourceSelector_MatchName{MatchName: "buckets.s3.aws.upbound.io"},
							},
							"/v1, Kind=ConfigMap": {
								ApiVersion: "apiextensions.k8s.io/v1",
								Kind:       "CustomResourceDefinition",
								Match:      &fnv1.ResourceSelector_MatchName{MatchName: "configmaps."},
							},
							"config": {
								ApiVersion: "v1",
								Kind:       "ConfigMap",
								Match:      &fnv1.ResourceSelector_MatchName{MatchName: "platform-config"},
								Namespace:  ptr.To("xr-ns"),
							},
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.crossplane.io/v1",
								"kind": "XBucket",
								"status": {
									"region": "cool-region-2"
								}
							}`),
						},
						Resources: map[string]*fnv1.Resource{
							"bucket": {
								Resource: resource.MustStructJSON(`{
									"apiVersion": "s3.aws.upbound.io/v1beta1",
									"kind": "Bucket",
									"metadata": {"namespace": "xr-ns"},
									"spec": {
										"forProvider": {
											"region": "cool-region-2"
										}
									}
								}`),
								Ready: fnv1.Ready_READY_FALSE,
							},
						},
					},
				},
			},
		},
		"ExternalRefWithCELExpressionInName": {
			reason: "External refs should support CEL expressions that reference schema.spec fields",
			args: args{
				req: &fnv1.RunFunctionRequest{
					Meta: &fnv1.RequestMeta{Tag: "test"},
					Input: resource.MustStructJSON(`{
						"apiVersion": "kro.fn.crossplane.io/v1beta1",
						"kind": "ResourceGraph",
						"resources": [{
							"id": "config",
							"externalRef": {
								"apiVersion": "v1",
								"kind": "ConfigMap",
								"metadata": {
									"name": "${schema.spec.configMapName}"
								}
							}
						}, {
							"id": "bucket",
							"template": {
								"apiVersion": "s3.aws.upbound.io/v1beta1",
								"kind": "Bucket",
								"metadata": {},
								"spec": {
									"forProvider": {
										"region": "${config.data.region}"
									}
								}
							}
						}],
						"status": {
							"region": "${config.data.region}"
						}
					}`),
					Observed: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.crossplane.io/v1",
								"kind": "XBucket",
								"metadata": {"name": "test-bucket", "namespace": "xr-ns"},
								"spec": {
									"configMapName": "my-platform-config"
								}
							}`),
						},
					},
					RequiredSchemas: map[string]*fnv1.Schema{
						"example.crossplane.io/v1, Kind=XBucket": {
							OpenapiV3: resource.MustStructJSON(`{
								"type": "object",
								"properties": {
									"apiVersion": {"type": "string"},
									"kind": {"type": "string"},
									"metadata": {"$ref": "#/components/schemas/ObjectMeta"},
									"spec": {
										"type": "object",
										"properties": {
											"configMapName": {"type": "string"}
										}
									},
									"status": {
										"type": "object",
										"properties": {
											"region": {"type": "string"}
										}
									}
								},
								"components": {
									"schemas": {
										"ObjectMeta": {
											"type": "object",
											"properties": {
												"name": {"type": "string"},
												"namespace": {"type": "string"},
												"labels": {"type": "object", "additionalProperties": {"type": "string"}},
												"annotations": {"type": "object", "additionalProperties": {"type": "string"}}
											}
										}
									}
								}
							}`),
						},
						"s3.aws.upbound.io/v1beta1, Kind=Bucket": {
							OpenapiV3: resource.MustStructJSON(`{
								"type": "object",
								"properties": {
									"apiVersion": {"type": "string"},
									"kind": {"type": "string"},
									"metadata": {"$ref": "#/components/schemas/ObjectMeta"},
									"spec": {
										"type": "object",
										"properties": {
											"forProvider": {
												"type": "object",
												"properties": {
													"region": {"type": "string"}
												}
											}
										}
									}
								},
								"components": {
									"schemas": {
										"ObjectMeta": {
											"type": "object",
											"properties": {
												"name": {"type": "string"},
												"namespace": {"type": "string"},
												"labels": {"type": "object", "additionalProperties": {"type": "string"}},
												"annotations": {"type": "object", "additionalProperties": {"type": "string"}}
											}
										}
									}
								}
							}`),
						},
						"/v1, Kind=ConfigMap": {
							OpenapiV3: resource.MustStructJSON(`{
								"type": "object",
								"properties": {
									"apiVersion": {"type": "string"},
									"kind": {"type": "string"},
									"metadata": {"$ref": "#/components/schemas/ObjectMeta"},
									"data": {
										"type": "object",
										"additionalProperties": {"type": "string"}
									}
								},
								"components": {
									"schemas": {
										"ObjectMeta": {
											"type": "object",
											"properties": {
												"name": {"type": "string"},
												"namespace": {"type": "string"},
												"labels": {"type": "object", "additionalProperties": {"type": "string"}},
												"annotations": {"type": "object", "additionalProperties": {"type": "string"}}
											}
										}
									}
								}
							}`),
						},
					},
					RequiredResources: map[string]*fnv1.Resources{
						"config": {
							Items: []*fnv1.Resource{{
								Resource: resource.MustStructJSON(`{
									"apiVersion": "v1",
									"kind": "ConfigMap",
									"metadata": {"name": "my-platform-config", "namespace": "xr-ns"},
									"data": {
										"region": "us-west-2",
										"environment": "production"
									}
								}`),
							}},
						},
					},
				},
			},
			want: want{
				rsp: &fnv1.RunFunctionResponse{
					Meta: &fnv1.ResponseMeta{Tag: "test", Ttl: durationpb.New(response.DefaultTTL)},
					Requirements: &fnv1.Requirements{
						Schemas: map[string]*fnv1.SchemaSelector{
							"example.crossplane.io/v1, Kind=XBucket": {
								ApiVersion: "example.crossplane.io/v1",
								Kind:       "XBucket",
							},
							"s3.aws.upbound.io/v1beta1, Kind=Bucket": {
								ApiVersion: "s3.aws.upbound.io/v1beta1",
								Kind:       "Bucket",
							},
							"/v1, Kind=ConfigMap": {
								ApiVersion: "v1",
								Kind:       "ConfigMap",
							},
						},
						Resources: map[string]*fnv1.ResourceSelector{
							"example.crossplane.io/v1, Kind=XBucket": {
								ApiVersion: "apiextensions.k8s.io/v1",
								Kind:       "CustomResourceDefinition",
								Match:      &fnv1.ResourceSelector_MatchName{MatchName: "xbuckets.example.crossplane.io"},
							},
							"s3.aws.upbound.io/v1beta1, Kind=Bucket": {
								ApiVersion: "apiextensions.k8s.io/v1",
								Kind:       "CustomResourceDefinition",
								Match:      &fnv1.ResourceSelector_MatchName{MatchName: "buckets.s3.aws.upbound.io"},
							},
							"/v1, Kind=ConfigMap": {
								ApiVersion: "apiextensions.k8s.io/v1",
								Kind:       "CustomResourceDefinition",
								Match:      &fnv1.ResourceSelector_MatchName{MatchName: "configmaps."},
							},
							// Key assertion: the external ref name should be evaluated from CEL expression
							// "${schema.spec.configMapName}" -> "my-platform-config"
							"config": {
								ApiVersion: "v1",
								Kind:       "ConfigMap",
								Match:      &fnv1.ResourceSelector_MatchName{MatchName: "my-platform-config"},
								Namespace:  ptr.To("xr-ns"),
							},
						},
					},
					Desired: &fnv1.State{
						Composite: &fnv1.Resource{
							Resource: resource.MustStructJSON(`{
								"apiVersion": "example.crossplane.io/v1",
								"kind": "XBucket",
								"status": {
									"region": "us-west-2"
								}
							}`),
						},
						Resources: map[string]*fnv1.Resource{
							"bucket": {
								Resource: resource.MustStructJSON(`{
									"apiVersion": "s3.aws.upbound.io/v1beta1",
									"kind": "Bucket",
									"metadata": {"namespace": "xr-ns"},
									"spec": {
										"forProvider": {
											"region": "us-west-2"
										}
									}
								}`),
								Ready: fnv1.Ready_READY_FALSE,
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := &Function{log: logging.NewNopLogger()}
			rsp, err := f.RunFunction(tc.args.ctx, tc.args.req)

			if diff := cmp.Diff(tc.want.rsp, rsp, protocmp.Transform()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want rsp, +got rsp:\n%s", tc.reason, diff)
			}

			if diff := cmp.Diff(tc.want.err, err, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("%s\nf.RunFunction(...): -want err, +got err:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestResolveSchemaRefs(t *testing.T) {
	type args struct {
		input *structpb.Struct
	}
	type want struct {
		schema *spec.Schema
		err    error
	}

	cases := map[string]struct {
		reason string
		args   args
		want   want
	}{
		"NilInput": {
			reason: "Nil input should return error",
			args:   args{input: nil},
			want:   want{err: errors.New("schema struct is nil")},
		},
		"NoComponents": {
			reason: "Schema without components should pass through unchanged",
			args: args{
				input: mustStruct(map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{"type": "string"},
					},
				}),
			},
			want: want{
				schema: &spec.Schema{
					SchemaProps: spec.SchemaProps{
						Type: []string{"object"},
						Properties: map[string]spec.Schema{
							"name": {SchemaProps: spec.SchemaProps{Type: []string{"string"}}},
						},
					},
				},
			},
		},
		"SimpleRefResolution": {
			reason: "Refs to components should be resolved inline",
			args: args{
				input: mustStruct(map[string]any{
					"type": "object",
					"properties": map[string]any{
						"timestamp": map[string]any{
							"$ref": "#/components/schemas/Time",
						},
					},
					"components": map[string]any{
						"schemas": map[string]any{
							"Time": map[string]any{
								"type":        "string",
								"format":      "date-time",
								"description": "A timestamp",
							},
						},
					},
				}),
			},
			want: want{
				schema: &spec.Schema{
					SchemaProps: spec.SchemaProps{
						Type: []string{"object"},
						Properties: map[string]spec.Schema{
							"timestamp": {
								SchemaProps: spec.SchemaProps{
									Type:        []string{"string"},
									Format:      "date-time",
									Description: "A timestamp",
								},
							},
						},
					},
				},
			},
		},
		"NestedRefResolution": {
			reason: "Nested refs (ref pointing to schema with another ref) should be resolved",
			args: args{
				input: mustStruct(map[string]any{
					"type": "object",
					"properties": map[string]any{
						"metadata": map[string]any{
							"$ref": "#/components/schemas/ObjectMeta",
						},
					},
					"components": map[string]any{
						"schemas": map[string]any{
							"ObjectMeta": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"creationTimestamp": map[string]any{
										"$ref": "#/components/schemas/Time",
									},
								},
							},
							"Time": map[string]any{
								"type":   "string",
								"format": "date-time",
							},
						},
					},
				}),
			},
			want: want{
				schema: &spec.Schema{
					SchemaProps: spec.SchemaProps{
						Type: []string{"object"},
						Properties: map[string]spec.Schema{
							"metadata": {
								SchemaProps: spec.SchemaProps{
									Type: []string{"object"},
									Properties: map[string]spec.Schema{
										"creationTimestamp": {
											SchemaProps: spec.SchemaProps{
												Type:   []string{"string"},
												Format: "date-time",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"ArrayItemRef": {
			reason: "Refs in array items should be resolved",
			args: args{
				input: mustStruct(map[string]any{
					"type": "object",
					"properties": map[string]any{
						"items": map[string]any{
							"type": "array",
							"items": map[string]any{
								"$ref": "#/components/schemas/Item",
							},
						},
					},
					"components": map[string]any{
						"schemas": map[string]any{
							"Item": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"id": map[string]any{"type": "integer"},
								},
							},
						},
					},
				}),
			},
			want: want{
				schema: &spec.Schema{
					SchemaProps: spec.SchemaProps{
						Type: []string{"object"},
						Properties: map[string]spec.Schema{
							"items": {
								SchemaProps: spec.SchemaProps{
									Type: []string{"array"},
									Items: &spec.SchemaOrArray{
										Schema: &spec.Schema{
											SchemaProps: spec.SchemaProps{
												Type: []string{"object"},
												Properties: map[string]spec.Schema{
													"id": {SchemaProps: spec.SchemaProps{Type: []string{"integer"}}},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := resolveSchemaRefs(tc.args.input)

			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("%s\nresolveSchemaRefs(...): -want err, +got err:\n%s", tc.reason, diff)
			}

			// Ignore ExtraProps (contains leftover "components" from JSON) and unexported Ref fields
			opts := cmp.Options{
				cmpopts.IgnoreUnexported(spec.Ref{}, jsonreference.Ref{}),
				cmpopts.IgnoreFields(spec.Schema{}, "ExtraProps"),
			}
			if diff := cmp.Diff(tc.want.schema, got, opts); diff != "" {
				t.Errorf("%s\nresolveSchemaRefs(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

// mustStruct creates a structpb.Struct from a map, panicking on error.
func mustStruct(m map[string]any) *structpb.Struct {
	s, err := structpb.NewStruct(m)
	if err != nil {
		panic(err)
	}
	return s
}

# Dynamic Label and Annotation Keys

This example shows how to compute label and annotation keys from the XR spec,
without any changes to the vendored `kro/` libraries.

function-kro (like upstream KRO) treats a map key written directly in a template
as a literal string, so `${...}` in key position is not evaluated. The supported
way to compute a key is to assign the whole map with a single CEL expression. A
CEL map literal lets the keys themselves be expressions, so you can build keys
from the XR spec and mix them with static keys in the same literal:

```yaml
metadata:
  labels: >-
    ${{
      "serviceid.example.crossplane.io/" + schema.spec.serviceID: "",
      "team.example.crossplane.io/" + schema.spec.team: "owner",
      "app.kubernetes.io/managed-by": "function-kro"
    }}
```

## Run it locally

Run function-kro in one terminal:

```shell
go run . --insecure --debug
```

Render the example in another terminal:

```shell
cd example/dynamic-label-keys
crossplane render xr.yaml composition.yaml functions.yaml --required-schemas=schemas/ -r
```

The rendered `ConfigMap` carries the computed keys:

```yaml
metadata:
  annotations:
    example.crossplane.io/production: active
  labels:
    app.kubernetes.io/managed-by: function-kro
    serviceid.example.crossplane.io/svc-42: ""
    team.example.crossplane.io/platform: owner
```

`schemas/labeldemo.json` holds the OpenAPI schema for the `LabelDemo` XR. It is
only needed for local `crossplane render`, which has no cluster to read schemas
from. On a real cluster Crossplane supplies the schemas and you apply `xrd.yaml`,
`composition.yaml`, and `xr.yaml` directly. The `ConfigMap` schema is not
supplied here because function-kro resolves built-in Kubernetes types from its
own compiled-in definitions.

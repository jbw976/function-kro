# Conditions Example

Declares custom status conditions on the XR with `runtime.newCondition(...)`.
See the [Conditions Example](../README.md#conditions-example) in the top-level
README for the full walkthrough on a live cluster.

## Render it locally

Start function-kro from the repository root:

```shell
go run . --insecure --debug
```

Render the example:

```shell
cd example/conditions
crossplane render -r -x xr.yaml composition.yaml functions.yaml --required-schemas=schemas/
```

The ConfigMap has no dependencies, so this renders the complete result. Only
`ReplicasConfigured` appears: it reads the XR spec, which is available on the
first pass. `ConfigMapReady` reads the composed ConfigMap, and `render` has no
observed state, so that condition is left off until a real reconcile observes
the ConfigMap.

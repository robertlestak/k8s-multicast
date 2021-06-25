# k8s-multicast

Accept a `HTTP` request to this service and send this request to all pods in the service simultaneously.

By design, a request to a Kubernetes service will be load balanced across the N pods in that service.

For cache busting / hot reloading, the `/__reload` request will only be routed to a single pod at random.

This application receives the request and will multicast that request to all pods in the service.

## Configuration

### Kubernetes RBAC

```
kubectl apply -f k8s/rbac.yaml
```

### Environment

You can define the `NAMESPACE_NAME` and `ENDPOINT_NAME` environment variables and these will be used as defaults when retrieving the endpoint.

### Form Data

Assuming your RBAC is configured appropriately, you can also pass in `?namespace=NAMESPACE&endpoint=ENDPOINT` query string params / form data to dynamically configure.

If provided, these will take precendence over the default values configured via environment variables.

# OpenAPI Validator - Envoy/Istio Sidecar

A production-ready gRPC authorization filter for Envoy/Istio that validates incoming HTTP requests against OpenAPI 3.0 specifications. This sidecar ensures API compliance by enforcing request validation at the service mesh level.

## Overview

**openapi-validator** is a gRPC server that integrates with Envoy as an [external authorization service](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/filters/http/ext_authz/v3/ext_authz.proto). It validates HTTP requests before they reach your backend services, rejecting requests that don't conform to your OpenAPI specification.

### Key Features

- **OpenAPI 3.0 Validation**: Validates HTTP requests against OpenAPI 3.0 specifications
- **Envoy Integration**: Implements Envoy's gRPC Authorization API (`envoy.service.auth.v3.Authorization`)
- **Request Validation**: Checks HTTP methods, paths, headers, query parameters, and request bodies
- **Detailed Logging**: Comprehensive logging for debugging and monitoring validation failures
- **Container Ready**: Fully containerized with Docker for Kubernetes/Istio deployment
- **Istio Mesh Support**: Designed for Istio service mesh with authorization policies

## Architecture

### How It Works

```
HTTP Request → Envoy → gRPC Check Request → openapi-validator → Validation → gRPC Response → Allow/Deny
```

1. **Request Reception**: Envoy sends an authorization check request to the validator
2. **OpenAPI Parsing**: Validator parses the HTTP request metadata
3. **Validation**: Request is validated against the loaded OpenAPI specification
4. **Response**: Returns either:
   - `OK (codes.OK)`: Request is valid, allow it through
   - `PermissionDenied (codes.PermissionDenied)`: Request is invalid, reject with error details

### Components

- **main.go**: Core gRPC server implementing the `Check()` method from Envoy's auth API
- **build.sh**: Build script for Go dependencies and binary compilation
- **dockerfile**: Multi-stage Docker build configuration
- **resources/**: Istio and Kubernetes configuration files
  - `openapi.json` / `httpbin-oas-swagger2.0.json`: OpenAPI specification files
  - `authorization-policy.yaml`: Istio authorization policy defining when to invoke the validator
  - `mesh-config.yaml`: Envoy filter configuration for external authorization
  - `istio-ingressgateway-patch.yaml`: Patch to inject validator sidecar into ingress gateway
  - `service.yaml`: Kubernetes Service definition for the validator

## Prerequisites

- Go 1.25.4 or higher
- Docker (for containerization)
- Kubernetes cluster with Istio installed (for deployment)
- kind or minikube (for local testing)
- kubectl configured for your cluster

## Building from Source

### Local Build

```bash
# Install dependencies
go mod download

# Build for Linux (since it typically runs in containers)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o openapi-validator main.go

# Build for macOS (for local testing)
go build -o openapi-validator main.go
```

### Using the Build Script

```bash
# The build.sh script sets up fresh dependencies
bash build.sh

# Then build the binary
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o openapi-validator main.go
```

## Docker

### Building a Docker Image

```bash
# Build the Docker image
docker build -t oas-validator:latest .

# Tag for local registry
docker tag oas-validator:latest oas-validator:local
```

### Running Locally in Docker

```bash
# Mount your OpenAPI spec into the container
docker run -p 9000:9000 \
  -v /path/to/openapi.json:/etc/openapi/openapi.json \
  oas-validator:local

# Test with a gRPC client (requires protobuf definitions)
# The validator listens on port 9000
```

## Kubernetes / Istio Deployment

### Prerequisites

- Istio 1.10+ installed in your cluster
- The `istio-system` namespace created

### Deployment Steps

```bash
# 1. Apply mesh configuration (enables external authorization filter)
kubectl apply -f resources/mesh-config.yaml

# 2. Restart istiod to pick up the new filter
kubectl rollout restart deployment istiod -n istio-system

# 3. Create ConfigMap with the OpenAPI specification
kubectl create configmap oas-spec-config \
  -n istio-system \
  --from-file=openapi.json=resources/httpbin-oas.json

# 4. Load Docker image into kind (if using kind)
kind load docker-image oas-validator:local --name ok-resilience

# 5. Patch the ingress gateway to add the validator sidecar
kubectl patch deployment istio-ingressgateway \
  -n istio-system \
  --patch-file resources/istio-ingressgateway-patch.yaml

# 6. Restart the ingress gateway
kubectl rollout restart deployment istio-ingressgateway -n istio-system

# 7. Apply the authorization policy
kubectl apply -f resources/authorization-policy.yaml
```

### Updating the OpenAPI Specification

```bash
# Update the ConfigMap with a new spec
kubectl create configmap oas-spec-config \
  -n istio-system \
  --from-file=openapi.json=resources/httpbin-oas.json \
  --dry-run=client -o yaml | kubectl apply -f -

# Restart the validator pods to reload the spec
kubectl rollout restart deployment istio-ingressgateway -n istio-system
```

## Configuration

### Environment Variables

The validator respects the following environment variables:

- `OAS_SPEC_PATH` (default: `/etc/openapi/openapi.json`): Path to the OpenAPI specification file inside the container

### Container Entrypoint

The validator binary is the container entrypoint. It:
1. Reads the OpenAPI spec from `/etc/openapi/openapi.json`
2. Validates it as a proper OpenAPI 3.0 document
3. Starts a gRPC server on `0.0.0.0:9000`
4. Waits for authorization check requests from Envoy

## Logging

The validator outputs structured logs to stdout:

```
[INFO] Spec file read successfully (12345 bytes)
[INFO] Spec verified as valid OpenAPI 3.0.x
[SUCCESS] gRPC Server starting on :9000
[CHECK] Method: GET | Path: /api/users | Host: example.com
[VALIDATION-FAIL] Context: query | Issue: Missing required parameter 'id'
[REJECTED] GET /api/users - Count: 1
[ALLOWED] POST /api/users
```

### Log Levels

- `[DEBUG]`: Low-level diagnostic information
- `[INFO]`: General informational messages
- `[CHECK]`: Incoming check request details
- `[VALIDATION-FAIL]`: Validation error for a specific request component
- `[REJECTED]`: Request validation failed overall
- `[ALLOWED]`: Request validation passed
- `[SUCCESS]`: Service started successfully
- `[FATAL]`: Fatal errors during startup (causes exit)

## Dependencies

### Go Packages

- **libopenapi** (v0.31.2): OpenAPI document parsing and modeling
- **libopenapi-validator** (v0.10.2): Request validation against OpenAPI specs
- **google.golang.org/grpc** (v1.78.0): gRPC framework
- **google.golang.org/protobuf**: Protocol buffer support
- **envoyproxy/go-control-plane** (v1.36.0): Envoy authorization API definitions

## File Structure

```
openapi-validator/
├── main.go                          # Main application code
├── go.mod                           # Go module definition
├── go.sum                           # Go dependency checksums
├── build.sh                         # Build script for dependencies
├── dockerfile                       # Docker image definition
├── README.md                        # This file
├── .gitignore                       # Git ignore rules
└── resources/
    ├── openapi.json                 # OpenAPI 3.0 specification
    ├── httpbin-oas-swagger2.0.json  # Swagger 2.0 specification (alternative)
    ├── authorization-policy.yaml    # Istio authorization policy
    ├── mesh-config.yaml             # Envoy external auth filter config
    ├── mesh-fix.yaml                # Additional mesh configuration
    ├── istio-ingressgateway-patch.yaml  # Ingress gateway sidecar patch
    └── service.yaml                 # Kubernetes Service definition
```

## Development

### Running Locally

```bash
# Start the validator with a local OpenAPI spec
go run main.go

# This expects the OpenAPI spec at /etc/openapi/openapi.json
# For testing, create this path or modify the specPath in main.go
```

### Testing

The validator implements the `envoy.service.auth.v3.Authorization` gRPC service. You can test it using:

```bash
# With gRPCurl
grpcurl -plaintext \
  -d @ \
  localhost:9000 \
  envoy.service.auth.v3.Authorization/Check < request.json
```

### Debugging

1. **Enable verbose logging**: Check stdout/stderr logs for `[VALIDATION-FAIL]` messages
2. **Validate OpenAPI spec**: Use online tools or `libopenapi` CLI to validate your spec
3. **Check filter config**: Review `mesh-config.yaml` for correct filter configuration
4. **Inspect logs**: Use `kubectl logs <pod-name>` for container logs

## Troubleshooting

### "Could not read spec" Error

```
FATAL: Could not read spec at /etc/openapi/openapi.json: no such file or directory
```

**Solution**: Ensure the ConfigMap is created and mounted correctly:

```bash
kubectl get configmap oas-spec-config -n istio-system
kubectl describe configmap oas-spec-config -n istio-system
```

### "Spec is not valid OpenAPI 3" Error
```
FATAL: Spec is not valid OpenAPI 3: [validation errors]
```

**Solution**: Validate your OpenAPI specification using online validators or tools like Swagger Editor.

### Requests Not Being Validated

**Check**:

1. Authorization policy is applied: `kubectl get authorizationpolicies -n istio-system`
2. Validator pods are running: `kubectl get pods -n istio-system | grep openapi`
3. Ingress gateway has the sidecar: `kubectl get pods -n istio-system istio-ingressgateway-* -o jsonpath='{.items[0].spec.containers[*].name}'`

### High Latency

The validator adds minimal latency (~5-10ms typically). If you observe high latency:

1. Check validator pod resources and limits
2. Monitor CPU and memory usage
3. Consider caching validation results for identical requests

## Performance Considerations

- **Memory**: ~50-100MB typical for a single validator instance
- **Latency**: 5-10ms per request validation (varies with spec complexity)
- **Throughput**: Supports thousands of requests/sec depending on hardware
- **Scalability**: Use Kubernetes Horizontal Pod Autoscaling for multiple instances

## Security Considerations

- The validator only validates against the OpenAPI spec—it does NOT authenticate users
- Combine with other Istio security policies (authentication, RBAC) for complete security
- Keep the OpenAPI specification up-to-date to reflect actual API changes
- Review validation error logs regularly to catch unexpected request patterns

# OpenAPI Validator - Envoy/Istio Sidecar

A gRPC authorization filter for Envoy/Istio that validates incoming HTTP requests against OpenAPI 3.0 specifications.

## Quick Start

### Prerequisites

- **Docker** and **Docker Compose** (or Kubernetes with Istio 1.10+)
- **kubectl** (for Kubernetes deployment)
- **kind** (optional, for local Kubernetes cluster)

### Build the Validator

```bash
# Install dependencies
go mod download

# Build for Linux (since it typically runs in containers)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o openapi-validator main.go

# Build for macOS (for local testing)
go build -o openapi-validator main.go
```

### Build Docker Image

```bash
# Build the Docker image
docker build -t oas-validator:latest .
```

### Deploy to Kubernetes/Istio

Use the helper scripts to quickly set up and deploy httpbin and Istio in a kind cluster:

```bash
# 1. Set up a kind cluster
bash scripts/setup-kind.sh

# 2. Install Istio
bash scripts/install-istio.sh

# 3. Deploy httpbin for testing
bash scripts/deploy-httpbin.sh

# 4. Generate test traffic
bash scripts/generate-traffic.sh
```

Upload the `oas-validator` Docker image to the kind cluster:

```bash
# Load the Docker image into the kind cluster
kind load docker-image oas-validator:latest --name kind-oap-validator
```

Enable the OpenAPI validator by applying the following Istio configurations:

```bash
#1. Apply the mesh config to enable the gRPC authorization filter
kubectl apply -f resources/oas-validator-service/mesh-config.yaml
#2. Restart istiod pod to pick up the changes
kubectl rollout restart deployment istiod -n istio-system

#3. Create ConfigMap for OpenAPI spec
kubectl create configmap oas-spec-config \
  -n istio-system \
  --from-file=openapi.json=resources/oas-validator-service/httpbin-oas.json

#4. Patch the Istio ingress gateway to include the validator filter
kubectl patch deployment istio-ingressgateway \
  -n istio-system \
  --patch-file resources/oas-validator-service/istio-ingressgateway-patch.yaml

#kubectl rollout restart deployment istio-ingressgateway -n istio-system

#5. Deploy the OAS Validator service
kubectl apply -f resources/oas-validator-service/service.yaml

#6. Wait for the validator pod to be running
kubectl wait --for=condition=available --timeout=120s deployment/istio-ingressgateway -n istio-system

#7. Verify the istioctl proxy-config to ensure the oas-validator endpoint is configured
istioctl proxy-config listeners istio-ingressgateway-<pod-suffix> -n istio-system
istioctl proxy-config endpoints istio-ingressgateway-<pod-suffix> -n istio-system

#7. Apply the authorization policy to enforce validation
kubectl apply -f resources/oas-validator-service/authorization-policy.yaml
```

### How It Works

```
HTTP Request → Envoy → gRPC Authorization Check → OAS Validator → Validate → Allow/Deny Response
```

The validator receives HTTP requests from Envoy, validates them against your OpenAPI spec, and returns:
- **Allow**: Request conforms to OpenAPI spec
- **Deny**: Request violates OpenAPI spec (with error details)


## Configuration

### OpenAPI Specification

Place your OpenAPI 3.0 specification at `resources/oas-validator-service/httpbin-oas.json`. The validator will:
1. Read the spec from `/etc/openapi/openapi.json` inside the container
2. Validate it as a proper OpenAPI 3.0 document
3. Start the gRPC server on `0.0.0.0:9000`

### Updating the Specification

```bash
# Update the ConfigMap with a new spec
kubectl create configmap oas-spec-config \
  -n istio-system \
  --from-file=openapi.json=resources/oas-validator-service/httpbin-oas.json \
  --dry-run=client -o yaml | kubectl apply -f -

# Restart the validator pods
kubectl rollout restart deployment istio-ingressgateway -n istio-system
```


## Project Structure

```
openapi-validator/
├── main.go                          # gRPC authorization server
├── go.mod / go.sum                  # Go dependencies
├── build.sh                         # Build script
├── dockerfile                       # Docker image
├── scripts/                         # Deployment helpers
│   ├── setup-kind.sh               # Create kind cluster
│   ├── install-istio.sh            # Install Istio
│   ├── deploy-httpbin.sh           # Deploy test service
│   ├── generate-traffic.sh         # Generate test traffic
│   └── cleanup.sh                  # Clean up resources
└── resources/
    ├── httpbin/                    # httpbin configurations
    ├── istio/                      # Istio configs (Kiali, Prometheus)
    └── oas-validator-service/      # Validator deployment files
        ├── httpbin-oas.json        # Your OpenAPI spec
        ├── authorization-policy.yaml
        ├── mesh-config.yaml
        ├── istio-ingressgateway-patch.yaml
        └── service.yaml
```

## Troubleshooting

**Spec file not found:**
```bash
kubectl get configmap oas-spec-config -n istio-system
```

**Invalid OpenAPI spec:**
Validate your spec using online tools like [Swagger Editor](https://editor.swagger.io/)

**Validator pod not running:**
```bash
kubectl get pods -n istio-system | grep oas-validator
kubectl logs -n istio-system <pod-name>
```

**Authorization policy not working:**
```bash
kubectl get authorizationpolicies -n istio-system
kubectl describe authorizationpolicy -n istio-system <policy-name>
```

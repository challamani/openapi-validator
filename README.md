# openapi-validator
openapi-validator

## TinyGo Setup

```shell
brew tap tinygo-org/tools
brew install tinygo
tinygo version
```

## Build Instructions

```shell
# Initialize module
go mod init openapi-validator

# Tidy up
go mod tidy

# Build for Linux (since it runs in a container)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o openapi-validator main.go
```

## Docker Build & Load into Kind

```shell
docker build -t oas-validator:local .
kind load docker-image oas-validator:local --name ok-resilience
```

```shell
kubectl apply -f resources/mesh-config.yaml
kubectl rollout restart deployment istiod -n istio-system

kubectl create configmap oas-spec-config -n istio-system --from-file=openapi.json=resources/httpbin-oas.json
kubectl patch deployment istio-ingressgateway -n istio-system --patch-file resources/istio-ingressgateway-patch.yaml
kubectl rollout restart deployment istio-ingressgateway -n istio-system

kubectl apply -f resources/authorization-policy.yaml
```

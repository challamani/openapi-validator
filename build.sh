rm -rf go.mod go.sum
go mod init openapi-validator
# Get exactly what we need
go get github.com/pb33f/libopenapi@v0.16.0
go get github.com/pb33f/libopenapi-validator@v0.10.2
go get google.golang.org/grpc
go get github.com/envoyproxy/go-control-plane/envoy/service/auth/v3
go mod tidy

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	auth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	envoy_type "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
)

type server struct {
	v validator.Validator
}

func (s *server) Check(ctx context.Context, req *auth.CheckRequest) (*auth.CheckResponse, error) {
	attr := req.GetAttributes().GetRequest().GetHttp()
	if attr == nil {
		log.Println("[DEBUG] Received non-HTTP check request, skipping...")
		return &auth.CheckResponse{Status: &status.Status{Code: int32(codes.OK)}}, nil
	}

	// 1. Detailed Trace Log for incoming Request
	log.Printf("[CHECK] Method: %s | Path: %s | Host: %s", attr.GetMethod(), attr.GetPath(), attr.GetHost())

	dummyReq, _ := http.NewRequest(attr.GetMethod(), attr.GetPath(), strings.NewReader(attr.GetBody()))
	for k, v := range attr.GetHeaders() {
		dummyReq.Header.Set(k, v)
	}

	// 2. Run Validation
	ok, validationErrs := s.v.ValidateHttpRequest(dummyReq)

	if !ok {
		var errorMsgs []string
		for _, e := range validationErrs {
			// Added detail: Log the specific part of the request that failed (query, header, body)
			log.Printf("[VALIDATION-FAIL] Context: %s | Issue: %s", e.Context, e.Message)
			errorMsgs = append(errorMsgs, fmt.Sprintf("[%s] %s", e.Context, e.Message))
		}

		log.Printf("[REJECTED] %s %s - Count: %d", attr.GetMethod(), attr.GetPath(), len(validationErrs))

		return &auth.CheckResponse{
			Status: &status.Status{Code: int32(codes.PermissionDenied)},
			HttpResponse: &auth.CheckResponse_DeniedResponse{
				DeniedResponse: &auth.DeniedHttpResponse{
					Status: &envoy_type.HttpStatus{Code: envoy_type.StatusCode_BadRequest},
					Body:   fmt.Sprintf("OAS Validation Failed:\n%s", strings.Join(errorMsgs, "\n")),
				},
			},
		}, nil
	}

	// 3. Success Log
	log.Printf("[ALLOWED] %s %s", attr.GetMethod(), attr.GetPath())

	return &auth.CheckResponse{
		Status:       &status.Status{Code: int32(codes.OK)},
		HttpResponse: &auth.CheckResponse_OkResponse{OkResponse: &auth.OkHttpResponse{}},
	}, nil
}

func main() {
	log.Println("==========================================")
	log.Println("Initializing OAS3 Validator Sidecar...")
	log.Println("==========================================")

	specPath := "/etc/openapi/openapi.json"
	specBytes, err := os.ReadFile(specPath)
	if err != nil {
		log.Fatalf("FATAL: Could not read spec at %s: %v", specPath, err)
	}
	log.Printf("[INFO] Spec file read successfully (%d bytes)", len(specBytes))

	// 1. Create the Document
	doc, err := libopenapi.NewDocument(specBytes)
	if err != nil {
		log.Fatalf("FATAL: Failed to parse document: %v", err)
	}

	// 2. Verify V3 model
	_, v3Errors := doc.BuildV3Model()
	if v3Errors != nil {
		log.Fatalf("FATAL: Spec is not valid OpenAPI 3: %v", v3Errors)
	}
	log.Println("[INFO] Spec verified as valid OpenAPI 3.0.x")

	// 3. Create Validator
	v, vErrs := validator.NewValidator(doc)
	if vErrs != nil {
		log.Fatalf("FATAL: Failed to create validator instance: %v", vErrs)
	}

	port := ":9000"
	lis, err := net.Listen("tcp", "0.0.0.0"+port)
	if err != nil {
		log.Fatalf("FATAL: Failed to listen on %s: %v", port, err)
	}

	log.Printf("[SUCCESS] gRPC Server starting on %s", port)
	log.Println("Waiting for Envoy check requests...")

	s := grpc.NewServer()
	auth.RegisterAuthorizationServer(s, &server{v: v})
	reflection.Register(s)

	if err := s.Serve(lis); err != nil {
		log.Fatalf("FATAL: gRPC server failed: %v", err)
	}
}

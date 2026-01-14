package main

import (
	"context"
	"encoding/json"
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

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	auth "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	envoy_type "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
)

type server struct {
	v validator.Validator
}

// ValidationErrorDetail provides detailed validation error information
type ValidationErrorDetail struct {
	Field    string `json:"field"`
	Location string `json:"location"`
	Message  string `json:"message"`
	Value    string `json:"value,omitempty"`
}

// ValidationErrorResponse is the JSON error response structure
type ValidationErrorResponse struct {
	Error   string                  `json:"error"`
	Message string                  `json:"message"`
	Details []ValidationErrorDetail `json:"details"`
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
		var errorDetails []ValidationErrorDetail

		for _, e := range validationErrs {
			// Extract location and field information from schema validation errors
			if len(e.SchemaValidationErrors) > 0 {
				for _, schemaErr := range e.SchemaValidationErrors {
					schemaDetail := ValidationErrorDetail{
						Field:    schemaErr.FieldName,
						Location: schemaErr.Location,
						Message:  schemaErr.Reason,
					}

					// Include field path if available (JSONPath format)
					if schemaErr.FieldPath != "" {
						schemaDetail.Field = schemaErr.FieldPath
					}

					errorDetails = append(errorDetails, schemaDetail)

					// Log detailed validation failure
					log.Printf("[VALIDATION-FAIL] Field: %s | Location: %s | Reason: %s",
						schemaErr.FieldPath, schemaErr.Location, schemaErr.Reason)
				}
			} else {
				// For non-schema errors (e.g., missing operations, parameter errors, content-type errors)
				detail := ValidationErrorDetail{
					Message: e.Message,
				}

				// Add validation context
				if e.ValidationType != "" && e.ValidationSubType != "" {
					detail.Field = fmt.Sprintf("%s.%s", e.ValidationType, e.ValidationSubType)
				} else if e.ValidationType != "" {
					detail.Field = e.ValidationType
				}

				// Add parameter name if available
				if e.ParameterName != "" {
					detail.Field = e.ParameterName
				}

				// Add the reason as the message (cleaner than the generic message)
				if e.Reason != "" {
					detail.Message = e.Reason
				}

				// Add path information if available
				if e.RequestPath != "" {
					detail.Location = fmt.Sprintf("%s %s", e.RequestMethod, e.RequestPath)
				}

				errorDetails = append(errorDetails, detail)

				log.Printf("[VALIDATION-FAIL] Type: %s.%s | Reason: %s",
					e.ValidationType, e.ValidationSubType, e.Reason)
			}
		}

		log.Printf("[REJECTED] %s %s - Errors: %d", attr.GetMethod(), attr.GetPath(), len(validationErrs))

		// Create JSON error response
		errorResponse := ValidationErrorResponse{
			Error:   "OAS_VALIDATION_FAILED",
			Message: "Request does not conform to OpenAPI specification",
			Details: errorDetails,
		}

		jsonBody, err := json.Marshal(errorResponse)
		if err != nil {
			log.Printf("[ERROR] Failed to marshal JSON response: %v", err)
			jsonBody = []byte(`{"error":"OAS_VALIDATION_FAILED","message":"Request validation failed"}`)
		}

		return &auth.CheckResponse{
			Status: &status.Status{Code: int32(codes.PermissionDenied)},
			HttpResponse: &auth.CheckResponse_DeniedResponse{
				DeniedResponse: &auth.DeniedHttpResponse{
					Status: &envoy_type.HttpStatus{Code: envoy_type.StatusCode_BadRequest},
					Body:   string(jsonBody),
					Headers: []*core.HeaderValueOption{
						{
							Header: &core.HeaderValue{
								Key:   "content-type",
								Value: "application/json",
							},
						},
					},
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

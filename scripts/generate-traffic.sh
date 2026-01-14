#!/bin/bash

set -euo pipefail

MODE=${1:-status}
REQUEST_COUNT=${2:-50}
STATUS_CODE=${3:-200}
INGRESS_IP=$(kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

if [[ -z "${INGRESS_IP}" ]]; then
    echo "Could not determine istio-ingressgateway external IP. Ensure the gateway service is provisioned."
    exit 1
fi

case "${MODE}" in
    status)
        ENDPOINT="/status/${STATUS_CODE}"
        CURL_FLAGS=(-s -D - -o /dev/null)
        ;;
    header)
        ENDPOINT="/gets"
        CURL_FLAGS=(-s)
        ;;
    delay)
        ENDPOINT="/delay/5"
        CURL_FLAGS=(-s -D - -o /dev/null)
        ;;
    post-valid)
        ENDPOINT="/post"
        CURL_FLAGS=(-s -X POST -H "Content-Type: application/json" -d '{"name":"test","value":"example"}')
        ;;
    post-invalid)
        # This should FAIL validation - wrong content-type for /redirect-to endpoint
        # Spec requires: multipart/form-data
        ENDPOINT="/redirect-to"
        CURL_FLAGS=(-s -D - -X POST -H "Content-Type: application/x-www-form-urlencoded" -d 'url=http://httpbin.org&status_code=302')
        ;;
    post-invalid-schema)
        # This should FAIL validation - wrong HTTP method on /delete endpoint
        # DELETE endpoint only accepts DELETE, not POST
        ENDPOINT="/delete"
        CURL_FLAGS=(-s -D - -X POST -H "Content-Type: application/json" -d '{}')
        ;;
    post-valid-redirect)
        # This should PASS validation - GET on /get endpoint (basic valid request)
        ENDPOINT="/get"
        CURL_FLAGS=(-s -D -)
        ;;
    put-invalid)
        # This should FAIL validation - /get endpoint doesn't accept PUT method
        ENDPOINT="/get"
        CURL_FLAGS=(-s -D - -X PUT)
        ;;
    delete-invalid)
        # This should FAIL validation - /post endpoint doesn't accept DELETE method
        ENDPOINT="/post"
        CURL_FLAGS=(-s -D - -X DELETE)
        ;;
    *)
        echo "Usage: $0 [status|header|delay|post-valid|post-invalid|post-missing-field|post-invalid-schema|put-invalid|delete-invalid]"
        echo ""
        echo "Test validation failures:"
        echo "  post-invalid         - Wrong content-type for /redirect-to endpoint"
        echo "  post-invalid-schema  - Wrong HTTP method (POST on /delete endpoint)"
        echo "  put-invalid          - PUT method not allowed on /get"
        echo "  delete-invalid       - DELETE method not allowed on /post"
        echo ""
        echo "Test validation success:"
        echo "  post-valid           - Correct POST to /post endpoint with JSON"
        echo "  post-valid-redirect  - Valid GET to /get endpoint"
        exit 1
        ;;
esac

for ((i=1; i<=REQUEST_COUNT; i++)); do
    echo -e "\nRequest ==> [${i}], endpoint=${ENDPOINT}"
    # Debug: show the actual curl command
    echo "[DEBUG] curl ${CURL_FLAGS[@]} -H \"Host: httpbin.local\" \"http://${INGRESS_IP}${ENDPOINT}\""
    curl "${CURL_FLAGS[@]}" -H "Host: httpbin.local" "http://${INGRESS_IP}${ENDPOINT}"
    sleep 1
done

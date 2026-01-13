FROM alpine:latest
# Install CA certs just in case your validator needs to call external APIs
RUN apk --no-cache add ca-certificates
COPY openapi-validator /usr/local/bin/openapi-validator
RUN chmod +x /usr/local/bin/openapi-validator
ENTRYPOINT ["/usr/local/bin/openapi-validator"]
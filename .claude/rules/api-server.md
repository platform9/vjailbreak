---
paths:
  - "pkg/vpwned/**/*.go"
---

# API Server Development Rules

Rules for developing the vJailbreak REST API server (vpwned) for cluster conversion operations.

## External Documentation

**ALWAYS consult these resources when working on API server code:**
- **OpenStack API Documentation**: https://docs.openstack.org/ - Target cloud platform APIs
- **Platform9 PCD Documentation**: https://docs.platform9.com/ - PCD-specific features and deployment
- **OpenAPI/Swagger**: https://swagger.io/specification/ - API specification standards

## API Design Principles

### RESTful Conventions
- Use standard HTTP methods (GET, POST, PUT, PATCH, DELETE)
- Return appropriate HTTP status codes
- Use JSON for request/response bodies
- Follow REST naming conventions for endpoints

### Endpoint Structure
- Use plural nouns for resources (e.g., `/migrations`, `/clusters`)
- Use path parameters for resource IDs (e.g., `/migrations/{id}`)
- Use query parameters for filtering and pagination
- Version APIs if breaking changes are needed

### Error Handling
- Return consistent error response format
- Include error codes and descriptive messages
- Log errors with appropriate context
- Use appropriate HTTP status codes (400, 404, 500, etc.)

## OpenStack Integration

### API Client Usage
- Use gophercloud for OpenStack API interactions
- Reference OpenStack API docs for endpoint specifications
- Handle authentication and token refresh properly
- Implement retry logic for transient failures

### PCD-Specific Features
- Consult Platform9 PCD documentation for PCD-specific APIs
- Handle PCD extensions to standard OpenStack APIs
- Test against actual PCD deployment
- Document any PCD-specific behavior

### Common Operations
- Image upload and management
- Flavor selection and validation
- Network configuration
- Volume type mapping
- Security group management

## OpenAPI/Swagger Generation

### API Documentation
- Use OpenAPI annotations in Go code
- Generate Swagger documentation automatically
- Keep API docs in sync with implementation
- Include request/response examples

### Generation Commands
```bash
# Generate OpenAPI specs
cd pkg/vpwned
make generate-openapi

# Or from repo root
./scripts/generate_all_openapi.sh
```

### Swagger UI
- Serve Swagger UI for API exploration
- Update swagger-ui-template when needed
- Test API endpoints via Swagger UI

## Cluster Conversion Logic

### Conversion Workflow
- Validate source cluster configuration
- Map VMware resources to OpenStack equivalents
- Handle network and storage mappings
- Orchestrate VM migrations
- Track conversion progress

### Resource Mapping
- Network mapping: VMware portgroups → OpenStack networks
- Storage mapping: VMware datastores → OpenStack volume types
- Compute mapping: VMware resource pools → OpenStack flavors
- Validate mappings before starting conversion

## Module Management

- This is an independent Go module at `pkg/vpwned/`
- Run `go mod tidy` from `pkg/vpwned/` directory
- Cross-module imports use full module path

## Build Process

### Building API Server
```bash
# From repo root
make build-vpwned
```

### Docker Image
- Image includes API server binary and dependencies
- Exposes REST API endpoints
- Configured via environment variables

## Testing

### Unit Tests
- Test API handlers with mock dependencies
- Test OpenStack client integration
- Test request validation and error handling
- Mock external API calls

### Integration Tests
- Test against real OpenStack/PCD deployment
- Verify API contract compliance
- Test error scenarios and edge cases
- Validate OpenAPI spec accuracy

## Authentication & Authorization

### API Authentication
- Implement token-based authentication
- Validate tokens on protected endpoints
- Handle token expiration and refresh
- Secure sensitive endpoints

### OpenStack Credentials
- Store credentials securely
- Use Kubernetes secrets for credential management
- Support credential rotation
- Validate credentials before use

## Configuration

### Environment Variables
- Use environment variables for configuration
- Document all required and optional variables
- Provide sensible defaults
- Validate configuration on startup

### Runtime Configuration
- Support dynamic configuration updates where possible
- Reload configuration without restart when feasible
- Log configuration changes

## Logging & Monitoring

### Structured Logging
- Use structured logging (JSON format)
- Include request IDs for tracing
- Log API requests and responses (sanitize sensitive data)
- Log OpenStack API interactions

### Metrics
- Expose Prometheus metrics
- Track API request counts and latencies
- Monitor OpenStack API call success/failure rates
- Track conversion progress metrics

## Common Patterns

### API Handler Pattern
```go
func (s *Server) HandleResource(w http.ResponseWriter, r *http.Request) {
    // Parse request
    // Validate input
    // Call business logic
    // Return response
}
```

### Error Response Format
```go
type ErrorResponse struct {
    Error   string `json:"error"`
    Code    string `json:"code"`
    Details string `json:"details,omitempty"`
}
```

## Debugging

### API Testing
- Use curl or Postman for manual API testing
- Check API server logs for errors
- Verify OpenStack API connectivity
- Test with various input scenarios

### Common Issues
- **OpenStack auth failures**: Check credentials and endpoint URLs
- **PCD-specific errors**: Consult PCD documentation
- **Network connectivity**: Verify API server can reach OpenStack endpoints
- **Swagger generation**: Ensure annotations are correct

### Logging
```bash
# Check API server logs
kubectl -n vjailbreak logs -l app=vpwned -f
```

## Security Best Practices

- Validate all user input
- Sanitize data before logging
- Use HTTPS for API endpoints
- Implement rate limiting
- Protect against common vulnerabilities (SQL injection, XSS, etc.)
- Keep dependencies updated

#!/bin/bash
set -e

# Builds an OpenAPI document for one release tag from that tag's CRDs.

VERSION="${1:-v0.0.0}"
PROJECT_ROOT="${2:-$(git rev-parse --show-toplevel)}"
CRD_BUNDLE="${3:-$PROJECT_ROOT/deploy/00crds.yaml}"

echo "PROJECT_ROOT: $PROJECT_ROOT"
echo "CRD_BUNDLE:   $CRD_BUNDLE"

if [ ! -s "$CRD_BUNDLE" ]; then
  echo "No CRD bundle at $CRD_BUNDLE"
  exit 1
fi

SWAGGER_OUT_DIR="$PROJECT_ROOT/docs/swagger-ui/$VERSION"
mkdir -p "$SWAGGER_OUT_DIR"
OUTPUT_OPENAPI="$SWAGGER_OUT_DIR/openapi.yaml"

echo "Using version: $VERSION"
echo "Cleaning previous OpenAPI file..."
rm -f "$OUTPUT_OPENAPI"
echo "Building OpenAPI document..."
cat > "$OUTPUT_OPENAPI" <<EOF
openapi: 3.0.0
info:
  title: vJailbreak API's
  version: $VERSION
paths:
EOF

# 00crds.yaml is an installer bundle: 24 CRDs among ~80 documents (ClusterRole,
# Service, Ingress and so on). Without this filter the loop would read
# .spec.names.plural off objects that have no such field and emit null paths.
/usr/local/bin/yq -r '
  select(.kind == "CustomResourceDefinition")
  | [.spec.names.kind, .spec.names.plural, .spec.group] | @tsv
' "$CRD_BUNDLE" |
while IFS=$'\t' read -r KIND PLURAL GROUP; do
  [ -z "$PLURAL" ] && continue
  KIND="${KIND//-/}"

  echo "  Adding REST paths for: $PLURAL"

  cat >> "$OUTPUT_OPENAPI" <<EOF
  /apis/$GROUP/v1alpha1/namespaces/{namespace}/$PLURAL:
    get:
      summary: List $KIND
      operationId: list$KIND
      responses:
        '200':
          description: Successful response
    post:
      summary: Create $KIND
      operationId: create$KIND
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
      responses:
        '201':
          description: Created
  /apis/$GROUP/v1alpha1/namespaces/{namespace}/$PLURAL/{name}:
    get:
      summary: Get $KIND
      operationId: get$KIND
      responses:
        '200':
          description: Successful response
    put:
      summary: Replace $KIND
      operationId: replace$KIND
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
      responses:
        '200':
          description: Updated
    delete:
      summary: Delete $KIND
      operationId: delete$KIND
      responses:
        '200':
          description: Deleted
EOF
done

echo "OpenAPI written to $OUTPUT_OPENAPI"

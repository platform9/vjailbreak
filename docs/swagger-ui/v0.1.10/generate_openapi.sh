#!/bin/bash
set -e
# === Config ===
CRD_DIR="../../../k8s/migration/config/crd"
CRD_BASES="$CRD_DIR/bases"
OUTPUT_OPENAPI="./openapi.yaml"

echo "Cleaning previous OpenAPI file..."
rm -f "$OUTPUT_OPENAPI"
echo "Building OpenAPI document..."
cat > "$OUTPUT_OPENAPI" <<EOF
openapi: 3.0.0
info:
  title: vJailbreak API's
  version: v0.1.10
paths:
EOF

for file in "$CRD_BASES"/*.yaml; do
  PLURAL=$(yq '.spec.names.plural' "$file")
  KIND=$(yq '.spec.names.kind' "$file" | sed 's/-//g')
  GROUP=$(yq '.spec.group' "$file")

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
              \$ref: "#/components/schemas/$PLURAL"
      responses:
        '201':
          description: Created

  /apis/$GROUP/v1alpha1/namespaces/{namespace}/$PLURAL/{name}:
    get:
      summary: Get $KIND
      operationId: get$KIND
      responses:
        '200':
          description: Found
    delete:
      summary: Delete $KIND
      operationId: delete$KIND
      responses:
        '204':
          description: Deleted

EOF
done

# schemas
echo "components:" >> "$OUTPUT_OPENAPI"
echo "  schemas:" >> "$OUTPUT_OPENAPI"

for file in "$CRD_BASES"/*.yaml; do
  NAME=$(yq '.metadata.name' "$file" | cut -d'.' -f1 | sed 's/-//g')
  SCHEMA=$(yq -o=json '.spec.versions[0].schema.openAPIV3Schema' "$file")

  if [ -z "$SCHEMA" ] || [ "$SCHEMA" == "null" ]; then
    echo "⚠️  Skipping $file (no schema)"
    continue
  fi

  echo "  📦 Adding schema: $NAME"
  echo "    $NAME:" >> "$OUTPUT_OPENAPI"
  echo "$SCHEMA" | yq -P | sed 's/^/      /' >> "$OUTPUT_OPENAPI"
done

echo "OpenAPI written to $OUTPUT_OPENAPI"

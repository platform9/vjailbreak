#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR/.."
OUTPUT_FILE="$PROJECT_ROOT/docs/src/content/docs/guides/using_apis.mdx"

TAGS_ARRAY=$(git tag --sort=-creatordate | head -n 5 | jq -R . | jq -s .)

cat <<EOF > "$OUTPUT_FILE"
---
title: "API Reference by Version"
---
import ApiReference from '../../../components/ApiReference.astro';
<ApiReference versions={${TAGS_ARRAY}} />
EOF

for tag in $(git tag --sort=-creatordate | head -n 5); do
  echo "- [${tag}](/vjailbreak/swagger-ui/${tag}/)" >> $OUTPUT_FILE
done

echo "API reference page updated to use ApiReference component in $OUTPUT_FILE"
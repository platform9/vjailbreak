package utils

// novaMetadataMaxLength is the Nova API limit for instance metadata keys and values.
const novaMetadataMaxLength = 255

// BuildTargetMetadata merges the preserved source tag/attribute metadata with the
// user-entered custom metadata into the instance metadata applied to the target VM.
// Custom metadata keys win over colliding source-derived keys. Keys and values are
// truncated to the Nova 255-character limit.
func BuildTargetMetadata(sourceTagsMetadata, customMetadata map[string]string) map[string]string {
	if len(sourceTagsMetadata) == 0 && len(customMetadata) == 0 {
		return nil
	}
	metadata := make(map[string]string, len(sourceTagsMetadata)+len(customMetadata))
	for key, value := range sourceTagsMetadata {
		metadata[truncateMetadata(key)] = truncateMetadata(value)
	}
	for key, value := range customMetadata {
		metadata[truncateMetadata(key)] = truncateMetadata(value)
	}
	return metadata
}

func truncateMetadata(s string) string {
	if len(s) > novaMetadataMaxLength {
		return s[:novaMetadataMaxLength]
	}
	return s
}

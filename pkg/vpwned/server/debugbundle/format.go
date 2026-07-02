package debugbundle

import (
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

const sectionSeparator = "================================================================================\n"

// FormatYAMLBundle renders the collected resources as one text document with
// a FILE: header per object, matching the layout produced by the UI bundle
// (formatBundleYaml.ts). managedFields metadata is stripped from every
// object before rendering.
func FormatYAMLBundle(entries []BundleEntry, warnings []string) string {
	var out strings.Builder

	sorted := make([]BundleEntry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	for i, entry := range sorted {
		if i > 0 {
			out.WriteString("\n")
		}
		out.WriteString(sectionSeparator)
		out.WriteString("FILE: " + entry.Path + "\n")
		out.WriteString(sectionSeparator)
		out.WriteString(renderObjectYAML(entry.Object))
	}

	if len(warnings) > 0 {
		if out.Len() > 0 {
			out.WriteString("\n")
		}
		out.WriteString(sectionSeparator)
		out.WriteString("FILE: collection-warnings.txt\n")
		out.WriteString(sectionSeparator)
		out.WriteString(strings.Join(warnings, "\n"))
		out.WriteString("\n")
	}

	return out.String()
}

func renderObjectYAML(obj *unstructured.Unstructured) string {
	clean := obj.DeepCopy()
	unstructured.RemoveNestedField(clean.Object, "metadata", "managedFields")
	data, err := yaml.Marshal(clean.Object)
	if err != nil {
		return fmt.Sprintf("[failed to render %s/%s as YAML: %v]\n", obj.GetKind(), obj.GetName(), err)
	}
	return string(data)
}

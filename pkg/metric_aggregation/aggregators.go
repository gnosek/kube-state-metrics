package aggregation

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kube-state-metrics/pkg/metric"
	"strconv"
)

// Given a list of label names, return the corresponding
// label values for a particular metric.
// Ensures that we always return the right number
// of values, even if a label is missing
func GetLabelsByName(m *metric.Metric, names ...string) []string {
	var metricLabels []string

	if len(names) == 0 {
		return metricLabels
	}

	nameIndex := 0
	for i, label := range m.LabelKeys {
		if label == names[nameIndex] {
			metricLabels = append(metricLabels, m.LabelValues[i])
			nameIndex += 1
			if nameIndex >= len(names) {
				break
			}
		}
	}

	for len(metricLabels) < len(names) {
		metricLabels = append(metricLabels, "<none>")
	}

	return metricLabels
}

func ByLabels(names ...string) Aggregation {
	return Aggregation{
		LabelNames: names,
		Aggregate: func(obj interface{}, f metric.FamilyInterface) []AggregatedValue {
			var aggregated []AggregatedValue

			f.Inspect(func(f metric.Family) {
				for _, m := range f.Metrics {
					aggregated = append(aggregated, AggregatedValue{
						Value:       m.Value,
						LabelValues: GetLabelsByName(m, names...),
					})
				}
			})

			return aggregated
		},
	}
}

func ByNamespaceAndOwner(labels ...string) Aggregation {
	return Aggregation{
		LabelNames: append([]string{"namespace", "owner_kind", "owner_name", "owner_is_controller"}, labels...),
		Aggregate: func(obj interface{}, f metric.FamilyInterface) []AggregatedValue {
			metaAccessor := obj.(metav1.ObjectMetaAccessor)
			meta := metaAccessor.GetObjectMeta()
			ns := meta.GetNamespace()
			var aggregated []AggregatedValue
			var labelSets [][]string

			owners := meta.GetOwnerReferences()
			if len(owners) == 0 {
				labelSets = [][]string{{ns, "<none>", "<none>", "<none>"}}
			} else {
				labelSets = make([][]string, len(owners))

				for i, owner := range owners {
					if owner.Controller != nil {
						labelSets[i] = []string{ns, owner.Kind, owner.Name, strconv.FormatBool(*owner.Controller)}
					} else {
						labelSets[i] = []string{ns, owner.Kind, owner.Name, "false"}
					}
				}
			}

			f.Inspect(func(f metric.Family) {
				for _, m := range f.Metrics {
					for _, s := range labelSets {
						aggregated = append(aggregated, AggregatedValue{
							Value:       m.Value,
							LabelValues: append(s, GetLabelsByName(m, labels...)...),
						})
					}
				}
			})

			return aggregated
		},
	}
}

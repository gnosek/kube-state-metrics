/*
Copyright 2018 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package metricsstore

import (
	"fmt"
	aggregation "k8s.io/kube-state-metrics/pkg/metric_aggregation"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/kube-state-metrics/pkg/metric"
)

func TestObjectsSameNameDifferentNamespaces(t *testing.T) {
	serviceIDS := []string{"a", "b"}

	genFunc := func(obj interface{}) []metric.FamilyInterface {
		o, err := meta.Accessor(obj)
		if err != nil {
			t.Fatal(err)
		}

		metricFamily := metric.Family{
			Name: "kube_service_info",
			Metrics: []*metric.Metric{
				{
					LabelKeys:   []string{"uid"},
					LabelValues: []string{string(o.GetUID())},
					Value:       float64(1),
				},
			},
		}

		return []metric.FamilyInterface{&metricFamily}
	}

	aggregatedOnly := []bool{false}
	aggregations := make(map[int]*aggregation.AggregationSet)

	ms := NewMetricsStore([]string{"Information about service."}, genFunc, aggregatedOnly, aggregations, 0)

	for _, id := range serviceIDS {
		s := v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service",
				Namespace: id,
				UID:       types.UID(id),
			},
		}

		err := ms.Add(&s)
		if err != nil {
			t.Fatal(err)
		}
	}

	w := strings.Builder{}
	ms.WriteAll(&w)
	m := w.String()

	for _, id := range serviceIDS {
		if !strings.Contains(m, fmt.Sprintf("uid=\"%v\"", id)) {
			t.Fatalf("expected to find metric with uid %v", id)
		}
	}
}

func TestAggregations(t *testing.T) {
	serviceIDS := []string{"a", "b"}
	serviceTypes := []v1.ServiceType{v1.ServiceTypeClusterIP, v1.ServiceTypeNodePort}

	genFunc := func(obj interface{}) []metric.FamilyInterface {
		o, err := meta.Accessor(obj)
		if err != nil {
			t.Fatal(err)
		}
		svc := obj.(*v1.Service)

		metricFamily := metric.Family{
			Name: "kube_service_info",
			Metrics: []*metric.Metric{
				{
					LabelKeys:   []string{"uid", "type"},
					LabelValues: []string{string(o.GetUID()), string(svc.Spec.Type)},
					Value:       float64(1),
				},
			},
		}

		return []metric.FamilyInterface{&metricFamily}
	}

	aggregatedOnly := []bool{true}
	aggregations := make(map[int]*aggregation.AggregationSet)
	aggregations[0] = aggregation.NewAggregationSet(
		map[string]aggregation.Aggregation{
			"type": aggregation.ByLabels("type"),
		},
		"kube_service_info",
		"Information about service.",
		metric.Gauge,
	)

	ms := NewMetricsStore([]string{"Information about service."}, genFunc, aggregatedOnly, aggregations, 0)

	for _, id := range serviceIDS {
		for _, svcType := range serviceTypes {
			s := v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service",
					Namespace: id,
					UID:       types.UID(id + string(svcType)),
				},
				Spec: v1.ServiceSpec{
					Type: svcType,
				},
			}

			err := ms.Add(&s)
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	w := strings.Builder{}
	ms.WriteAll(&w)
	m := w.String()

	if strings.Contains(m, fmt.Sprintf("uid=")) {
		t.Errorf("did not expect to find the uid= label in aggregated metrics")
	}

	for _, svcType := range serviceTypes {
		if !strings.Contains(m, fmt.Sprintf("type=\"%s\"", svcType)) {
			t.Errorf("expected to find label type=\"%s\" in aggregated metrics", svcType)
		}
	}
}

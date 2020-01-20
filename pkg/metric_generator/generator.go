/*
Copyright 2019 The Kubernetes Authors All rights reserved.

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

package generator

import (
	aggregation "k8s.io/kube-state-metrics/pkg/metric_aggregation"
	"strings"

	"k8s.io/kube-state-metrics/pkg/metric"
)

// FamilyGenerator provides everything needed to generate a metric family with a
// Kubernetes object.
type FamilyGenerator struct {
	Name         string
	Help         string
	Type         metric.Type
	GenerateFunc func(obj interface{}) *metric.Family

	// A map of names -> aggregation functions. Each function returns an array
	// of labels and a set of arrays containing the corresponding values for
	//a particular metric (there may be more than one and in that case the metric
	// is aggregated into multiple aggregations, so the sum of aggregated metrics
	// won't equal the sum of raw metrics)
	AggregateBy map[string]aggregation.Aggregation
	// Set to true when the original family is filtered out but there are still
	// enabled aggregations of that family. This means we still need to collect
	// the original family but should not emit it in MetricStore.WriteAll()
	AggregationsOnly bool
}

// Generate calls the FamilyGenerator.GenerateFunc and gives the family its
// name. The reasoning behind injecting the name at such a late point in time is
// deduplication in the code, preventing typos made by developers as
// well as saving memory.
func (g *FamilyGenerator) Generate(obj interface{}) *metric.Family {
	family := g.GenerateFunc(obj)
	family.Name = g.Name
	family.Type = g.Type
	return family
}

func (g *FamilyGenerator) generateHeader() string {
	header := strings.Builder{}
	header.WriteString("# HELP ")
	header.WriteString(g.Name)
	header.WriteByte(' ')
	header.WriteString(g.Help)
	header.WriteByte('\n')
	header.WriteString("# TYPE ")
	header.WriteString(g.Name)
	header.WriteByte(' ')
	header.WriteString(string(g.Type))

	return header.String()
}

// Get aggregated metric names for all supported label aggregations
// of this family
func (g *FamilyGenerator) aggregatedMetricNames() map[string]string {
	if g.AggregateBy == nil {
		return nil
	}
	names := make(map[string]string)
	for name, _ := range g.AggregateBy {
		names[name] = g.Name + aggregation.AggregatedMetricNameSuffix(name)
	}

	return names
}

// Filter familyGenerator objects, taking aggregations into account
func (g *FamilyGenerator) filterAggregations(l whiteBlackLister) bool {
	if l.IsExcluded(g.Name) {
		// we don't want this family on its own but may still need it
		// for aggregations
		g.AggregationsOnly = true
	}

	// only pick the aggregations that match the filter
	for aggName, metricName := range g.aggregatedMetricNames() {
		if l.IsExcluded(metricName) {
			delete(g.AggregateBy, aggName)
		}
	}
	if len(g.AggregateBy) == 0 {
		g.AggregateBy = nil
	}

	// we want this family if:
	// - the raw family isn't filtered out
	// - *or* any of the aggregations isn't filtered out
	return !g.AggregationsOnly || g.AggregateBy != nil
}

// ExtractMetricFamilyHeaders takes in a slice of FamilyGenerator metrics and
// returns the extracted headers.
func ExtractMetricFamilyHeaders(families []FamilyGenerator) []string {
	headers := make([]string, len(families))

	for i, f := range families {
		headers[i] = f.generateHeader()
	}

	return headers
}

// ComposeMetricGenFuncs takes a slice of metric families and returns a function
// that composes their metric generation functions into a single one.
func ComposeMetricGenFuncs(familyGens []FamilyGenerator) func(obj interface{}) []metric.FamilyInterface {
	return func(obj interface{}) []metric.FamilyInterface {
		families := make([]metric.FamilyInterface, len(familyGens))

		for i, gen := range familyGens {
			families[i] = gen.Generate(obj)
		}

		return families
	}
}

type whiteBlackLister interface {
	IsIncluded(string) bool
	IsExcluded(string) bool
}

// FilterMetricFamilies takes a white- and a blacklist and a slice of metric
// families and returns a filtered slice.
func FilterMetricFamilies(l whiteBlackLister, families []FamilyGenerator) []FamilyGenerator {
	filtered := []FamilyGenerator{}

	for _, f := range families {
		if f.filterAggregations(l) {
			filtered = append(filtered, f)
		}
	}

	return filtered
}

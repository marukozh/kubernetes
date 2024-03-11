/*
Copyright 2015 The Kubernetes Authors.

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

package storage

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apiserver/pkg/features"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	utilversion "k8s.io/apiserver/pkg/util/version"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
)

func TestDisabledVersion(t *testing.T) {
	g1v1 := schema.GroupVersion{Group: "group1", Version: "version1"}
	g1v2 := schema.GroupVersion{Group: "group1", Version: "version2"}
	g2v1 := schema.GroupVersion{Group: "group2", Version: "version1"}

	config := NewResourceConfigIgnoreLifecycle()

	config.DisableVersions(g1v1)
	config.EnableVersions(g1v2, g2v1)

	if config.versionEnabled(g1v1) {
		t.Errorf("expected disabled for %v, from %v", g1v1, config)
	}
	if !config.versionEnabled(g1v2) {
		t.Errorf("expected enabled for %v, from %v", g1v1, config)
	}
	if !config.versionEnabled(g2v1) {
		t.Errorf("expected enabled for %v, from %v", g1v1, config)
	}
}

func TestDisabledResource(t *testing.T) {
	g1v1 := schema.GroupVersion{Group: "group1", Version: "version1"}
	g1v1rUnspecified := g1v1.WithResource("unspecified")
	g1v1rEnabled := g1v1.WithResource("enabled")
	g1v1rDisabled := g1v1.WithResource("disabled")
	g1v2 := schema.GroupVersion{Group: "group1", Version: "version2"}
	g1v2rUnspecified := g1v2.WithResource("unspecified")
	g1v2rEnabled := g1v2.WithResource("enabled")
	g1v2rDisabled := g1v2.WithResource("disabled")
	g2v1 := schema.GroupVersion{Group: "group2", Version: "version1"}
	g2v1rUnspecified := g2v1.WithResource("unspecified")
	g2v1rEnabled := g2v1.WithResource("enabled")
	g2v1rDisabled := g2v1.WithResource("disabled")

	config := NewResourceConfigIgnoreLifecycle()

	config.DisableVersions(g1v1)
	config.EnableVersions(g1v2, g2v1)

	config.EnableResources(g1v1rEnabled, g1v2rEnabled, g2v1rEnabled)
	config.DisableResources(g1v1rDisabled, g1v2rDisabled, g2v1rDisabled)

	// all resources not explicitly enabled under g1v1 are disabled because the group-version is disabled
	if config.ResourceEnabled(g1v1rUnspecified) {
		t.Errorf("expected disabled for %v, from %v", g1v1rUnspecified, config)
	}
	if !config.ResourceEnabled(g1v1rEnabled) {
		t.Errorf("expected enabled for %v, from %v", g1v1rEnabled, config)
	}
	if config.ResourceEnabled(g1v1rDisabled) {
		t.Errorf("expected disabled for %v, from %v", g1v1rDisabled, config)
	}
	if config.ResourceEnabled(g1v1rUnspecified) {
		t.Errorf("expected disabled for %v, from %v", g1v1rUnspecified, config)
	}

	// explicitly disabled resources in enabled group-versions are disabled
	if config.ResourceEnabled(g1v2rDisabled) {
		t.Errorf("expected disabled for %v, from %v", g1v2rDisabled, config)
	}
	if config.ResourceEnabled(g2v1rDisabled) {
		t.Errorf("expected disabled for %v, from %v", g2v1rDisabled, config)
	}

	// unspecified and explicitly enabled resources in enabled group-versions are enabled
	if !config.ResourceEnabled(g1v2rUnspecified) {
		t.Errorf("expected enabled for %v, from %v", g1v2rUnspecified, config)
	}
	if !config.ResourceEnabled(g1v2rEnabled) {
		t.Errorf("expected enabled for %v, from %v", g1v2rEnabled, config)
	}
	if !config.ResourceEnabled(g2v1rUnspecified) {
		t.Errorf("expected enabled for %v, from %v", g2v1rUnspecified, config)
	}
	if !config.ResourceEnabled(g2v1rEnabled) {
		t.Errorf("expected enabled for %v, from %v", g2v1rEnabled, config)
	}
}

func TestAnyVersionForGroupEnabled(t *testing.T) {
	tests := []struct {
		name      string
		creator   func() APIResourceConfigSource
		testGroup string

		expectedResult bool
	}{
		{
			name: "empty",
			creator: func() APIResourceConfigSource {
				return NewResourceConfigIgnoreLifecycle()
			},
			testGroup: "one",

			expectedResult: false,
		},
		{
			name: "present, but disabled",
			creator: func() APIResourceConfigSource {
				ret := NewResourceConfigIgnoreLifecycle()
				ret.DisableVersions(schema.GroupVersion{Group: "one", Version: "version1"})
				return ret
			},
			testGroup: "one",

			expectedResult: false,
		},
		{
			name: "present, and one version enabled",
			creator: func() APIResourceConfigSource {
				ret := NewResourceConfigIgnoreLifecycle()
				ret.DisableVersions(schema.GroupVersion{Group: "one", Version: "version1"})
				ret.EnableVersions(schema.GroupVersion{Group: "one", Version: "version2"})
				return ret
			},
			testGroup: "one",

			expectedResult: true,
		},
		{
			name: "present, and one resource enabled",
			creator: func() APIResourceConfigSource {
				ret := NewResourceConfigIgnoreLifecycle()
				ret.DisableVersions(schema.GroupVersion{Group: "one", Version: "version1"})
				ret.EnableResources(schema.GroupVersionResource{Group: "one", Version: "version2", Resource: "foo"})
				return ret
			},
			testGroup: "one",

			expectedResult: true,
		},
		{
			name: "present, and one resource under disabled version enabled",
			creator: func() APIResourceConfigSource {
				ret := NewResourceConfigIgnoreLifecycle()
				ret.DisableVersions(schema.GroupVersion{Group: "one", Version: "version1"})
				ret.EnableResources(schema.GroupVersionResource{Group: "one", Version: "version1", Resource: "foo"})
				return ret
			},
			testGroup: "one",

			expectedResult: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if e, a := tc.expectedResult, tc.creator().AnyResourceForGroupEnabled(tc.testGroup); e != a {
				t.Errorf("expected %v, got %v", e, a)
			}
		})
	}
}

func TestEnabledVersionWithEmulationVersionOff(t *testing.T) {
	defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.EmulationVersion, false)()
	g1v1 := schema.GroupVersion{Group: "group1", Version: "version1"}
	g1v2 := schema.GroupVersion{Group: "group1", Version: "version2"}
	g2v1 := schema.GroupVersion{Group: "group2", Version: "version1"}
	g2v2 := schema.GroupVersion{Group: "group2", Version: "version2"}
	g2v3 := schema.GroupVersion{Group: "group2", Version: "version3"}

	scheme := runtime.NewScheme()
	scheme.SetGroupVersionLifecycle(g1v2, schema.APILifecycle{
		IntroducedVersion: version.MajorMinor(1, 31),
	})
	scheme.SetGroupVersionLifecycle(g2v1, schema.APILifecycle{
		RemovedVersion: version.MajorMinor(1, 29),
	})
	scheme.SetGroupVersionLifecycle(g2v2, schema.APILifecycle{
		IntroducedVersion: version.MajorMinor(1, 28),
	})
	scheme.SetGroupVersionLifecycle(g2v3, schema.APILifecycle{
		RemovedVersion: version.MajorMinor(1, 30),
	})
	config := NewResourceConfig(scheme)

	config.DisableVersions(g1v1)
	config.EnableVersions(g1v2, g2v1, g2v2, g2v3)

	if config.versionEnabled(g1v1) {
		t.Errorf("expected disabled for %v, from %v", g1v1, config)
	}
	if !config.versionEnabled(g1v2) {
		t.Errorf("expected enabled for %v, from %v", g1v2, config)
	}
	if !config.versionEnabled(g2v1) {
		t.Errorf("expected enabled for %v, from %v", g2v1, config)
	}
	if !config.versionEnabled(g2v2) {
		t.Errorf("expected enabled for %v, from %v", g2v2, config)
	}
	if !config.versionEnabled(g2v3) {
		t.Errorf("expected enabled for %v, from %v", g2v3, config)
	}
}

func TestEnabledVersionWithEmulationVersion(t *testing.T) {
	defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.EmulationVersion, true)()
	g1v1 := schema.GroupVersion{Group: "group1", Version: "version1"}
	g1v2 := schema.GroupVersion{Group: "group1", Version: "version2"}
	g2v1 := schema.GroupVersion{Group: "group2", Version: "version1"}
	g2v2 := schema.GroupVersion{Group: "group2", Version: "version2"}
	g2v3 := schema.GroupVersion{Group: "group2", Version: "version3"}

	scheme := runtime.NewScheme()
	scheme.SetGroupVersionLifecycle(g1v2, schema.APILifecycle{
		IntroducedVersion: version.MajorMinor(1, 31),
	})
	scheme.SetGroupVersionLifecycle(g2v1, schema.APILifecycle{
		RemovedVersion: version.MajorMinor(1, 29),
	})
	scheme.SetGroupVersionLifecycle(g2v2, schema.APILifecycle{
		IntroducedVersion: version.MajorMinor(1, 28),
	})
	scheme.SetGroupVersionLifecycle(g2v3, schema.APILifecycle{
		RemovedVersion: version.MajorMinor(1, 30),
	})

	utilversion.Effective.Set(version.MustParse("v1.31.0"), version.MustParse("v1.30.2"), version.MustParse("v1.30.0"))
	config := NewResourceConfig(scheme)

	config.DisableVersions(g1v1)
	config.EnableVersions(g1v2, g2v1, g2v2, g2v3)

	if config.versionEnabled(g1v1) {
		t.Errorf("expected disabled for %v, from %v", g1v1, config)
	}
	if config.versionEnabled(g1v2) {
		t.Errorf("expected disabled for %v, from %v", g1v2, config)
	}
	if config.versionEnabled(g2v1) {
		t.Errorf("expected disabled for %v, from %v", g2v1, config)
	}
	if !config.versionEnabled(g2v2) {
		t.Errorf("expected enabled for %v, from %v", g2v2, config)
	}
	if !config.versionEnabled(g2v3) {
		t.Errorf("expected enabled for %v, from %v", g2v3, config)
	}
}

func TestApiAvailable(t *testing.T) {
	tests := []struct {
		name              string
		emulationVersion  *version.Version
		introducedVersion *version.Version
		removedVersion    *version.Version
		expectedResult    bool
	}{
		{
			name:              "unspecified emulation version",
			emulationVersion:  version.MajorMinor(0, 0),
			introducedVersion: version.MajorMinor(1, 27),
			removedVersion:    version.MajorMinor(1, 30),
			expectedResult:    true,
		},
		{
			name:              "emulation version less than introduced",
			emulationVersion:  version.MajorMinor(1, 26),
			introducedVersion: version.MajorMinor(1, 27),
			removedVersion:    version.MajorMinor(1, 30),
			expectedResult:    false,
		},
		{
			name:              "emulation version equal to introduced",
			emulationVersion:  version.MajorMinor(1, 27),
			introducedVersion: version.MajorMinor(1, 27),
			removedVersion:    version.MajorMinor(1, 30),
			expectedResult:    true,
		},
		{
			name:              "emulation version between introduced and removed",
			emulationVersion:  version.MajorMinor(1, 29),
			introducedVersion: version.MajorMinor(1, 27),
			removedVersion:    version.MajorMinor(1, 30),
			expectedResult:    true,
		},
		{
			name:              "emulation version equal to removed",
			emulationVersion:  version.MajorMinor(1, 30),
			introducedVersion: version.MajorMinor(1, 27),
			removedVersion:    version.MajorMinor(1, 30),
			expectedResult:    true,
		},
		{
			name:              "emulation version greater than removed",
			emulationVersion:  version.MajorMinor(1, 31),
			introducedVersion: version.MajorMinor(1, 27),
			removedVersion:    version.MajorMinor(1, 30),
			expectedResult:    false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.EmulationVersion, true)()
			config := ResourceConfig{emulationVersion: tc.emulationVersion}
			available, _ := config.apiAvailable(schema.APILifecycle{IntroducedVersion: tc.introducedVersion, RemovedVersion: tc.removedVersion})
			if tc.expectedResult != available {
				t.Errorf("expected %v, got %v", tc.expectedResult, available)
			}
		})
	}
}

type introducedInObj struct {
	major, minor int
}

func (r introducedInObj) GetObjectKind() schema.ObjectKind {
	panic("don't do this")
}
func (r introducedInObj) DeepCopyObject() runtime.Object {
	panic("don't do this either")
}
func (r introducedInObj) APILifecycleIntroduced() (major, minor int) {
	return r.major, r.minor
}

func TestEnabledResourceWithEmulationVersion(t *testing.T) {
	tests := []struct {
		name                   string
		groupVersionIntroduced *version.Version
		resourceIntroduced     *version.Version
		enableGroupVersion     bool
		enableResource         bool
		expectedResult         bool
	}{
		{
			name:                   "enable gv introduced before emulation version",
			groupVersionIntroduced: version.MajorMinor(1, 29),
			enableGroupVersion:     true,
			enableResource:         true,
			expectedResult:         true,
		},
		{
			name:                   "enable gv introduced after emulation version",
			groupVersionIntroduced: version.MajorMinor(1, 31),
			enableGroupVersion:     true,
			enableResource:         true,
			expectedResult:         false,
		},
		{
			name:                   "enable resource for disabled gv introduced before emulation version",
			groupVersionIntroduced: version.MajorMinor(1, 29),
			enableGroupVersion:     false,
			enableResource:         true,
			expectedResult:         true,
		},
		{
			name:                   "enable resource introduced before and gv introduced before emulation version",
			groupVersionIntroduced: version.MajorMinor(1, 29),
			resourceIntroduced:     version.MajorMinor(1, 28),
			enableGroupVersion:     true,
			enableResource:         true,
			expectedResult:         true,
		},
		{
			name:                   "enable resource introduced after and gv introduced before emulation version",
			groupVersionIntroduced: version.MajorMinor(1, 29),
			resourceIntroduced:     version.MajorMinor(1, 31),
			enableGroupVersion:     true,
			enableResource:         true,
			expectedResult:         false,
		},
		{
			name:                   "enable resource introduced before and gv introduced after emulation version",
			groupVersionIntroduced: version.MajorMinor(1, 31),
			resourceIntroduced:     version.MajorMinor(1, 29),
			enableGroupVersion:     true,
			enableResource:         true,
			expectedResult:         false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.EmulationVersion, true)()
			scheme := runtime.NewScheme()
			utilversion.Effective.Set(version.MustParse("v1.31.0"), version.MustParse("v1.30.2"), version.MustParse("v1.30.0"))
			config := NewResourceConfig(scheme)
			gv := schema.GroupVersion{Group: "group", Version: "version"}
			r := gv.WithResource("resource")
			if tc.groupVersionIntroduced != nil {
				scheme.SetGroupVersionLifecycle(gv, schema.APILifecycle{
					IntroducedVersion: tc.groupVersionIntroduced,
				})
			}
			if tc.resourceIntroduced != nil {
				obj := introducedInObj{int(tc.resourceIntroduced.Major()), int(tc.resourceIntroduced.Minor())}
				scheme.SetResourceLifecycle(r, obj)
			}
			if tc.enableGroupVersion {
				config.EnableVersions(gv)
			} else {
				config.DisableVersions(gv)
			}
			if tc.enableResource {
				config.EnableResources(r)
			} else {
				config.DisableResources(r)
			}
			if e, a := tc.expectedResult, config.ResourceEnabled(r); e != a {
				t.Errorf("expected %v, got %v", e, a)
			}
		})
	}
}

func TestStorageVersionEmulation(t *testing.T) {
	defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.EmulationVersion, true)()

	type versionInfo struct {
		version    string
		introduced *version.Version
		removed    *version.Version
	}

	testCases := []struct {
		name                    string
		emulationVersion        *version.Version
		minCompatibilityVersion *version.Version
		resources               []versionInfo
		groups                  []versionInfo
		expectedStorageVersion  string
		expectedError           string
	}{
		{
			name:                    "emulation version is less than introduced group version",
			emulationVersion:        version.MustParse("1.28.0"),
			minCompatibilityVersion: version.MustParse("1.28.0"),
			resources: []versionInfo{
				{"v1", version.MustParse("1.28.0"), nil},
			},
			groups: []versionInfo{
				{"v1", version.MustParse("1.29.0"), version.MustParse("1.30.0")},
			},
			expectedError: "resource not codable by both emulation version and min compatibility version",
		},
		{
			name:                    "emulation version is greater than removed group version",
			emulationVersion:        version.MustParse("1.30.0"),
			minCompatibilityVersion: version.MustParse("1.28.0"),
			resources: []versionInfo{
				{"v1", version.MustParse("1.28.0"), version.MustParse("1.30.0")},
			},
			groups: []versionInfo{
				{"v1", version.MustParse("1.29.0"), version.MustParse("1.30.0")},
			},
			expectedError: "resource not codable by both emulation version and min compatibility version",
		},
		{
			name:                    "emulation version is equal to introduced group version",
			emulationVersion:        version.MustParse("1.29.0"),
			minCompatibilityVersion: version.MustParse("1.29.0"),
			resources: []versionInfo{
				{"v1", version.MustParse("1.29.0"), nil},
			},
			groups: []versionInfo{
				{"v1", version.MustParse("1.29.0"), nil},
			},
			expectedStorageVersion: "v1",
		},
		{
			name:                    "groupVersions added but not exposed",
			emulationVersion:        version.MustParse("1.29.0"),
			minCompatibilityVersion: version.MustParse("1.28.0"),
			resources: []versionInfo{
				{"v1", version.MustParse("1.26.0"), nil},
				{"v2", version.MustParse("1.29.0"), nil},
			},
			groups: []versionInfo{
				{"v1", version.MustParse("1.26.0"), nil},
				{"v2", version.MustParse("1.29.0"), nil},
			},
			expectedStorageVersion: "v1",
		},
		{
			name:                    "GA after mincompatibility version",
			emulationVersion:        version.MustParse("1.30.0"),
			minCompatibilityVersion: version.MustParse("1.29.0"),
			resources: []versionInfo{
				{"v1alpha1", version.MustParse("1.26.0"), version.MustParse("1.28.0")},
				{"v1beta1", version.MustParse("1.27.0"), nil},
				{"v1", version.MustParse("1.30.0"), nil},
			},
			groups: []versionInfo{
				{"v1alpha1", version.MustParse("1.25.0"), version.MustParse("1.26.0")},
				{"v1beta1", version.MustParse("1.26.0"), nil},
				{"v1", version.MustParse("1.25.0"), nil},
			},
			expectedStorageVersion: "v1beta1",
		},
		{
			// real world hardcoded storage encoding that
			// should be able to be inferred when the feature is enabled
			// without manual specification
			name:                    "validatingadmissionpolicies_1.30",
			emulationVersion:        version.MustParse("1.30.0"),
			minCompatibilityVersion: version.MustParse("1.29.0"),
			resources: []versionInfo{
				{"v1alpha1", version.MustParse("1.26.0"), nil},
				{"v1beta1", version.MustParse("1.28.0"), nil},
			},
			groups:                 []versionInfo{},
			expectedStorageVersion: "v1beta1",
		},
		{
			// real world hardcoded storage encoding that
			// should be able to be inferred when the feature is enabled
			// without manual specification
			name:                    "ipaddresses-1.30",
			emulationVersion:        version.MustParse("1.30.0"),
			minCompatibilityVersion: version.MustParse("1.29.0"),
			resources: []versionInfo{
				{"v1alpha1", version.MustParse("1.27.0"), nil},
			},
			groups:                 []versionInfo{},
			expectedStorageVersion: "v1alpha1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			gr := schema.GroupResource{Group: "test-group.example.com", Resource: "myresources"}

			prioritizedVersions := []schema.GroupVersion{}
			for _, r := range tc.resources {
				scheme.AddKnownTypeWithName(schema.GroupVersionKind{
					Group:   gr.Group,
					Version: r.version,
					Kind:    "MyResource",
				}, &runtime.Unknown{})
				scheme.AddKnownTypeWithName(schema.GroupVersionKind{
					Group:   gr.Group,
					Version: r.version,
					Kind:    "MyResourceList",
				}, &runtime.Unknown{})
				obj := introducedInObj{int(r.introduced.Major()), int(r.introduced.Minor())}
				scheme.SetResourceLifecycle(gr.WithVersion(r.version), obj)
				prioritizedVersions = append(prioritizedVersions, schema.GroupVersion{
					Group:   gr.Group,
					Version: r.version,
				})
			}

			for _, g := range tc.groups {
				scheme.SetGroupVersionLifecycle(schema.GroupVersion{
					Group:   gr.Group,
					Version: g.version,
				}, schema.APILifecycle{
					IntroducedVersion: g.introduced,
					RemovedVersion:    g.removed,
				})
			}

			// reverse prioritizedVersions
			slices.Reverse(prioritizedVersions)
			require.NoError(t, scheme.SetVersionPriority(prioritizedVersions...))

			// Can't actually set emulation version used by the constructor,
			// (blocked from having feature enabled at the same time as low
			// emulation version)
			// so we just override it here
			resourceEncodingConfig := NewDefaultResourceEncodingConfig(scheme)
			resourceEncodingConfig.emulationVersion = version.MajorMinor(tc.emulationVersion.Major(), tc.emulationVersion.Minor())
			resourceEncodingConfig.minCompatibilityVersion = version.MajorMinor(tc.minCompatibilityVersion.Major(), tc.minCompatibilityVersion.Minor())

			storageVersion, err := resourceEncodingConfig.StorageEncodingFor(gr)

			if len(tc.expectedError) > 0 {
				require.ErrorContains(t, err, tc.expectedError, "expected error, got %v", storageVersion)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedStorageVersion, storageVersion.Version)
			}
		})
	}
}

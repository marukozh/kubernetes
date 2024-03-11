/*
Copyright 2016 The Kubernetes Authors.

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

package feature

import (
	"k8s.io/component-base/featuregate"
)

var (
	// DefaultMutableVersionedFeatureGate is DefaultMutableFeatureGate with access to settings specific to the emuation version.
	// Only top-level commands/options setup and the k8s.io/component-base/featuregate/testing package should make use of this.
	DefaultMutableVersionedFeatureGate featuregate.MutableVersionedFeatureGate = featuregate.NewFeatureGate()
	// DefaultVersionedFeatureGate is a shared global FeatureGate with emulation version.
	// Top-level commands/options setup that needs to modify this feature gate should use DefaultMutableVersionedFeatureGate.
	DefaultVersionedFeatureGate featuregate.VersionedFeatureGate = DefaultMutableVersionedFeatureGate
	// DefaultMutableFeatureGate is a mutable version of DefaultFeatureGate.
	// Only top-level commands/options setup and the k8s.io/component-base/featuregate/testing package should make use of this.
	// Tests that need to modify feature gates for the duration of their test should use:
	//   defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.<FeatureName>, <value>)()
	DefaultMutableFeatureGate featuregate.MutableFeatureGate = DefaultMutableVersionedFeatureGate

	// DefaultFeatureGate is a shared global FeatureGate.
	// Top-level commands/options setup that needs to modify this feature gate should use DefaultMutableFeatureGate.
	DefaultFeatureGate featuregate.FeatureGate = DefaultMutableFeatureGate
)

/*
Copyright 2025 The Kubernetes Authors.

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

package compatibility

import (
	"k8s.io/apimachinery/pkg/util/version"
	baseversion "k8s.io/component-base/version"
)

// minimumKubeEmulationVersion is the first release emulation version is introduced,
// so the emulation version cannot go lower than that.
var minimumKubeEmulationVersion *version.Version = version.MajorMinor(1, 31)

// DefaultBuildEffectiveVersion returns the MutableEffectiveVersion based on the
// current build information.
func DefaultBuildEffectiveVersion() baseversion.MutableEffectiveVersion {
	binaryVersion := defaultBuildBinaryVersion()
	if binaryVersion.Major() == 0 && binaryVersion.Minor() == 0 {
		return DefaultKubeEffectiveVersion()
	}
	effectiveVersion := baseversion.NewEffectiveVersion(binaryVersion)
	return withKubeEffectiveVersionFloors(effectiveVersion)
}

func withKubeEffectiveVersionFloors(effectiveVersion baseversion.MutableEffectiveVersion) baseversion.MutableEffectiveVersion {
	// // both emulationVersion and minCompatibilityVersion can be set to binaryVersion - 3
	versionFloor := effectiveVersion.BinaryVersion().WithPatch(0).SubtractMinor(3)
	if versionFloor.LessThan(minimumKubeEmulationVersion) {
		versionFloor = minimumKubeEmulationVersion
	}
	return effectiveVersion.WithEmulationVersionFloor(versionFloor).WithMinCompatibilityVersionFloor(versionFloor)
}

// DefaultKubeEffectiveVersion returns the MutableEffectiveVersion based on the
// latest K8s release.
func DefaultKubeEffectiveVersion() baseversion.MutableEffectiveVersion {
	binaryVersion := version.MustParse(baseversion.DefaultKubeBinaryVersion).WithInfo(baseversion.Get())
	effectiveVersion := baseversion.NewEffectiveVersion(binaryVersion)
	return withKubeEffectiveVersionFloors(effectiveVersion)
}

func defaultBuildBinaryVersion() *version.Version {
	verInfo := baseversion.Get()
	return version.MustParse(verInfo.String()).WithInfo(verInfo)
}

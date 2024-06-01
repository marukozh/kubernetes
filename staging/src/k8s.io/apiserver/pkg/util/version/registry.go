/*
Copyright 2024 The Kubernetes Authors.

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

package version

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/pflag"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/version"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/featuregate"
	"k8s.io/klog/v2"
)

// DefaultComponentGlobalsRegistry is the global var to store the effective versions and feature gates for all components for easy access.
// Example usage:
// // register the component effective version and feature gate first
// _, _ = utilversion.DefaultComponentGlobalsRegistry.ComponentGlobalsOrRegister(utilversion.DefaultKubeComponent, utilversion.DefaultKubeEffectiveVersion(), utilfeature.DefaultMutableFeatureGate)
// wardleEffectiveVersion := utilversion.NewEffectiveVersion("1.2")
// wardleFeatureGate := featuregate.NewFeatureGate()
// utilruntime.Must(utilversion.DefaultComponentGlobalsRegistry.Register(apiserver.WardleComponentName, wardleEffectiveVersion, wardleFeatureGate, false))
//
//	cmd := &cobra.Command{
//	 ...
//		// call DefaultComponentGlobalsRegistry.Set() in PersistentPreRunE
//		PersistentPreRunE: func(*cobra.Command, []string) error {
//			if err := utilversion.DefaultComponentGlobalsRegistry.Set(); err != nil {
//				return err
//			}
//	 ...
//		},
//		RunE: func(c *cobra.Command, args []string) error {
//			// call utilversion.DefaultComponentGlobalsRegistry.Validate() somewhere
//		},
//	}
//
// flags := cmd.Flags()
// // add flags
// utilversion.DefaultComponentGlobalsRegistry.AddFlags(flags)
var DefaultComponentGlobalsRegistry ComponentGlobalsRegistry = NewComponentGlobalsRegistry()

const (
	DefaultKubeComponent = "kube"

	klogLevel = 2
)

type VersionMapping func(from *version.Version) *version.Version

// ComponentGlobals stores the global variables for a component for easy access.
type ComponentGlobals struct {
	effectiveVersion MutableEffectiveVersion
	featureGate      featuregate.MutableVersionedFeatureGate
}

type ComponentGlobalsRegistry interface {
	// EffectiveVersionFor returns the EffectiveVersion registered under the component.
	// Returns nil if the component is not registered.
	EffectiveVersionFor(component string) EffectiveVersion
	// FeatureGateFor returns the FeatureGate registered under the component.
	// Returns nil if the component is not registered.
	FeatureGateFor(component string) featuregate.FeatureGate
	// Register registers the EffectiveVersion and FeatureGate for a component.
	// returns error if the component is already registered.
	Register(component string, effectiveVersion MutableEffectiveVersion, featureGate featuregate.MutableVersionedFeatureGate) error
	// ComponentGlobalsOrRegister would return the registered global variables for the component if it already exists in the registry.
	// Otherwise, the provided variables would be registered under the component, and the same variables would be returned.
	ComponentGlobalsOrRegister(component string, effectiveVersion MutableEffectiveVersion, featureGate featuregate.MutableVersionedFeatureGate) (MutableEffectiveVersion, featuregate.MutableVersionedFeatureGate)
	// AddFlags adds flags of "--emulated-version" and "--feature-gates"
	AddFlags(fs *pflag.FlagSet)
	// Set sets the flags for all global variables for all components registered.
	Set() error
	// Validate calls the Validate() function for all the global variables for all components registered.
	Validate() []error
	// Reset removes all stored ComponentGlobals, configurations, and version mappings.
	Reset()
	// SetEmulationVersionMapping sets the mapping from the emulation version of one component
	// to the emulation version of another component.
	// Once set, the emulation version of the toComponent will be determined by the emulation version of the fromComponent,
	// and cannot be set from cmd flags anymore.
	// For a given component, its emulation version can only depend on one other component, no multiple dependency is allowed.
	SetEmulationVersionMapping(fromComponent, toComponent string, f VersionMapping) error
}

type componentGlobalsRegistry struct {
	componentGlobals map[string]ComponentGlobals
	mutex            sync.RWMutex
	// map of component name to emulation version set from the flag.
	emulationVersionConfig cliflag.ConfigurationMap
	// map of component name to the list of feature gates set from the flag.
	featureGatesConfig map[string][]string
	// emulationVersionMapping contains the mapping from the emulation version of one component
	// to the emulation version of another component.
	emulationVersionMapping map[string]map[string]VersionMapping
	// componentsWithDependentEmulationVersion stores whether or not a component's EmulationVersion is dependent through mapping on another component.
	// For a given component, there can only be one mapping from another component.
	componentsWithDependentEmulationVersion map[string]bool
	// minCompatibilityVersionMapping contains the mapping from the min compatibility version of one component
	// to the min compatibility version of another component.
	minCompatibilityVersionMapping map[string]map[string]VersionMapping
	// componentsWithDependentMinCompatibilityVersion stores whether or not a component's MinCompatibilityVersion is dependent through mapping on another component
	// For a given component, there can only be one mapping from another component.
	componentsWithDependentMinCompatibilityVersion map[string]bool
}

func NewComponentGlobalsRegistry() *componentGlobalsRegistry {
	return &componentGlobalsRegistry{
		componentGlobals:                               make(map[string]ComponentGlobals),
		emulationVersionConfig:                         nil,
		featureGatesConfig:                             nil,
		emulationVersionMapping:                        make(map[string]map[string]VersionMapping),
		minCompatibilityVersionMapping:                 make(map[string]map[string]VersionMapping),
		componentsWithDependentEmulationVersion:        make(map[string]bool),
		componentsWithDependentMinCompatibilityVersion: make(map[string]bool),
	}
}

func (r *componentGlobalsRegistry) Reset() {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	r.componentGlobals = make(map[string]ComponentGlobals)
	r.emulationVersionConfig = nil
	r.featureGatesConfig = nil
	r.emulationVersionMapping = make(map[string]map[string]VersionMapping)
	r.minCompatibilityVersionMapping = make(map[string]map[string]VersionMapping)
	r.componentsWithDependentEmulationVersion = make(map[string]bool)
	r.componentsWithDependentMinCompatibilityVersion = make(map[string]bool)
}

func (r *componentGlobalsRegistry) EffectiveVersionFor(component string) EffectiveVersion {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	globals, ok := r.componentGlobals[component]
	if !ok {
		return nil
	}
	return globals.effectiveVersion
}

func (r *componentGlobalsRegistry) FeatureGateFor(component string) featuregate.FeatureGate {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	globals, ok := r.componentGlobals[component]
	if !ok {
		return nil
	}
	return globals.featureGate
}

func (r *componentGlobalsRegistry) unsafeRegister(component string, effectiveVersion MutableEffectiveVersion, featureGate featuregate.MutableVersionedFeatureGate) error {
	if _, ok := r.componentGlobals[component]; ok {
		return fmt.Errorf("component globals of %s already registered", component)
	}
	if featureGate != nil {
		if err := featureGate.SetEmulationVersion(effectiveVersion.EmulationVersion()); err != nil {
			return err
		}
	}
	c := ComponentGlobals{effectiveVersion: effectiveVersion, featureGate: featureGate}
	r.componentGlobals[component] = c
	return nil
}

func (r *componentGlobalsRegistry) Register(component string, effectiveVersion MutableEffectiveVersion, featureGate featuregate.MutableVersionedFeatureGate) error {
	if effectiveVersion == nil {
		return fmt.Errorf("cannot register nil effectiveVersion")
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.unsafeRegister(component, effectiveVersion, featureGate)
}

func (r *componentGlobalsRegistry) ComponentGlobalsOrRegister(component string, effectiveVersion MutableEffectiveVersion, featureGate featuregate.MutableVersionedFeatureGate) (MutableEffectiveVersion, featuregate.MutableVersionedFeatureGate) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	globals, ok := r.componentGlobals[component]
	if ok {
		return globals.effectiveVersion, globals.featureGate
	}
	utilruntime.Must(r.unsafeRegister(component, effectiveVersion, featureGate))
	return effectiveVersion, featureGate
}

func (r *componentGlobalsRegistry) unsafeKnownFeatures() []string {
	var known []string
	for component, globals := range r.componentGlobals {
		if globals.featureGate == nil {
			continue
		}
		for _, f := range globals.featureGate.KnownFeatures() {
			known = append(known, component+":"+f)
		}
	}
	sort.Strings(known)
	return known
}

func (r *componentGlobalsRegistry) unsafeVersionFlagOptions(isEmulation bool) []string {
	var vs []string
	for component, globals := range r.componentGlobals {
		binaryVer := globals.effectiveVersion.BinaryVersion()
		if isEmulation {
			if r.componentsWithDependentEmulationVersion[component] {
				continue
			}
			// emulated version could be between binaryMajor.{binaryMinor} and binaryMajor.{binaryMinor}
			// TODO: change to binaryMajor.{binaryMinor-1} and binaryMajor.{binaryMinor} in 1.32
			vs = append(vs, fmt.Sprintf("%s=%s..%s (default=%s)", component,
				binaryVer.SubtractMinor(0).String(), binaryVer.String(), globals.effectiveVersion.EmulationVersion().String()))
		} else {
			if r.componentsWithDependentMinCompatibilityVersion[component] {
				continue
			}
			// min compatibility version could be between binaryMajor.{binaryMinor-1} and binaryMajor.{binaryMinor}
			vs = append(vs, fmt.Sprintf("%s=%s..%s (default=%s)", component,
				binaryVer.SubtractMinor(1).String(), binaryVer.String(), globals.effectiveVersion.MinCompatibilityVersion().String()))
		}
	}
	sort.Strings(vs)
	return vs
}

func (r *componentGlobalsRegistry) AddFlags(fs *pflag.FlagSet) {
	if r == nil {
		return
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for _, globals := range r.componentGlobals {
		if globals.featureGate != nil {
			globals.featureGate.Close()
		}
	}
	if r.emulationVersionConfig != nil || r.featureGatesConfig != nil {
		klog.Warning("calling componentGlobalsRegistry.AddFlags more than once, the registry will be set by the latest flags")
	}
	r.emulationVersionConfig = make(cliflag.ConfigurationMap)
	r.featureGatesConfig = make(map[string][]string)

	fs.Var(&r.emulationVersionConfig, "emulated-version", ""+
		"The versions different components emulate their capabilities (APIs, features, ...) of.\n"+
		"If set, the component will emulate the behavior of this version instead of the underlying binary version.\n"+
		"Version format could only be major.minor, for example: '--emulated-version=wardle=1.2,kube=1.31'. Options are:\n"+strings.Join(r.unsafeVersionFlagOptions(true), "\n"))

	fs.Var(cliflag.NewColonSeparatedMultimapStringStringAllowDefaultEmptyKey(&r.featureGatesConfig), "feature-gates", "Comma-separated list of component:key=value pairs that describe feature gates for alpha/experimental features of different components.\n"+
		"If the component is not specified, defaults to \"kube\". This flag can be repeatedly invoked. For example: --feature-gates 'wardle:featureA=true,wardle:featureB=false' --feature-gates 'kube:featureC=true'"+
		"Options are:\n"+strings.Join(r.unsafeKnownFeatures(), "\n"))
}

type componentVersion struct {
	component string
	ver       *version.Version
}

// getFullVersionConfig expands the given version config with version registered version mapping,
// and returns the map of component to Version.
func (r *componentGlobalsRegistry) getFullVersionConfig(
	config cliflag.ConfigurationMap, versionMapping map[string]map[string]VersionMapping) (map[string]*version.Version, error) {
	result := map[string]*version.Version{}
	setQueue := []componentVersion{}
	for comp, verStr := range config {
		if _, ok := r.componentGlobals[comp]; !ok {
			return result, fmt.Errorf("component not registered: %s", comp)
		}
		ver, err := version.Parse(verStr)
		if err != nil {
			return result, err
		}
		if ver.Patch() != 0 {
			return result, fmt.Errorf("patch version not allowed, got: %s", verStr)
		}
		klog.V(klogLevel).Infof("setting version %s=%s", comp, ver.String())
		setQueue = append(setQueue, componentVersion{comp, ver})
	}
	for len(setQueue) > 0 {
		cv := setQueue[0]
		if _, visited := result[cv.component]; visited {
			return result, fmt.Errorf("setting version of %s more than once, probably version mapping loop", cv.component)
		}
		setQueue = setQueue[1:]
		result[cv.component] = cv.ver
		for toComp, f := range versionMapping[cv.component] {
			toVer := f(cv.ver)
			if toVer == nil {
				return result, fmt.Errorf("got nil version from mapping of %s=%s to component:%s", cv.component, cv.ver.String(), toComp)
			}
			klog.V(klogLevel).Infof("setting version %s=%s from version mapping of %s=%s", toComp, toVer.String(), cv.component, cv.ver.String())
			setQueue = append(setQueue, componentVersion{toComp, toVer})
		}
	}
	return result, nil
}

func (r *componentGlobalsRegistry) Set() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for comp := range r.emulationVersionConfig {
		// only components without any dependencies can be set from the flag.
		if r.componentsWithDependentEmulationVersion[comp] {
			return fmt.Errorf("EmulationVersion of %s is set by mapping, cannot set it by flag", comp)
		}
	}
	if emulationVersions, err := r.getFullVersionConfig(r.emulationVersionConfig, r.emulationVersionMapping); err != nil {
		return err
	} else {
		for comp, ver := range emulationVersions {
			r.componentGlobals[comp].effectiveVersion.SetEmulationVersion(ver)
		}
	}
	// Set feature gate emulation version before setting feature gate flag values.
	for comp, globals := range r.componentGlobals {
		if globals.featureGate == nil {
			continue
		}
		klog.V(klogLevel).Infof("setting %s:feature gate emulation version to %s", comp, globals.effectiveVersion.EmulationVersion().String())
		if err := globals.featureGate.SetEmulationVersion(globals.effectiveVersion.EmulationVersion()); err != nil {
			return err
		}
	}
	for comp, fg := range r.featureGatesConfig {
		if comp == "" {
			if _, ok := r.featureGatesConfig[DefaultKubeComponent]; ok {
				return fmt.Errorf("set kube feature gates with default empty prefix or kube: prefix consistently, do not mix use")
			}
			comp = DefaultKubeComponent
		}
		if _, ok := r.componentGlobals[comp]; !ok {
			return fmt.Errorf("component not registered: %s", comp)
		}
		featureGate := r.componentGlobals[comp].featureGate
		if featureGate == nil {
			return fmt.Errorf("component featureGate not registered: %s", comp)
		}
		flagVal := strings.Join(fg, ",")
		klog.V(klogLevel).Infof("setting %s:feature-gates=%s", comp, flagVal)
		if err := featureGate.Set(flagVal); err != nil {
			return err
		}
	}
	return nil
}

func (r *componentGlobalsRegistry) Validate() []error {
	var errs []error
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for _, globals := range r.componentGlobals {
		errs = append(errs, globals.effectiveVersion.Validate()...)
		if globals.featureGate != nil {
			errs = append(errs, globals.featureGate.Validate()...)
		}
	}
	return errs
}

func (r *componentGlobalsRegistry) SetEmulationVersionMapping(fromComponent, toComponent string, f VersionMapping) error {
	if f == nil {
		return nil
	}
	klog.V(klogLevel).Infof("setting EmulationVersion mapping from %s to %s", fromComponent, toComponent)
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if _, ok := r.componentGlobals[fromComponent]; !ok {
		return fmt.Errorf("component not registered: %s", fromComponent)
	}
	if _, ok := r.componentGlobals[toComponent]; !ok {
		return fmt.Errorf("component not registered: %s", toComponent)
	}
	// check multiple dependency
	if r.componentsWithDependentEmulationVersion[toComponent] {
		return fmt.Errorf("mapping of %s already exists from another component", toComponent)
	}
	r.componentsWithDependentEmulationVersion[toComponent] = true

	if _, ok := r.emulationVersionMapping[fromComponent]; !ok {
		r.emulationVersionMapping[fromComponent] = make(map[string]VersionMapping)
	}
	versionMapping := r.emulationVersionMapping[fromComponent]
	if _, ok := versionMapping[toComponent]; ok {
		return fmt.Errorf("EmulationVersion from %s to %s already exists", fromComponent, toComponent)
	}
	versionMapping[toComponent] = f
	klog.V(klogLevel).Infof("setting the default EmulationVersion of %s based on mapping from the default EmulationVersion of %s", fromComponent, toComponent)
	defaultFromVersion := r.componentGlobals[fromComponent].effectiveVersion.EmulationVersion().String()
	emulationVersions, err := r.getFullVersionConfig(
		cliflag.ConfigurationMap{fromComponent: defaultFromVersion}, r.emulationVersionMapping)
	if err != nil {
		return err
	}
	for comp, ver := range emulationVersions {
		r.componentGlobals[comp].effectiveVersion.SetEmulationVersion(ver)
	}
	return nil
}

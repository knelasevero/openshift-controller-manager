package controller

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

type okBuildLister struct{}

func (okc *okBuildLister) List(namespace string, opts metav1.ListOptions) (*buildapi.BuildList, error) {
	return &buildapi.BuildList{Items: []buildapi.Build{}}, nil
}

type okBuildDeleter struct{}

func (okc *okBuildDeleter) DeleteBuild(*buildapi.Build) error {
	return nil
}

type okBuildConfigGetter struct {
	BuildConfig *buildapi.BuildConfig
}

func (okc *okBuildConfigGetter) Get(namespace, name string, options metav1.GetOptions) (*buildapi.BuildConfig, error) {
	if okc.BuildConfig != nil {
		return okc.BuildConfig, nil
	} else {
		return &buildapi.BuildConfig{}, nil
	}
}

func TestHandleBuildConfig(t *testing.T) {
	tests := []struct {
		name              string
		bc                *buildapi.BuildConfig
		expectBuild       bool
		instantiatorError bool
		expectErr         bool
	}{
		{
			name:        "build config with no config change trigger",
			bc:          baseBuildConfig(),
			expectBuild: false,
		},
		{
			name:        "build config with non-zero last version",
			bc:          buildConfigWithNonZeroLastVersion(),
			expectBuild: false,
		},
		{
			name:        "build config with config change trigger",
			bc:          buildConfigWithConfigChangeTrigger(),
			expectBuild: true,
		},
		{
			name:              "instantiator error",
			bc:                buildConfigWithConfigChangeTrigger(),
			instantiatorError: true,
			expectErr:         true,
		},
	}

	for _, tc := range tests {
		instantiator := &testInstantiator{
			err: tc.instantiatorError,
		}
		controller := &BuildConfigController{
			BuildConfigInstantiator: instantiator,
			BuildLister:             &okBuildLister{},
			BuildDeleter:            &okBuildDeleter{},
			BuildConfigGetter:       &okBuildConfigGetter{BuildConfig: tc.bc},
			Recorder:                &record.FakeRecorder{},
		}
		err := controller.HandleBuildConfig(tc.bc)
		if err != nil {
			if !tc.expectErr {
				t.Errorf("%s: unexpected error: %v", tc.name, err)
			}
			continue
		}
		if err == nil && tc.expectErr {
			t.Errorf("%s: expected error, but got none", tc.name)
			continue
		}
		if tc.expectBuild && len(instantiator.requestName) == 0 {
			t.Errorf("%s: expected a build to be started.", tc.name)
		}
		if !tc.expectBuild && len(instantiator.requestName) > 0 {
			t.Errorf("%s: did not expect a build to be started.", tc.name)
		}
	}

}

type testInstantiator struct {
	requestName string
	err         bool
}

func (i *testInstantiator) Instantiate(namespace string, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	i.requestName = request.Name
	if i.err {
		return nil, fmt.Errorf("error")
	}
	return &buildapi.Build{}, nil
}

func baseBuildConfig() *buildapi.BuildConfig {
	bc := &buildapi.BuildConfig{}
	bc.Name = "testBuildConfig"
	bc.Spec.Strategy.SourceStrategy = &buildapi.SourceBuildStrategy{}
	bc.Spec.Strategy.SourceStrategy.From.Name = "builderimage:latest"
	bc.Spec.Strategy.SourceStrategy.From.Kind = "ImageStreamTag"
	return bc
}

func buildConfigWithConfigChangeTrigger() *buildapi.BuildConfig {
	bc := baseBuildConfig()
	configChangeTrigger := buildapi.BuildTriggerPolicy{}
	configChangeTrigger.Type = buildapi.ConfigChangeBuildTriggerType
	bc.Spec.Triggers = append(bc.Spec.Triggers, configChangeTrigger)
	return bc
}

func buildConfigWithNonZeroLastVersion() *buildapi.BuildConfig {
	bc := buildConfigWithConfigChangeTrigger()
	bc.Status.LastVersion = 1
	return bc
}

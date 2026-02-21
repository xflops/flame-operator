/*
Copyright 2024 The Flame Authors.

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

package controller

import (
	"testing"

	flamev1alpha1 "github.com/xflops/flame-operator/api/v1alpha1"
)

func TestValidateSpec(t *testing.T) {
	r := &FlameClusterReconciler{}

	tests := []struct {
		name    string
		cluster *flamev1alpha1.FlameCluster
		wantErr bool
	}{
		{
			name: "valid spec",
			cluster: &flamev1alpha1.FlameCluster{
				Spec: flamev1alpha1.FlameClusterSpec{
					SessionManager: flamev1alpha1.SessionManagerSpec{
						Image: "valid-image",
					},
					ExecutorManager: flamev1alpha1.ExecutorManagerSpec{
						Image: "valid-image",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing session manager image",
			cluster: &flamev1alpha1.FlameCluster{
				Spec: flamev1alpha1.FlameClusterSpec{
					SessionManager: flamev1alpha1.SessionManagerSpec{
						Image: "",
					},
					ExecutorManager: flamev1alpha1.ExecutorManagerSpec{
						Image: "valid-image",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing executor manager image",
			cluster: &flamev1alpha1.FlameCluster{
				Spec: flamev1alpha1.FlameClusterSpec{
					SessionManager: flamev1alpha1.SessionManagerSpec{
						Image: "valid-image",
					},
					ExecutorManager: flamev1alpha1.ExecutorManagerSpec{
						Image: "",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := r.validateSpec(tt.cluster); (err != nil) != tt.wantErr {
				t.Errorf("validateSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

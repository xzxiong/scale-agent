// Copyright 2024 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"reflect"
	"testing"

	apires "k8s.io/apimachinery/pkg/api/resource"
)

func TestParseResourceRange(t *testing.T) {
	type args struct {
		jsonStr string
	}
	tests := []struct {
		name    string
		args    args
		want    *ResourceRange
		wantErr bool
	}{
		{
			name: "normal",
			args: args{
				jsonStr: `{"min":{"cpu":"1","memory":"4Gi"},"max":{"cpu":"15","memory":"60Gi"}}`,
			},
			want: &ResourceRange{
				Min: Resource{
					Cpu:       "1",
					Memory:    "4Gi",
					cpuVal:    apires.MustParse("1"),
					memoryVal: apires.MustParse("4Gi"),
				},
				Max: Resource{
					Cpu:       "15",
					Memory:    "60Gi",
					cpuVal:    apires.MustParse("15"),
					memoryVal: apires.MustParse("60Gi"),
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseResourceRange(tt.args.jsonStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseResourceRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseResourceRange() got = %v, want %v", got, tt.want)
			}
		})
	}
}

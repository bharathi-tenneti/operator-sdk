// Copyright 2020 The Operator-SDK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helm

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/operator-framework/operator-sdk/internal/scaffold"
	"github.com/operator-framework/operator-sdk/internal/util/fileutil"
)

func TestUpdateHelmWatchForResource(t *testing.T) {
	watchFilePath1 := "./testdata/testwatches/valid-1/watches.yaml"
	if err := ioutil.WriteFile(watchFilePath1, []byte(sampleWatch), fileutil.DefaultFileMode); err != nil {
		fmt.Printf("failed to write sample watch %v", err)
	}
	watchFilePath2 := "./testdata/testwatches/valid-2/watches.yaml"
	if err := ioutil.WriteFile(watchFilePath2, []byte(sampleWatch), fileutil.DefaultFileMode); err != nil {
		fmt.Printf("failed to write sample watch %v", err)
	}

	defer remove(watchFilePath1)
	defer remove(watchFilePath2)

	testCases := []struct {
		name           string
		r              *scaffold.Resource
		absProjectPath string
		chart          string
		expError       string
		expWatchesFile string
	}{
		{
			// Valid watch without chart
			name: "Valid watch without chart",
			r: &scaffold.Resource{
				APIVersion: "app.example.com/v1alpha1",
				Kind:       "App",
				LowerKind:  "app",
				FullGroup:  "app.example.com",
				Version:    "v1alpha1",
			},
			absProjectPath: "./testdata/testwatches/valid-1",
			expError:       "",
			expWatchesFile: "./testdata/testwatches/valid-1/validWatches.yaml",
		},
		{
			// Valid watch with chart
			name: "Valid watch with chart",
			r: &scaffold.Resource{
				APIVersion: "app.example.com/v1alpha1",
				Kind:       "App",
				LowerKind:  "app",
				FullGroup:  "app.example.com",
				Version:    "v1alpha1",
			},
			absProjectPath: "./testdata/testwatches/valid-2",
			chart:          "appservice",
			expError:       "",
			expWatchesFile: "./testdata/testwatches/valid-2/validWatches.yaml",
		},
		{
			// Invalid Watch
			name: "Duplicate GVK",
			r: &scaffold.Resource{
				APIVersion: "app.example.com/v1alpha1",
				Kind:       "App",
				LowerKind:  "app",
				FullGroup:  "app.example.com",
				Version:    "v1alpha1",
			},
			absProjectPath: "./testdata/testwatches/valid-1",
			expError:       "duplicate GVK: app.example.com/v1alpha1, Kind=App",
		},
		{
			// Invalid Watch
			name: "Empty Directory",
			r: &scaffold.Resource{
				APIVersion: "app.example.com/v1alpha1",
				Kind:       "App",
				LowerKind:  "app",
				FullGroup:  "app.example.com",
				Version:    "v1alpha1",
			},
			absProjectPath: "./testdata/testwatches/",
			expError: "failed to read watch manifest testdata/testwatches/watches.yaml: " +
				"open testdata/testwatches/watches.yaml: no such file or directory",
		},
		{
			// Invalid Watch
			name: "Invalid Watch file",
			r: &scaffold.Resource{
				APIVersion: "app.example.com/v1alpha1",
				Kind:       "App",
				LowerKind:  "app",
				FullGroup:  "app.example.com",
				Version:    "v1alpha1",
			},
			absProjectPath: "./testdata/testwatches/invalid/invalid_watch",
			expError: "failed to unmarshal watch config yaml: unmarshal errors:" + "\n" +
				"  line 2: cannot unmarshal !!map into []watches.Watch ",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var result string
			if err := UpdateHelmWatchForResource(tc.r, tc.absProjectPath, tc.chart); err != nil {
				result = err.Error()
			}
			assert.Equal(t, tc.expError, result)
			// Check watchfile content
			if tc.expError == "" {
				actualFilePath := tc.absProjectPath + "/watches.yaml"
				expectedWatchFile, err := ioutil.ReadFile(tc.expWatchesFile)
				if err != nil {
					fmt.Printf("failed to read expectedWatchFile  %v: %v", tc.expWatchesFile, err)
				}
				actualWatchFile, err := ioutil.ReadFile(actualFilePath)
				if err != nil {
					fmt.Printf("failed to read actualWatchFile  %v: %v", actualFilePath, err)
				}
				assert.Equal(t, expectedWatchFile, actualWatchFile)
			}
		})
	}
}

// remove removes path from disk. Used in defer statements.
func remove(path string) {
	if err := os.RemoveAll(path); err != nil {
		log.Fatal(err)
	}
}

const sampleWatch = `---
- group: mykind.example.com
  version: v1alpha1
  kind: MyKind
  chart: helm-charts/mykind`

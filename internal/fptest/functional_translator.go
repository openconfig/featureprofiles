// Copyright 2024 Google LLC
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

package fptest

import (
	"testing"

	"github.com/openconfig/functional-translators/registrar"
	"github.com/openconfig/ygnmi/ygnmi"
)

// GetOptsForFunctionalTranslator returns ygnmi options for a given functional translator name.
func GetOptsForFunctionalTranslator(t testing.TB, functionalTranslatorName string) []ygnmi.Option {
	t.Helper()
	if functionalTranslatorName == "" {
		return nil
	}
	ft, ok := registrar.FunctionalTranslatorRegistry[functionalTranslatorName]
	if !ok {
		t.Fatalf("Functional translator %q is not registered", functionalTranslatorName)
	}
	return []ygnmi.Option{ygnmi.WithFT(ft)}
}

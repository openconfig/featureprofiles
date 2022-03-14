// Copyright 2022 Google LLC
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

	"github.com/openconfig/testt"
)

// NonFatal converts fatal to an error so the test could continue.
func NonFatal(t testing.TB, f func(t testing.TB)) (ok bool) {
	msg := testt.ExpectFatal(t, func(t testing.TB) {
		f(t)
		ok = true
		t.FailNow()
	})
	if !ok {
		t.Error(msg)
	}
	return ok
}

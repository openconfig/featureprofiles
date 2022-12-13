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

/**
 * Package args define arguments that remains constant for a single vendor but
 * varies with each different vendor.
 * Having these arguments at the project level help us run the whole suite of tests
 * without defining them per test.
 */
package args

import (
	"flag"
)

var (
	P4RTNodeName1 = flag.String("arg_p4rt_node_name1", "",
		"Name for the P4 Runtime Controller. This is different for different vendors.")
	P4RTNodeName2 = flag.String("arg_p4rt_node_name2", "",
		"Name for the P4 Runtime Controller. This is different for different vendors.")
)

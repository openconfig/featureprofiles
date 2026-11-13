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

// Package binding implements a simple binding that can work with a
// specific hardware configuration without a reservation system.
//
// SONiC translib unmarshals gNMI Set values in the RESTCONF shape, while
// ygnmi sends the bare node value. Set gnmi.restconf_json on that DUT in
// the binding file, or pass -restconf-gnmi, so DialGNMI rewrites those
// payloads and normalizes JSON-IETF replies. Leave both unset for a NOS
// that accepts the ygnmi payload directly.
package binding

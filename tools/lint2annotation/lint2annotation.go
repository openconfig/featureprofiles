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

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

type diag struct {
	Category string `json:"category,omitempty"`
	Posn     string `json:"posn"`
	Message  string `json:"message"`
}

type jsonOutput map[string]map[string][]diag

func main() {
	outfile := os.Args[1]
	outBytes, err := os.ReadFile(outfile)
	if err != nil {
		log.Fatal(err)
	}

	out := jsonOutput{}
	if err := json.Unmarshal(outBytes, &out); err != nil {
		log.Fatal(err)
	}
	for _, pkg := range out {
		for _, diags := range pkg {
			for _, diag := range diags {
				pos := strings.Split(diag.Posn, ":")
				fmt.Printf("::error file=%s,line=%s,col=%s::%s\n", pos[0], pos[1], pos[2], diag.Message)
			}
		}
	}
}

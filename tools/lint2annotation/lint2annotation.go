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

// The lint2annotation command converts Go analysis Diagnostic messages into GitHub annotations.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"flag"
)

type diag struct {
	Posn    string `json:"posn"`
	Message string `json:"message"`
}

type diagErr struct {
	Err string `json:"error"`
}

type jsonOutput map[string]map[string]json.RawMessage

var (
	severity = flag.String("severity", "notice", "Sets the severity of the Github annotations")
)

func main() {
	flag.Parse()
	dec := json.NewDecoder(os.Stdin)
	for {
		out := jsonOutput{}
		if err := dec.Decode(&out); err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}
		for _, pkg := range out {
			for _, msg := range pkg {
				var diags []diag
				if err := json.Unmarshal(msg, &diags); err == nil {
					for _, diag := range diags {
						pos := strings.Split(diag.Posn, ":")
						fmt.Printf("::%s file=%s,line=%s,col=%s::%s\n", *severity, pos[0], pos[1], pos[2], diag.Message)
					}
					continue
				}
				diagErr := &diagErr{}
				if err := json.Unmarshal(msg, diagErr); err == nil {
					fmt.Printf("Error reported by analyzer %v\n", diagErr)
					continue
				}
				fmt.Printf("Unknown message type: %q\n", string(msg))
			}
		}
	}
}

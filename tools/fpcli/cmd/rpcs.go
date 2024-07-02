// Copyright © 2024 Google LLC
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

package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/openconfig/featureprofiles/tools/internal/ocrpcs"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/exp/maps"
)

// rpcsCmd represents the rpcs command
var rpcsCmd = &cobra.Command{
	Use:   "rpcs",
	Short: "rpcs is used to show all RPCs belonging to one or more OpenConfig interfaces (e.g. gnmi, gnoi)",
	Long: `rpcs is used to show all RPCs belonging to one or more OpenConfig interfaces (e.g. gnmi, gnoi)

Example:
$ fpcli show rpcs gnoi -d tmp

gnoi.bgp.BGP.ClearBGPNeighbor
gnoi.bootconfig.BootConfig.GetBootConfig
gnoi.bootconfig.BootConfig.SetBootConfig
gnoi.certificate.CertificateManagement.CanGenerateCSR
gnoi.certificate.CertificateManagement.GenerateCSR
...`,
	Run: func(cmd *cobra.Command, args []string) {
		downloadPath := viper.GetString("download-dir")
		if err := os.MkdirAll(downloadPath, 0750); err != nil {
			fmt.Fprintf(os.Stderr, "cannot create download path directory: %v", downloadPath)
			os.Exit(1)
		}
		for _, protocol := range args {
			ps, err := ocrpcs.Read(downloadPath, protocol)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to read OC protocol %q: %v", protocol, err)
				os.Exit(1)
			}
			rpcs := maps.Keys(ps)
			sort.Strings(rpcs)
			fmt.Println(strings.Join(rpcs, "\n"))
		}
	},
}

func init() {
	showCmd.AddCommand(rpcsCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// rpcsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// rpcsCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rpcsCmd.Flags().StringP("download-dir", "d", "", "Directory to download OC repositories. If already downloaded, then won't download again.")
	rpcsCmd.MarkFlagRequired("download-dir")
	viper.BindPFlag("download-dir", rpcsCmd.Flags().Lookup("download-dir"))
}

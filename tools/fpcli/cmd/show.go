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
	"github.com/spf13/cobra"
)

// showCmd represents the show command
var showCmd = &cobra.Command{
	Use:   "show",
	Short: "show is used to show information related to OpenConfig featureprofiles",
	Long: `show is used to show information related to OpenConfig featureprofiles.

For example, you can use it to show what RPCs exist for a particular OpenConfig protocol:

Example:
$ fpcli show rpcs gnoi -d tmp

gnoi.bgp.BGP.ClearBGPNeighbor
gnoi.bootconfig.BootConfig.GetBootConfig
gnoi.bootconfig.BootConfig.SetBootConfig
gnoi.certificate.CertificateManagement.CanGenerateCSR
gnoi.certificate.CertificateManagement.GenerateCSR
...`,
	// Uncomment the following line if "show"
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

func init() {
	rootCmd.AddCommand(showCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// showCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// showCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

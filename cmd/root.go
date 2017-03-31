// Copyright 2017 Qubit Ltd.
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

package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/QubitProducts/dubber"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config.yaml", "config file (default is dubber.yaml)")
	RootCmd.PersistentFlags().BoolVar(&dryrun, "dry-run", false, "Just log the actions to be taken")
}

var cfgFile = "dubber.yaml"
var dryrun bool

var RootCmd = &cobra.Command{
	Use:   "dubber",
	Short: "dubber provisions DNS names for dynamic services",
	Long: `A tool for dynamically updating DNS providers based on applications
                discovered from orchestration tools.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		sigs := make(chan os.Signal)
		signal.Notify(sigs, os.Interrupt)
		go func() {
			<-sigs
			cancel()
		}()

		r, err := os.Open(cfgFile)
		if err != nil {
			log.Fatalf("Unable to open config file %s, %v", cfgFile, err)
		}

		cfg, err := dubber.FromYAML(r)
		if err != nil {
			log.Fatalf("Unable to read config, %v", err)
		}

		dubber.Run(ctx, cfg)
	},
}

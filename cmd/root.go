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
	goflag "flag"
	"os"
	"os/signal"

	"github.com/QubitProducts/dubber"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
)

var cfgFile = "dubber.yaml"
var dryrun bool
var oneshot bool

// RootCmd is the main Cobra command for the dubber application
var RootCmd *cobra.Command

func init() {
	goflag.CommandLine.Set("alsologtostderr", "true")
	RootCmd = &cobra.Command{
		Use:   "dubber",
		Short: "dubber provisions DNS names for dynamic services",
		Long: `A tool for dynamically updating DNS providers based on applications
                discovered from orchestration tools.`,
	}

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config.yaml", "config file (default is dubber.yaml)")
	RootCmd.PersistentFlags().BoolVar(&dryrun, "dry-run", false, "Just log the actions to be taken")
	RootCmd.PersistentFlags().BoolVar(&oneshot, "oneshot", false, "Do one run only and exit")
	RootCmd.PersistentFlags().AddGoFlagSet(goflag.CommandLine)
	RootCmd.Run = func(cmd *cobra.Command, args []string) {
		defer glog.Flush()
		goflag.CommandLine.Parse([]string{})

		glog.Info("Starting dubber")

		ctx, cancel := context.WithCancel(context.Background())
		sigs := make(chan os.Signal)
		signal.Notify(sigs, os.Interrupt)
		go func() {
			<-sigs
			cancel()
		}()

		r, err := os.Open(cfgFile)
		if err != nil {
			glog.Fatalf("Unable to open config file %s, %v", cfgFile, err)
		}

		cfg, err := dubber.FromYAML(r)
		if err != nil {
			glog.Fatalf("Unable to read config, %v", err)
		}

		cfg.DryRun = dryrun
		cfg.OneShot = oneshot

		err = dubber.Run(ctx, cfg)
		if err != nil {
			glog.Fatalf("run failed, %v", err)
		}

		os.Exit(0)
	}
}

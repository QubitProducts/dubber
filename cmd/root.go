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
	"net/http"
	"os"
	"os/signal"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/QubitProducts/dubber"
	"github.com/spf13/cobra"
	klog "k8s.io/klog/v2"
)

var cfgFile = "dubber.yaml"
var statsAddr = ":8080"
var dryrun bool
var oneshot bool
var pollInterval time.Duration

// RootCmd is the main Cobra command for the dubber application
var RootCmd *cobra.Command

func init() {
	RootCmd = &cobra.Command{
		Use:   "dubber",
		Short: "dubber provisions DNS names for dynamic services",
		Long: `A tool for dynamically updating DNS providers based on applications
                discovered from orchestration tools.`,
	}

	klog.InitFlags(goflag.CommandLine)
	RootCmd.PersistentFlags().AddGoFlagSet(goflag.CommandLine)
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", cfgFile, "config file (default is dubber.yaml)")
	RootCmd.PersistentFlags().StringVar(&statsAddr, "addr", statsAddr, "statistics endpoint")
	RootCmd.PersistentFlags().BoolVar(&dryrun, "dry-run", false, "Just log the actions to be taken")
	RootCmd.PersistentFlags().BoolVar(&oneshot, "oneshot", false, "Do one run only and exit")
	RootCmd.PersistentFlags().DurationVar(&pollInterval, "poll.interval", time.Minute*1, "How often to poll and check for updates")
	RootCmd.PersistentFlags().AddGoFlagSet(goflag.CommandLine)
	RootCmd.Run = func(cmd *cobra.Command, args []string) {
		goflag.CommandLine.Set("alsologtostderr", "true")

		goflag.CommandLine.Parse([]string{})

		klog.Info("Starting dubber")

		ctx, cancel := context.WithCancel(context.Background())
		sigs := make(chan os.Signal)
		signal.Notify(sigs, os.Interrupt)
		go func() {
			sig := <-sigs
			klog.Infof("Shutting down due to %v", sig)
			cancel()
		}()

		var g *errgroup.Group
		g, ctx = errgroup.WithContext(ctx)

		r, err := os.Open(cfgFile)
		if err != nil {
			klog.Fatalf("Unable to open config file %s, %v", cfgFile, err)
		}

		cfg, err := dubber.FromYAML(r)
		if err != nil {
			klog.Fatalf("Unable to read config, %v", err)
		}

		cfg.DryRun = dryrun
		cfg.OneShot = oneshot
		cfg.PollInterval = pollInterval

		d := dubber.New(&cfg)

		if statsAddr != "" {
			g.Go(func() error {
				if err := http.ListenAndServe(statsAddr, d); err != nil {
					klog.Fatalf("stats service failed, %v", err)
					return err
				}
				return nil
			})
		}

		g.Go(func() error {
			if err := d.Run(ctx); err != nil {
				klog.Fatalf("runner failed, %v", err)
				return err
			}
			return nil
		})

		<-ctx.Done()

		if ctx.Err() != context.Canceled && ctx.Err() != nil {
			klog.Fatalf("%v", ctx.Err())
		}
	}
}

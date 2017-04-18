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

package dubber

import (
	"context"
	"net/http"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server wraps the configuration and basic functionality.
type Server struct {
	cfg *Config

	*http.ServeMux
	*prometheus.Registry
	MetricActiveDicoverers prometheus.Gauge
	MetricDiscovererRuns   *prometheus.CounterVec
	MetricReconcileRuns    *prometheus.CounterVec
	MetricReconcileTimes   *prometheus.HistogramVec
}

// New creates a new dubber server.
func New(cfg *Config) *Server {
	srv := &Server{
		cfg:      cfg,
		ServeMux: http.NewServeMux(),
		Registry: prometheus.NewRegistry(),
	}

	srv.MetricActiveDicoverers = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "dubber_active_discoverers",
		Help: "Current running number of discoverers.",
	})

	srv.MetricDiscovererRuns = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dubber_discoverer_runs_total",
		Help: "Total count of discoverer runs.",
	}, []string{"status"})

	srv.MetricReconcileRuns = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "dubber_reconcile_runs_total",
		Help: "Total count of reconcile runs.",
	}, []string{"status"})

	srv.MetricReconcileTimes = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "dubber_reconcile_time_seconds",
		Help: "Timings for reconcile runs",
	}, []string{"zone"})

	srv.MustRegister(srv.MetricActiveDicoverers)
	srv.MustRegister(srv.MetricDiscovererRuns)
	srv.MustRegister(srv.MetricReconcileRuns)
	srv.MustRegister(srv.MetricReconcileTimes)

	srv.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("OK")) })
	srv.Handle("/metrics", promhttp.HandlerFor(srv.Registry, promhttp.HandlerOpts{}))

	return srv
}

// Run process the configuration, passing updates form discoverers,
// managing state, and request action from provisioners.
func (srv *Server) Run(ctx context.Context) error {
	provs, err := srv.cfg.BuildProvisioners()
	if err != nil {
		return err
	}

	var provisionZones []string
	for k := range provs {
		provisionZones = append(provisionZones, k)
	}

	ds, err := srv.cfg.BuildDiscoveres()
	if err != nil {
		return err
	}

	type update struct {
		i int
		z Zone
	}
	upds := make(chan update)

	// Launch the discoverers
	for i, d := range ds {
		go func(i int, d Discoverer) {
			srv.MetricActiveDicoverers.Inc()
			defer srv.MetricActiveDicoverers.Dec()

			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					z, err := d.Discover(ctx)
					if err != nil {
						glog.Info("error", err)
						srv.MetricDiscovererRuns.With(prometheus.Labels{"status": "failed"}).Inc()
					}
					srv.MetricDiscovererRuns.With(prometheus.Labels{"status": "success"}).Inc()
					upds <- update{i, z}
				}
			}
		}(i, d)
	}

	dzones := make([]Zone, len(ds))
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case up := <-upds:
			dzones[up.i] = up.z

			var fullZone Zone
			for i := range dzones {
				fullZone = append(fullZone, dzones[i]...)
			}

			zones := fullZone.Partition(provisionZones)

			for zn, newzone := range zones {
				p, ok := provs[zn]
				if !ok {
					glog.V(1).Infof("no provisioner for zone %q\n", zn)
					continue
				}
				func() {
					timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
						srv.MetricReconcileTimes.With(prometheus.Labels{"zone": zn}).Observe(v)
					}))
					defer timer.ObserveDuration()

					if err := srv.ReconcileZone(p, newzone); err != nil {
						glog.Infof(err.Error())
						srv.MetricReconcileRuns.With(prometheus.Labels{"status": "failed"}).Inc()
						return
					}
					srv.MetricReconcileRuns.With(prometheus.Labels{"status": "success"}).Inc()
				}()
			}
		}
	}
}

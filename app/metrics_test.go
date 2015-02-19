// Copyright 2015 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/tsuru/tsuru/app/bind"
	"gopkg.in/check.v1"
)

type metricHandler struct {
	cpuMax string
}

func (h *metricHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	content := fmt.Sprintf(`[{"target": "sometarget", "datapoints": [[2.2, 1415129040], [2.2, 1415129050], [2.2, 1415129060], [2.2, 1415129070], [%s, 1415129080]]}]`, h.cpuMax)
	w.Write([]byte(content))
}

func (s *S) TestMetric(c *check.C) {
	h := metricHandler{cpuMax: "8.2"}
	ts := httptest.NewServer(&h)
	defer ts.Close()
	newApp := App{
		Name:     "myApp",
		Platform: "Django",
		Env: map[string]bind.EnvVar{
			"GRAPHITE_HOST": {Name: "GRAPHITE_HOST", Value: ts.URL},
		},
	}
	cpu, err := newApp.Metric("cpu")
	c.Assert(err, check.IsNil)
	c.Assert(cpu, check.Equals, 8.2)
}

func (s *S) TestMetricServerDown(c *check.C) {
	h := metricHandler{cpuMax: "8.2"}
	ts := httptest.NewServer(&h)
	newApp := App{
		Name:     "myApp",
		Platform: "Django",
	}
	ts.Close()
	_, err := newApp.Metric("cpu")
	c.Assert(err, check.Not(check.IsNil))
}

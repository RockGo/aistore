// Package stats provides methods and functionality to register, track, log,
// and StatsD-notify statistics that, for the most part, include "counter" and "latency" kinds.
/*
 * Copyright (c) 2018-2020, NVIDIA CORPORATION. All rights reserved.
 */
package stats

import (
	"time"

	"github.com/NVIDIA/aistore/3rdparty/atomic"
	"github.com/NVIDIA/aistore/3rdparty/glog"
	"github.com/NVIDIA/aistore/cluster"
	"github.com/NVIDIA/aistore/cmn"
	"github.com/NVIDIA/aistore/cmn/cos"
)

type (
	Prunner struct {
		statsRunner
	}
	ClusterStats struct {
		Proxy  *CoreStats          `json:"proxy"`
		Target map[string]*Trunner `json:"target"`
	}
	ClusterStatsRaw struct {
		Proxy  *CoreStats      `json:"proxy"`
		Target cos.JSONRawMsgs `json:"target"`
	}
)

/////////////
// Prunner //
/////////////

// interface guard
var _ cos.Runner = (*Prunner)(nil)

func (r *Prunner) Run() error                  { return r.runcommon(r) }
func (r *Prunner) CoreStats() *CoreStats       { return r.Core }
func (r *Prunner) Get(name string) (val int64) { return r.Core.get(name) }

func (*Prunner) RegMetrics() {} // have only common (regCommon())

// All stats that proxy currently has are CoreStats which are registered at startup
func (r *Prunner) Init(p cluster.Node) *atomic.Bool {
	r.Core = &CoreStats{}
	r.Core.init(24)
	r.Core.statsTime = cmn.GCO.Get().Periodic.StatsTime.D()
	r.ctracker = make(copyTracker, 24)
	r.Core.initStatsD(p.Snode())

	r.statsRunner.name = "proxystats"
	r.statsRunner.daemon = p

	r.statsRunner.stopCh = make(chan struct{}, 4)
	r.statsRunner.workCh = make(chan NamedVal64, 256)
	return &r.statsRunner.startedUp
}

// TODO: fix the scope of the return type
func (r *Prunner) GetWhatStats() interface{} {
	ctracker := make(copyTracker, 24)
	r.Core.copyCumulative(ctracker)
	return ctracker
}

// statsLogger interface impl
func (r *Prunner) log(uptime time.Duration) {
	r.Core.UpdateUptime(uptime)
	if idle := r.Core.copyT(r.ctracker, []string{"kalive", PostCount, Uptime}); !idle {
		b := cos.MustMarshal(r.ctracker)
		glog.Infoln(string(b))
	}
}

func (r *Prunner) doAdd(nv NamedVal64) {
	s := r.Core
	s.doAdd(nv.Name, nv.NameSuffix, nv.Value)
}

func (r *Prunner) statsTime(newval time.Duration) {
	r.Core.statsTime = newval
}

// Package xs is a collection of eXtended actions (xactions), including multi-object
// operations, list-objects, (cluster) rebalance and (target) resilver, ETL, and more.
/*
 * Copyright (c) 2018-2023, NVIDIA CORPORATION. All rights reserved.
 */
package xs

import (
	"path/filepath"
	"sync"

	"github.com/NVIDIA/aistore/api/apc"
	"github.com/NVIDIA/aistore/cluster"
	"github.com/NVIDIA/aistore/cluster/meta"
	"github.com/NVIDIA/aistore/cmn"
	"github.com/NVIDIA/aistore/cmn/cos"
	"github.com/NVIDIA/aistore/cmn/debug"
	"github.com/NVIDIA/aistore/cmn/nlog"
	"github.com/NVIDIA/aistore/fs"
	"github.com/NVIDIA/aistore/fs/mpather"
	"github.com/NVIDIA/aistore/xact"
	"github.com/NVIDIA/aistore/xact/xreg"
)

// XactDirPromote copies a bucket locally within the same cluster

type (
	proFactory struct {
		xreg.RenewBase
		xctn *XactDirPromote
		args *cluster.PromoteArgs
	}
	XactDirPromote struct {
		p    *proFactory
		smap *meta.Smap
		xact.BckJog
		confirmedFshare bool // set separately in the commit phase prior to Run
	}
)

// interface guard
var (
	_ cluster.Xact   = (*XactDirPromote)(nil)
	_ xreg.Renewable = (*proFactory)(nil)
)

////////////////
// proFactory //
////////////////

func (*proFactory) New(args xreg.Args, bck *meta.Bck) xreg.Renewable {
	c := args.Custom.(*cluster.PromoteArgs)
	p := &proFactory{RenewBase: xreg.RenewBase{Args: args, Bck: bck}, args: c}
	return p
}

func (p *proFactory) Start() error {
	xctn := &XactDirPromote{p: p}
	xctn.BckJog.Init(p.Args.UUID /*global xID*/, apc.ActPromote, p.Bck, &mpather.JgroupOpts{T: p.T}, cmn.GCO.Get())
	p.xctn = xctn
	return nil
}

func (*proFactory) Kind() string        { return apc.ActPromote }
func (p *proFactory) Get() cluster.Xact { return p.xctn }

func (*proFactory) WhenPrevIsRunning(xreg.Renewable) (xreg.WPR, error) {
	return xreg.WprKeepAndStartNew, nil
}

////////////////////
// XactDirPromote //
////////////////////

func (r *XactDirPromote) SetFshare(v bool) { r.confirmedFshare = v } // is called before Run()

func (r *XactDirPromote) Run(wg *sync.WaitGroup) {
	wg.Done()

	dir := r.p.args.SrcFQN
	nlog.Infof("%s(%s)", r.Name(), dir)

	r.smap = r.p.T.Sowner().Get()
	var (
		err  error
		opts = &fs.WalkOpts{Dir: dir, Callback: r.walk, Sorted: false}
	)
	if r.p.args.Recursive {
		err = fs.Walk(opts) // godirwalk
	} else {
		err = fs.WalkDir(dir, r.walk) // Go filepath.WalkDir
	}
	r.AddErr(err)
	r.Finish()
}

func (r *XactDirPromote) walk(fqn string, de fs.DirEntry) error {
	if de.IsDir() {
		return nil
	}
	debug.Assert(filepath.IsAbs(fqn))
	bck := r.Bck()

	// promote
	args := r.p.args
	objName, err := cmn.PromotedObjDstName(fqn, args.SrcFQN, args.ObjName)
	if err != nil {
		return err
	}
	// file share == true: promote only the part of the namespace that "lands" locally
	if r.confirmedFshare {
		si, err := r.smap.HrwName2T(bck.MakeUname(objName))
		if err != nil {
			return err
		}
		if si.ID() != r.p.T.SID() {
			return nil
		}
	}
	params := cluster.PromoteParams{
		Bck:  bck,
		Xact: r,
		PromoteArgs: cluster.PromoteArgs{
			SrcFQN:       fqn,
			ObjName:      objName,
			OverwriteDst: args.OverwriteDst,
			DeleteSrc:    args.DeleteSrc,
		},
	}
	// TODO: continue-on-error (unify w/ x-archive)
	_, err = r.p.T.Promote(&params)
	if cmn.IsNotExist(err) {
		err = nil
	}
	if r.BckJog.Config.FastV(5, cos.SmoduleXs) {
		nlog.Infof("%s: %s => %s (over=%t, del=%t, share=%t): %v", r.Base.Name(), fqn, bck.Cname(objName),
			args.OverwriteDst, args.DeleteSrc, r.confirmedFshare, err)
	}
	return err
}

func (r *XactDirPromote) Snap() (snap *cluster.Snap) {
	snap = &cluster.Snap{}
	r.ToSnap(snap)

	snap.IdleX = r.IsIdle()
	return
}

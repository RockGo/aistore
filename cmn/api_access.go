// Package provides common low-level types and utilities for all aistore projects
/*
 * Copyright (c) 2018-2020, NVIDIA CORPORATION. All rights reserved.
 */
package cmn

import (
	"fmt"
	"strings"
)

const (
	// object
	AccessGET = 1 << iota
	AccessObjHEAD
	AccessPUT
	AccessAPPEND
	AccessDownload
	AccessObjDELETE
	AccessObjRENAME
	AccessPROMOTE
	// bucket
	AccessBckHEAD
	AccessObjLIST
	AccessBckRENAME
	AccessPATCH
	AccessMAKENCOPIES
	AccessEC
	AccessSYNC
	AccessBckDELETE
	// cluster
	AccessBckCreate
	AccessBckLIST
	AccessADMIN
	// must be the last one
	AccessMax

	// Permissions
	allowAllAccess       = ^uint64(0)
	allowReadOnlyAccess  = AccessGET | AccessObjHEAD | AccessBckHEAD | AccessObjLIST
	allowReadWriteAccess = allowReadOnlyAccess |
		AccessPUT | AccessAPPEND | AccessDownload | AccessObjDELETE | AccessObjRENAME
	allowClusterAccess = ^uint64(0) & (AccessBckCreate - 1)

	// Permission Operations
	AllowAccess = "allow"
	DenyAccess  = "deny"
)

// access => operation
var accessOp = map[int]string{
	// object
	AccessGET:       "GET",
	AccessObjHEAD:   "HEAD-OBJECT",
	AccessPUT:       "PUT",
	AccessAPPEND:    "APPEND",
	AccessDownload:  "DOWNLOAD",
	AccessObjDELETE: "DELETE-OBJECT",
	AccessObjRENAME: "RENAME-OBJECT",
	AccessPROMOTE:   "PROMOTE",
	// bucket
	AccessBckHEAD:     "HEAD-BUCKET",
	AccessObjLIST:     "LIST-OBJECTS",
	AccessBckRENAME:   "RENAME-BUCKET",
	AccessPATCH:       "PATCH",
	AccessMAKENCOPIES: "MAKE-NCOPIES",
	AccessEC:          "EC",
	AccessSYNC:        "SYNC-BUCKET",
	AccessBckDELETE:   "DELETE-BUCKET",
	// cluster
	AccessBckCreate: "CREATE-BUCKET",
	AccessADMIN:     "ADMIN",
}

func NoAccess() uint64        { return 0 }
func AllAccess() uint64       { return allowAllAccess }
func ReadOnlyAccess() uint64  { return allowReadOnlyAccess }
func ReadWriteAccess() uint64 { return allowReadWriteAccess }

func AccessOp(access int) string {
	if s, ok := accessOp[access]; ok {
		return s
	}
	return "<unknown access>"
}

func (bp *BucketProps) AccessToStr() string {
	return accessToStr(bp.AccessAttrs)
}

func accessToStr(aattrs uint64) string {
	if aattrs == 0 {
		return "No access"
	}
	accList := make([]string, 0, 24)
	if aattrs&AccessGET == AccessGET {
		accList = append(accList, accessOp[AccessGET])
	}
	if aattrs&AccessObjHEAD == AccessObjHEAD {
		accList = append(accList, accessOp[AccessObjHEAD])
	}
	if aattrs&AccessPUT == AccessPUT {
		accList = append(accList, accessOp[AccessPUT])
	}
	if aattrs&AccessAPPEND == AccessAPPEND {
		accList = append(accList, accessOp[AccessAPPEND])
	}
	if aattrs&AccessDownload == AccessDownload {
		accList = append(accList, accessOp[AccessDownload])
	}
	if aattrs&AccessObjDELETE == AccessObjDELETE {
		accList = append(accList, accessOp[AccessObjDELETE])
	}
	if aattrs&AccessObjRENAME == AccessObjRENAME {
		accList = append(accList, accessOp[AccessObjRENAME])
	}
	if aattrs&AccessPROMOTE == AccessPROMOTE {
		accList = append(accList, accessOp[AccessPROMOTE])
	}
	//
	if aattrs&AccessBckHEAD == AccessBckHEAD {
		accList = append(accList, accessOp[AccessBckHEAD])
	}
	if aattrs&AccessObjLIST == AccessObjLIST {
		accList = append(accList, accessOp[AccessObjLIST])
	}
	if aattrs&AccessBckRENAME == AccessBckRENAME {
		accList = append(accList, accessOp[AccessBckRENAME])
	}
	if aattrs&AccessPATCH == AccessPATCH {
		accList = append(accList, accessOp[AccessPATCH])
	}
	if aattrs&AccessMAKENCOPIES == AccessMAKENCOPIES {
		accList = append(accList, accessOp[AccessMAKENCOPIES])
	}
	if aattrs&AccessSYNC == AccessSYNC {
		accList = append(accList, accessOp[AccessSYNC])
	}
	if aattrs&AccessBckDELETE == AccessBckDELETE {
		accList = append(accList, accessOp[AccessBckDELETE])
	}
	return strings.Join(accList, ",")
}

func ModifyAccess(aattr uint64, action string, bits uint64) (uint64, error) {
	if action == AllowAccess {
		return aattr | bits, nil
	}
	if action != DenyAccess {
		return 0, fmt.Errorf("unknown make-access action %q", action)
	}
	return aattr & (allowAllAccess ^ bits), nil
}

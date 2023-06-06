// Package archive: write, read, copy, append, list primitives
// across all supported formats
/*
 * Copyright (c) 2018-2023, NVIDIA CORPORATION. All rights reserved.
 */
package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"

	"github.com/NVIDIA/aistore/cmn/cos"
	"github.com/NVIDIA/aistore/cmn/debug"
	"github.com/pierrec/lz4/v3"
)

type (
	ReadCB func(reader io.ReadCloser) (bool, error)

	Reader interface {
		// non-empty filename to facilitate a simple-single selection
		// (generalize as a multi-selection callback? no need yet...)
		Range(filename string) (cos.ReadCloseSizer, error)

		// private
		init(fh *os.File, readcb ReadCB) error
	}

	baseR struct {
		fh     io.Reader
		readcb ReadCB
	}
	tarReader struct {
		baseR
		tr *tar.Reader
	}
	tgzReader struct {
		tr  tarReader
		gzr *gzip.Reader
	}
	zipReader struct {
		baseR
		size int64
		zr   *zip.Reader
	}
	lz4Reader struct {
		tr  tarReader
		lzr *lz4.Reader
	}
)

// interface guard
var (
	_ Reader = (*tarReader)(nil)
	_ Reader = (*tgzReader)(nil)
	_ Reader = (*zipReader)(nil)
	_ Reader = (*lz4Reader)(nil)
)

func NewReader(mime string, fh *os.File, readcb ReadCB, size ...int64) (ar Reader, err error) {
	switch mime {
	case ExtTar:
		ar = &tarReader{}
	case ExtTgz, ExtTarTgz:
		ar = &tgzReader{}
	case ExtZip:
		debug.Assert(len(size) > 0 && size[0] > 0, "size required")
		ar = &zipReader{size: size[0]}
	case ExtTarLz4:
		ar = &lz4Reader{}
	default:
		debug.Assert(false, mime)
	}
	err = ar.init(fh, readcb)
	return
}

// baseR

func (br *baseR) init(fh io.Reader, readcb ReadCB) {
	br.fh, br.readcb = fh, readcb
}

// tarReader

func (tr *tarReader) init(fh *os.File, readcb ReadCB) error {
	tr.baseR.init(fh, readcb)
	tr.tr = tar.NewReader(fh)
	return nil
}

func (tr *tarReader) Range(filename string) (reader cos.ReadCloseSizer, _ error) {
	debug.Assert(tr.readcb != nil || filename != "") // range read OR simple selection
	for {
		hdr, ern := tr.tr.Next()
		if ern != nil {
			if ern == io.EOF {
				return nil, nil
			}
			return nil, ern
		}
		// select one
		if filename != "" {
			if hdr.Name == filename || namesEq(hdr.Name, filename) {
				reader = &cslLimited{LimitedReader: io.LimitedReader{R: tr.fh, N: hdr.Size}}
				return
			}
			continue
		}
		// range-read
		stop, err := tr.readcb(&cslLimited{LimitedReader: io.LimitedReader{R: tr.fh, N: hdr.Size}})
		if stop || err != nil {
			return nil, err
		}
	}
}

// tgzReader

func (tgr *tgzReader) init(fh *os.File, readcb ReadCB) (err error) {
	tgr.gzr, err = gzip.NewReader(fh)
	if err != nil {
		return
	}
	tgr.tr.baseR.init(tgr.gzr, readcb)
	tgr.tr.tr = tar.NewReader(tgr.gzr)
	return
}

// NOTE the convention:
// - when the method returns non-nil reader the responsibility to close the latter goes to the caller (via reader.Close)
// - otherwise, gzip.Reader is closed here upon return
func (tgr *tgzReader) Range(filename string) (reader cos.ReadCloseSizer, err error) {
	reader, err = tgr.tr.Range(filename)
	if err == nil && reader != nil {
		csc := &cslClose{gzr: tgr.gzr /*to close*/, R: reader /*to read from*/, N: reader.Size()}
		return csc, err
	}
	err = tgr.gzr.Close()
	return
}

// zipReader

func (zr *zipReader) init(fh *os.File, readcb ReadCB) (err error) {
	zr.baseR.init(fh, readcb)
	zr.zr, err = zip.NewReader(fh, zr.size)
	return
}

func (zr *zipReader) Range(filename string) (reader cos.ReadCloseSizer, err error) {
	for _, f := range zr.zr.File {
		finfo := f.FileInfo()
		if finfo.IsDir() {
			continue
		}
		// select one
		if filename != "" {
			if f.FileHeader.Name == filename || namesEq(f.FileHeader.Name, filename) {
				csf := &cslFile{size: finfo.Size()}
				csf.file, err = f.Open()
				reader = csf
				return
			}
			continue
		}
		csf := &cslFile{size: finfo.Size()}
		if csf.file, err = f.Open(); err != nil {
			return
		}
		stop, err := zr.readcb(csf)
		if stop || err != nil {
			return nil, err
		}
	}
	return
}

// lz4Reader

func (lzr *lz4Reader) init(fh *os.File, readcb ReadCB) error {
	lzr.lzr = lz4.NewReader(fh)
	lzr.tr.baseR.init(lzr.lzr, readcb)
	lzr.tr.tr = tar.NewReader(lzr.lzr)
	return nil
}

func (lzr *lz4Reader) Range(filename string) (cos.ReadCloseSizer, error) {
	return lzr.tr.Range(filename)
}

//
// more limited readers
//

type (
	cslLimited struct {
		io.LimitedReader
	}
	cslClose struct {
		gzr io.ReadCloser
		R   io.Reader
		N   int64
	}
	cslFile struct {
		file io.ReadCloser
		size int64
	}
)

//
// assorted 'limited' readers
//

func (csl *cslLimited) Size() int64 { return csl.N }
func (*cslLimited) Close() error    { return nil }

func (csc *cslClose) Read(b []byte) (int, error) { return csc.R.Read(b) }
func (csc *cslClose) Size() int64                { return csc.N }
func (csc *cslClose) Close() error               { return csc.gzr.Close() }

func (csf *cslFile) Read(b []byte) (int, error) { return csf.file.Read(b) }
func (csf *cslFile) Size() int64                { return csf.size }
func (csf *cslFile) Close() error               { return csf.file.Close() }

// in re `--absolute-names` (simplified)
func namesEq(n1, n2 string) bool {
	if n1[0] == filepath.Separator {
		n1 = n1[1:]
	}
	if n2[0] == filepath.Separator {
		n2 = n2[1:]
	}
	return n1 == n2
}

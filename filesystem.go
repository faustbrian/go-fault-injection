package faultinject

import (
	"context"
	"io/fs"
)

// WrapFS returns an fs.FS adapter with separate open and read operation
// identities. Selected returned after-open errors close an acquired file;
// organic results and successful files remain caller-owned. Selected panics
// propagate without added cleanup. Nil base returns ErrInvalidConfig; callers must also avoid
// typed nil and honor fs.FS/fs.File ownership and counts. Nil/zero Runtime returns
// base unchanged. Open/Read admission uses Background and adds no cancellation or
// serialization to the base. Optional FS/File interfaces beyond the advertised
// surface are not forwarded; file Stat/Close delegate to the acquired file.
func WrapFS(base fs.FS, runtime *Runtime, openOperation, readOperation uint32) (fs.FS, error) {
	if base == nil {
		return nil, invalid("FS", "must be non-nil")
	}
	if !runtime.active() {
		return base, nil
	}
	return &injectedFS{
		base: base, runtime: runtime,
		openOperation: openOperation, readOperation: readOperation,
	}, nil
}

type injectedFS struct {
	base          fs.FS
	runtime       *Runtime
	openOperation uint32
	readOperation uint32
}

func (filesystem *injectedFS) Open(name string) (fs.File, error) {
	decision := filesystem.runtime.Decide(context.Background(), Metadata{
		Boundary: BoundaryFilesystemOpen, Operation: filesystem.openOperation,
	})
	if err := faultPhaseError(context.Background(), decision.faults, PhaseBefore, filesystem.runtime.sleeper()); err != nil {
		return nil, err
	}
	if err := faultPhaseError(context.Background(), decision.faults, PhaseDuring, filesystem.runtime.sleeper()); err != nil {
		return nil, err
	}
	file, organicError := filesystem.base.Open(name)
	if err := faultPhaseError(context.Background(), decision.faults, PhaseAfter, filesystem.runtime.sleeper()); err != nil {
		closeFile(file)
		return nil, err
	}
	if organicError != nil || file == nil {
		return file, organicError
	}
	return &injectedFile{
		File: file,
		reader: &injectedReader{
			reader: file, runtime: filesystem.runtime,
			operation: filesystem.readOperation, boundary: BoundaryFilesystemRead,
		},
	}, nil
}

type injectedFile struct {
	fs.File
	reader *injectedReader
}

func (file *injectedFile) Read(buffer []byte) (int, error) {
	return file.reader.Read(buffer)
}

func closeFile(file fs.File) {
	if file != nil {
		_ = file.Close()
	}
}

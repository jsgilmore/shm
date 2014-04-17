//   Copyright 2014 Vastech SA (PTY) LTD
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

// +build linux

package shm

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
)

const (
	PROT_RDWR = syscall.PROT_READ | syscall.PROT_WRITE
	PROT_READ = syscall.PROT_READ
)

const (
	devHugepages = "/dev/hugepages"
	devShm       = "/dev/shm"
	shmPrefix    = "shm-"
	shmDirFormat = "%s/" + shmPrefix + "%d"
)

func uniquePath(path string) string {
	return fmt.Sprintf(shmDirFormat, path, uint64(rng.Int63()))
}

var errInvalidFS = errors.New("shm: invalid file system")

func createBuffer(dir string, size int) ([]*os.File, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(dir, &stat)
	if err != nil {
		return nil, os.NewSyscallError("statfs", err)
	}
	const HUGETLBFS_MAGIC = 0x958458f6
	const TMPFS_MAGIC = 0x1021994
	if stat.Type != HUGETLBFS_MAGIC && stat.Type != TMPFS_MAGIC {
		return nil, errInvalidFS
	}

	// Quick check for an impending ENOMEM
	if stat.Bavail == 0 {
		return nil, syscall.ENOMEM
	}

	path := uniquePath(dir)
	// O_EXCL ensures that we never open a file twice
	const flags = os.O_RDWR | os.O_CREATE | os.O_EXCL | os.O_SYNC | syscall.O_NOATIME
	file, err := os.OpenFile(path, flags, 0660)
	for err != nil {
		if os.IsExist(err) {
			path = uniquePath(path)
			file, err = os.OpenFile(path, flags, 0660)
			continue
		} else {
			return nil, err
		}
	}

	localFile, err := os.Open(path)
	if err != nil {
		checkClose(file)
		if err := os.Remove(path); err != nil {
			panic(err)
		}
		return nil, err
	}

	if err := os.Remove(path); err != nil {
		checkClose(file)
		checkClose(localFile)
		return nil, err
	}

	if stat.Type == TMPFS_MAGIC {
		// reserve space. only needed for tmpfs.
		if err := file.Truncate(int64(size)); err != nil {
			checkClose(file)
			checkClose(localFile)
			return nil, err
		}
	}

	return []*os.File{file, localFile}, nil
}

// NewBufferMemoryFs maps a mounted file system into shared memory and returns a handle to the shared memory buffer
func NewBufferMemoryFs(dir string, size, prot int) (Buffer, error) {
	files, err := createBuffer(dir, size)
	if err != nil {
		return nil, err
	}
	file, localFile := files[0], files[1]

	// MAP_NORESERVE is specifically not used because it allows mmap
	// to succeed even when there are no huge pages reserved.
	const flags = syscall.MAP_SHARED | syscall.MAP_POPULATE
	b, err := syscall.Mmap(int(file.Fd()), 0, size, prot, flags)
	if err != nil {
		checkClose(file)
		checkClose(localFile)
		return nil, os.NewSyscallError("NewBufferMemoryFs: mmap", err)
	}

	buf := &sharedBuffer{file, localFile, b, stackToKeep()}

	// Lock everything to avoid SIGBUS when trying to use memory
	// mapped on a tmpfs that becomes full after the Statfs check in
	// createBuffer.
	if err := buf.LockAll(); err != nil {
		checkClose(buf)
		return nil, err
	}

	runtime.SetFinalizer(buf, (*sharedBuffer).finalize)

	return buf, nil
}

// NewBufferTmpfs maps /dev/shm into shared memory and returns a handle to the shared memory buffer
func NewBufferTmpfs(size, prot int) (Buffer, error) {
	return NewBufferMemoryFs(devShm, size, prot)
}

// NewBufferTmpfs maps /dev/hugepages into shared memory and returns a handle to the shared memory buffer
func NewBufferHugepages(size, prot int) (Buffer, error) {
	if prot == PROT_READ {
		// PROT_READ breaks mlock of hugepages.
		panic("NewBufferHugepages: PROT_READ wont't work here")
	}
	return NewBufferMemoryFs(devHugepages, size, prot)
}

// CloseFile closes both shared memory file descriptors
func (buf *sharedBuffer) CloseFile() error {
	if buf.file != nil {
		if err := buf.file.Close(); err != nil {
			panic("shm: Close: " + err.Error())
		}
		buf.file = nil
	}
	if buf.localFile != nil {
		if err := buf.localFile.Close(); err != nil {
			panic("shm: Close: " + err.Error())
		}
		buf.localFile = nil
	}
	return nil
}

// Close Munmaps the shared memory and closes the file descriptors
func (buf *sharedBuffer) Close() error {
	if buf.buf != nil {
		if err := syscall.Munmap(buf.buf); err != nil {
			panic("shm: Close: munmap: " + err.Error())
		}
		buf.buf = nil
	}
	if err := buf.CloseFile(); err != nil {
		return err
	}
	runtime.SetFinalizer(buf, nil)
	// the errors above are fatal
	return nil
}

func (buf *sharedBuffer) Advise(offset, length, advice int) error {
	b := buf.Bytes()
	err := checkOffsetLength(b, offset, length)
	if err == nil {
		err = syscall.Madvise(b[offset:offset+length], advice)
	}
	return os.NewSyscallError("shm: madvise", err)
}

// Lock locks a region of the mapped shared memory
func (buf *sharedBuffer) Lock(offset, length int) error {
	b := buf.Bytes()
	err := checkOffsetLength(b, offset, length)
	if err == nil {
		err = syscall.Mlock(b[offset : offset+length])
	}
	return os.NewSyscallError("shm: mlock", err)
}

// Unlock unlocks a region of the mapped shared memory
func (buf *sharedBuffer) Unlock(offset, length int) error {
	b := buf.Bytes()
	err := checkOffsetLength(b, offset, length)
	if err == nil {
		err = syscall.Munlock(b[offset : offset+length])
	}
	return os.NewSyscallError("shm: munlock", err)
}

type closer interface {
	Close() error
}

func checkClose(c closer) {
	if err := c.Close(); err != nil {
		panic(err)
	}
}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

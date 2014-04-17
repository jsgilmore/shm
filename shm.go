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

// Package shm allows for opening and operating on shared memory devices
package shm

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
)

// The Buffer interface represents a shared memory buffer
type Buffer interface {
	Bytes() []byte
	Len() int
	Close() error
	CloseFile() error
	String() string
	File() *os.File
	LocalFile() *os.File
	Advise(offset, length, advice int) error
	Lock(offset, length int) error
	LockAll() error
	Unlock(offset, length int) error
	UnlockAll() error
}

type sharedBuffer struct {
	file      *os.File
	localFile *os.File
	buf       []byte
	stack     []byte
}

// String identifies the shared memory buffer by its address and file descriptor
func (buf *sharedBuffer) String() string {
	if buf == nil {
		return "SharedBuffer:<nil>"
	}
	fd := -1
	if buf.file != nil {
		fd = int(buf.file.Fd())
	}
	return fmt.Sprintf("SharedBuffer[%p]{Fd:%d}", buf, fd)
}

const keepStack = false

func stackToKeep() []byte {
	if !keepStack {
		return nil
	}
	buf := make([]byte, 16384)
	n := runtime.Stack(buf, false)
	return buf[:n]
}

func (buf *sharedBuffer) finalize() {
	if buf.stack != nil {
		panic("shm: finalize: open buffer allocated at: " + string(buf.stack))
	}
	panic("shm: finalize: open buffer")
}

// Buffer length in bytes
func (buf *sharedBuffer) Len() int {
	return len(buf.buf)
}

// Bytes returns the contents of the buffer
func (buf *sharedBuffer) Bytes() []byte {
	if buf.buf == nil {
		panic("shm: Bytes: buffer closed")
	}
	// an unexpected fault address error here can be caused by a
	// PROT_READ buffer being read without having been written
	return buf.buf
}

// File returns the file descriptor
func (buf *sharedBuffer) File() *os.File {
	return buf.file
}

// File returns the local file descriptor
func (buf *sharedBuffer) LocalFile() *os.File {
	return buf.localFile
}

// LockAll performs an Mlock of all buffer bytes
func (buf *sharedBuffer) LockAll() error {
	return buf.Lock(0, buf.Len())
}

// UnlockAll performs an Mulock of all buffer bytes
func (buf *sharedBuffer) UnlockAll() error {
	return buf.Unlock(0, buf.Len())
}

var errOffsetLength = errors.New("invalid offset/length")

func checkOffsetLength(b []byte, offset, length int) error {
	// negative checks, zero/overflow check, bounds check
	if offset < 0 || length < 0 || offset+length <= offset || offset+length > len(b) {
		return errOffsetLength
	}
	return nil
}

var errWrongSize = errors.New("shm: NewBufferFile: wrong size")

// NewBufferFile maps a file to shared memory and returns a handle to the shared memory buffer
func NewBufferFile(file *os.File, size, prot int) (Buffer, error) {
	fi, err := file.Stat()
	if err != nil {
		return nil, err
	}
	sys := fi.Sys().(*syscall.Stat_t)
	if sys.Size != int64(size) {
		return nil, errWrongSize
	}

	// Dup to allow file parameter to be closed regardless
	fd, err := syscall.Dup(int(file.Fd()))
	if err != nil {
		return nil, err
	}
	const flags = syscall.MAP_SHARED
	b, err := syscall.Mmap(fd, 0, size, prot, flags)
	if err != nil {
		return nil, err
	}

	// localFile is nil because fd is from somewhere else
	buf := &sharedBuffer{os.NewFile(uintptr(fd), ""), nil, b, stackToKeep()}
	runtime.SetFinalizer(buf, (*sharedBuffer).finalize)

	return buf, nil
}

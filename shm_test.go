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

package shm

import (
	"flag"
	"github.com/jsgilmore/mount"
	"math"
	"os"
	"runtime"
	"sync"
	"syscall"
	"testing"
)

const size64MiB = 64 << 20
const size1G = 1024 << 20

func newBuffer(size, prot int) Buffer {
	buf, err := NewBufferTmpfs(size, prot)
	if err != nil {
		panic(err)
	}
	return buf
}

func testShm(newBuffer func(_, _ int) (Buffer, error)) {
	buf, err := newBuffer(size64MiB, PROT_RDWR)
	if err != nil {
		panic(err)
	}
	defer checkClose(buf)
	fd2, err := syscall.Dup(int(buf.File().Fd()))
	if err != nil {
		panic(err)
	}
	file2 := os.NewFile(uintptr(fd2), "")
	buf2, err := NewBufferFile(file2, buf.Len(), PROT_READ)
	if err != nil {
		panic(err)
	}
	defer checkClose(buf2)

	// NewBufferFile duped file2
	checkClose(file2)

	bufSlice := buf.Bytes()
	for i := 0; i < len(bufSlice); i++ {
		bufSlice[i] = byte(i)
	}
	buf2Slice := buf2.Bytes()
	if len(bufSlice) != len(buf2Slice) {
		panic("slices must have the same length")
	}
	for i := 0; i < len(bufSlice); i++ {
		if bufSlice[i] != buf2Slice[i] {
			panic("slices not equal")
		}
	}
	buf2.Lock(0, buf2.Len())
	buf2.Unlock(0, buf2.Len())
}

func stat(dir string) syscall.Statfs_t {
	var st syscall.Statfs_t
	err := syscall.Statfs(dir, &st)
	if err != nil {
		panic(err)
	}
	return st
}

func TestShmTmpfs(t *testing.T) {
	setup()
	st := stat(devShm)
	testShm(NewBufferTmpfs)
	if st != stat(devShm) {
		panic("stat mismatch")
	}
}

func TestShmDevHugepages(t *testing.T) {
	setup()
	st := stat(devHugepages)
	testShm(NewBufferHugepages)
	if st != stat(devHugepages) {
		panic("stat mismatch")
	}
}

func TestProtReadTmpfs(t *testing.T) {
	setup()
	st := stat(devShm)
	// this doesn't work with hugepages
	buf, err := NewBufferTmpfs(size1G, PROT_READ)
	if err != nil {
		panic(err)
	}
	if err := buf.LockAll(); err != nil {
		panic(err)
	}
	checkClose(buf)
	if st != stat(devShm) {
		panic("stat mismatch")
	}
}

func BenchmarkAllocate(b *testing.B) {
	setup()
	b.SetBytes(int64(size64MiB))
	origBuf := make([]byte, size64MiB)
	b.ResetTimer()
	b.StartTimer()
	for n := b.N; n > 0; n-- {
		buf := newBuffer(size64MiB, PROT_RDWR)
		copy(buf.Bytes(), origBuf)
		buf.Close()
	}
	b.StopTimer()
}

func BenchmarkCopySliceToSlice(b *testing.B) {
	setup()
	b.SetBytes(int64(size64MiB))
	dest := make([]byte, size64MiB)
	src := make([]byte, size64MiB)
	b.ResetTimer()
	b.StartTimer()
	for n := b.N; n > 0; n-- {
		copy(dest, src)
	}
	b.StopTimer()
}

func BenchmarkMemmoveSliceToSlice(b *testing.B) {
	setup()
	b.SetBytes(int64(size64MiB))
	dest := make([]byte, size64MiB)
	src := make([]byte, size64MiB)
	b.ResetTimer()
	b.StartTimer()
	for n := b.N; n > 0; n-- {
		memmove(dest, src)
	}
	b.StopTimer()
}

func BenchmarkCopySliceToShm(b *testing.B) {
	setup()
	b.SetBytes(int64(size64MiB))
	origBuf := make([]byte, size64MiB)
	buf := newBuffer(size64MiB, PROT_RDWR)
	b.ResetTimer()
	b.StartTimer()
	for n := b.N; n > 0; n-- {
		copy(buf.Bytes(), origBuf)
	}
	b.StopTimer()
	buf.Close()
}

func BenchmarkCopyShmToShm(b *testing.B) {
	setup()
	b.SetBytes(int64(size64MiB))
	origBuf := make([]byte, size64MiB)
	buf1 := newBuffer(size64MiB, PROT_RDWR)
	copy(buf1.Bytes(), origBuf)
	buf2 := newBuffer(size64MiB, PROT_RDWR)
	b.ResetTimer()
	b.StartTimer()
	for n := b.N; n > 0; n-- {
		copy(buf2.Bytes(), buf1.Bytes())
	}
	b.StopTimer()
	buf1.Close()
	buf2.Close()
}

func BenchmarkMemcpyShmToShm(b *testing.B) {
	setup()
	b.SetBytes(int64(size64MiB))
	origBuf := make([]byte, size64MiB)
	buf1 := newBuffer(size64MiB, PROT_RDWR)
	copy(buf1.Bytes(), origBuf)
	buf2 := newBuffer(size64MiB, PROT_RDWR)
	b.ResetTimer()
	b.StartTimer()
	for n := b.N; n > 0; n-- {
		memcpy(buf2.Bytes(), buf1.Bytes())
	}
	b.StopTimer()
	buf1.Close()
	buf2.Close()
}

func BenchmarkMemmoveShmToShm(b *testing.B) {
	setup()
	b.SetBytes(int64(size64MiB))
	origBuf := make([]byte, size64MiB)
	buf1 := newBuffer(size64MiB, PROT_RDWR)
	copy(buf1.Bytes(), origBuf)
	buf2 := newBuffer(size64MiB, PROT_RDWR)
	b.ResetTimer()
	b.StartTimer()
	for n := b.N; n > 0; n-- {
		memmove(buf2.Bytes(), buf1.Bytes())
	}
	b.StopTimer()
	buf1.Close()
	buf2.Close()
}

func TestUnlinkError(t *testing.T) {
	err := syscall.Unlink("/this/does/not/exist")
	if !os.IsNotExist(err) {
		panic("expected an IsNotExist error")
	}
}

var tmpfsSize *int64 = flag.Int64("tmpfs.size", 4<<30, "tmpfs size")
var hugeSize *int64 = flag.Int64("huge.size", 4<<30, "hugetlbfs size")
var setupDone = false

func setup() {
	if setupDone {
		return
	}
	mount.MountNamespace()
	if err := mount.MountTmpfs("/dev/shm", *tmpfsSize); err != nil {
		panic(err)
	}
	if err := mount.MountHugetlbfs("/dev/hugepages", 2<<20, *hugeSize); err != nil {
		panic(err)
	}
	setupDone = true
}

func checkNil(err error) {
	if err != nil {
		panic("expected nil error")
	}
}

func checkNotNil(err error) {
	if err == nil {
		panic("expected non-nil error")
	}
}

func testOffsetLength(lenb, offset, length int) error {
	// negative checks, zero/overflow check, bounds check
	if offset < 0 || length < 0 || offset+length <= offset || offset+length > lenb {
		return errOffsetLength
	}
	return nil
}

func TestCheckOffsetLength(t *testing.T) {
	checkNotNil(checkOffsetLength(make([]byte, 0), 0, 0))
	checkNotNil(checkOffsetLength(make([]byte, 1), 0, 0))
	checkNil(checkOffsetLength(make([]byte, 1), 0, 1))
	checkNotNil(checkOffsetLength(make([]byte, 1), 0, 0))
	checkNotNil(checkOffsetLength(make([]byte, 1), 0, -1))
	checkNil(checkOffsetLength(make([]byte, 2), 0, 1))
	checkNil(checkOffsetLength(make([]byte, 2), 0, 2))
	checkNil(checkOffsetLength(make([]byte, 2), 1, 1))
	checkNotNil(checkOffsetLength(make([]byte, 2), 1, 2))
	checkNotNil(checkOffsetLength(make([]byte, 1), math.MaxInt32, 1))
	checkNotNil(checkOffsetLength(make([]byte, 1), math.MinInt32, 1))
	checkNotNil(checkOffsetLength(make([]byte, 1), 0, math.MinInt32))
	checkNil(testOffsetLength(math.MaxInt32, math.MaxInt32-1, 1))
	checkNotNil(testOffsetLength(math.MaxInt32, math.MaxInt32, 0))
	checkNotNil(testOffsetLength(math.MaxInt32, math.MaxInt32, math.MinInt32))
	checkNotNil(testOffsetLength(math.MaxInt32, math.MaxInt32-10, 11))
	checkNil(testOffsetLength(math.MaxInt32, 0, math.MaxInt32))
	checkNotNil(testOffsetLength(math.MaxInt32, 0, math.MinInt32))
	checkNotNil(testOffsetLength(math.MaxInt32-1, 0, math.MaxInt32))
}

func BenchmarkSyscalls(b *testing.B) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(8))

	buf := newBuffer(size64MiB, PROT_RDWR)
	defer checkClose(buf)

	var wg sync.WaitGroup
	workChans := make([]chan struct{}, 0, 8)
	for i := 0; i < cap(workChans); i++ {
		wg.Add(1)
		workChan := make(chan struct{})
		workChans = append(workChans, workChan)
		go func() {
			<-workChan
			for i := 0; i < b.N; i++ {
				fd2, err := syscall.Dup(int(buf.File().Fd()))
				if err != nil {
					panic(err)
				}
				file2 := os.NewFile(uintptr(fd2), "")
				buf2, err := NewBufferFile(file2, buf.Len(), PROT_READ)
				if err != nil {
					panic(err)
				}
				checkClose(buf2)
				checkClose(file2)
			}
			wg.Done()
		}()
	}

	b.StartTimer()
	for _, workChan := range workChans {
		close(workChan)
	}
	wg.Wait()
	b.StopTimer()
}

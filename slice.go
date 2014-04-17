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
	"fmt"
	"math/rand"
	"os"
	"time"
)

// NewBufferSlice returns a mock shared memory object that is backed by a byte slice, instead of mapped memory
// Pay carefull attentions to which methods are implemented
func NewBufferSlice(size int) Buffer {
	return &sliceBuf{make([]byte, size)}
}

type sliceBuf struct {
	buf []byte
}

func (s *sliceBuf) Bytes() []byte {
	return s.buf
}

func (s *sliceBuf) Len() int {
	return len(s.buf)
}

func (s *sliceBuf) Close() error {
	return nil
}

func (s *sliceBuf) CloseFile() error {
	return nil
}

func (s *sliceBuf) String() string {
	return fmt.Sprintf("sliceBuf@%p:%d", &s.buf[0], len(s.buf))
}

// Calling File will cause a panic
func (s *sliceBuf) File() *os.File {
	panic("not to be implemented")
}

// Calling LocalFile will cause a panic
func (s *sliceBuf) LocalFile() *os.File {
	panic("not to be implemented")
}

func (s *sliceBuf) Remap(prot int) {
	// no-op for now
}

func (s *sliceBuf) Advise(offset, length, advice int) error {
	// no-op for now
	return nil
}

func (s *sliceBuf) Lock(offset, length int) error {
	// no-op for now
	time.Sleep(time.Duration(rand.Intn(200e6)))
	return nil
}

func (s *sliceBuf) Unlock(offset, length int) error {
	// no-op for now
	return nil
}

func (s *sliceBuf) LockAll() error {
	// no-op for now
	return nil
}

func (s *sliceBuf) UnlockAll() error {
	// no-op for now
	return nil
}

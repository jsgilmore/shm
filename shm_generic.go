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

// +build !linux

package shm

const (
	PROT_READ  = 0
	PROT_WRITE = 0
	PROT_RDWR  = 0
)

func (buf *sharedBuffer) Close() error {
	panic("!linux: not implemented")
}

func (buf *sharedBuffer) ReadOnly() {
	panic("!linux: not implemented")
}

func (buf *sharedBuffer) Advise(offset, length, advice int) error {
	panic("!linux: not implemented")
}

func (buf *sharedBuffer) Lock(offset, length int) error {
	panic("!linux: not implemented")
}

func (buf *sharedBuffer) Unlock(offset, length int) error {
	panic("!linux: not implemented")
}

func (buf *sharedBuffer) CloseFile() error {
	panic("!linux: not implemented")
}

func NewBufferHugepages(size, prot int) (Buffer, error) {
	panic("!linux: not implemented")
}

// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package zapcore

import (
	"io"
	"sync"

	"go.uber.org/multierr"
)

// write-sync接口
// 有三个实现
// 一个是就对io.write包了一层，啥都没干，和io.write一样
// 一个是lock-write
// 一个是multi-write≤

// A WriteSyncer is an io.Writer that can also flush any buffered data. Note
// that *os.File (and thus, os.Stderr and os.Stdout) implement WriteSyncer.
//
// 一个 write 接口 + sync 方法 *os.File， os.Stderr ， os.Stdout
type WriteSyncer interface {
	io.Writer
	Sync() error
}

// AddSync converts an io.Writer to a WriteSyncer. It attempts to be
// intelligent: if the concrete type of the io.Writer implements WriteSyncer,
// we'll use the existing Sync method. If it doesn't, we'll add a no-op Sync.
//
// 将一个 io.writer 变 WriteSyncer，如果是 WriteSyncer 就返回，否则只是假装报了一层 writer wrapper
func AddSync(w io.Writer) WriteSyncer {
	switch w := w.(type) {
	case WriteSyncer:
		return w
	default:
		return writerWrapper{w}
	}
}

// 实现WriteSyncer
//
// 用lock的方式
type lockedWriteSyncer struct {
	sync.Mutex
	ws WriteSyncer
}

// Lock wraps a WriteSyncer in a mutex to make it safe for concurrent use. In
// particular, *os.Files must be locked before use.
//
// 将 ws 包装为 lock-ws
func Lock(ws WriteSyncer) WriteSyncer {
	if _, ok := ws.(*lockedWriteSyncer); ok {
		// no need to layer on another lock
		return ws
	}
	return &lockedWriteSyncer{ws: ws}
}

// 锁 - 写 - 解锁
func (s *lockedWriteSyncer) Write(bs []byte) (int, error) {
	s.Lock()
	n, err := s.ws.Write(bs)
	s.Unlock()
	return n, err
}

// 锁 - 同步 - 解锁
func (s *lockedWriteSyncer) Sync() error {
	s.Lock()
	err := s.ws.Sync()
	s.Unlock()
	return err
}

// 实现了 WriteSyncer 接口，特假
type writerWrapper struct {
	io.Writer
}

func (w writerWrapper) Sync() error {
	return nil
}

// 学习：a是A，多个a也是A
//
// 多 - multiWriteSyncer，也实现了 WriteSyncer
type multiWriteSyncer []WriteSyncer

// NewMultiWriteSyncer creates a WriteSyncer that duplicates its writes
// and sync calls, much like io.MultiWriter.
//
// 用多个 WriteSyncer 搞一个新的 WriteSyncer
func NewMultiWriteSyncer(ws ...WriteSyncer) WriteSyncer {
	if len(ws) == 1 {
		return ws[0]
	}
	// Copy to protect against https://github.com/golang/go/issues/7809
	return multiWriteSyncer(append([]WriteSyncer(nil), ws...))
}

// See https://golang.org/src/io/multi.go
// When not all underlying syncers write the same number of bytes,
// the smallest number is returned even though Write() is called on
// all of them.
//
// 实现 io.write 接口
func (ws multiWriteSyncer) Write(p []byte) (int, error) {
	var writeErr error
	nWritten := 0

	// 遍历 ws
	for _, w := range ws {
		// write
		n, err := w.Write(p)
		// multierr：append error
		writeErr = multierr.Append(writeErr, err)

		if nWritten == 0 && n != 0 {
			// 以写长度为0，新写长度不等于0，将长度更新到已写长度
			nWritten = n
		} else if n < nWritten {
			// 否则，已经有值，但是旧的已写值大于现在的，更新；也就是 nWritten 存 multi-write 的较小值
			nWritten = n
		}
	}
	return nWritten, writeErr
}

// 实现 write-sync 接口
func (ws multiWriteSyncer) Sync() error {
	var err error
	for _, w := range ws {
		err = multierr.Append(err, w.Sync())
	}
	return err
}

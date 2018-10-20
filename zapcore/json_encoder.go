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
	"encoding/base64"
	"encoding/json"
	"go.uber.org/zap/debug"
	"math"
	"sync"
	"time"
	"unicode/utf8"

	"go.uber.org/zap/buffer"
	"go.uber.org/zap/internal/bufferpool"
)

// For JSON-escaping; see jsonEncoder.safeAddString below.
const _hex = "0123456789abcdef"

// json-decoder
// 使用pool加速
// 每一个都要clone一遍，
// 通过 spaced 记录是否添加空格
//   JSON-encoder 没有
//   Console-encoder 有
// 通过 openNamespaces 记录层次
// 通过 *buffer.Buffer 记录底层数据
// 通过 json.encode 处理reflect的数据（这个是在any添加的，所以为了性能考虑，类型确定就使用对应的方法）

var _jsonPool = sync.Pool{New: func() interface{} {
	return &jsonEncoder{}
}}

// 通过pool获取 *jsonEncoder
func getJSONEncoder() *jsonEncoder {
	return _jsonPool.Get().(*jsonEncoder)
}

func putJSONEncoder(enc *jsonEncoder) {
	if enc.reflectBuf != nil {
		enc.reflectBuf.Free()
	}
	enc.EncoderConfig = nil
	enc.buf = nil
	enc.spaced = false
	enc.openNamespaces = 0
	enc.reflectBuf = nil
	enc.reflectEnc = nil
	_jsonPool.Put(enc)
}

// colons 冒号
// commas 逗号

// 实现 ArrayEncoder 接口
// 实现 ObjectEncoder 接口
type jsonEncoder struct {
	*EncoderConfig
	buf *buffer.Buffer

	// 在逗号和冒号后又空格？？
	spaced bool // include spaces after colons and commas

	// 左大括号 { 次数
	openNamespaces int

	// for encoding generic values by reflection
	reflectBuf *buffer.Buffer
	reflectEnc *json.Encoder
}

// NewJSONEncoder creates a fast, low-allocation JSON encoder. The encoder
// appropriately escapes all field keys and values.
//
// Note that the encoder doesn't deduplicate keys, so it's possible to produce
// a message like
//   {"foo":"bar","foo":"baz"}
// This is permitted by the JSON specification, but not encouraged. Many
// libraries will ignore duplicate key-value pairs (typically keeping the last
// pair) when unmarshaling, but users should attempt to avoid adding duplicate
// keys.
func NewJSONEncoder(cfg EncoderConfig) Encoder {
	return newJSONEncoder(cfg, false)
}

func newJSONEncoder(cfg EncoderConfig, spaced bool) *jsonEncoder {
	return &jsonEncoder{
		EncoderConfig: &cfg,
		buf:           bufferpool.Get(), // 使用 buffer 接口的pool 新建一个 Buffer
		spaced:        spaced,
	}
}

// append 开头的表示有key
// add 开头的仅仅是一个值

func (enc *jsonEncoder) AddArray(key string, arr ArrayMarshaler) error {
	debug.Println("jsonEncoder.AddArray", key)
	enc.addKey(key)
	return enc.AppendArray(arr)
}

func (enc *jsonEncoder) AddObject(key string, obj ObjectMarshaler) error {
	debug.Println("jsonEncoder.AddObject", key)
	enc.addKey(key)
	return enc.AppendObject(obj)
}

func (enc *jsonEncoder) AddBinary(key string, val []byte) {
	// 添加 binary
	// bytes -> base64，到 string
	enc.AddString(key, base64.StdEncoding.EncodeToString(val))
}

func (enc *jsonEncoder) AddByteString(key string, val []byte) {
	// bytes 表示的 string
	enc.addKey(key)
	enc.AppendByteString(val)
}

func (enc *jsonEncoder) AddBool(key string, val bool) {
	// 添加 k-v bool，k-v之间有可选的 sep

	debug.Println("jsonEncoder.AddBool", key, val)
	enc.addKey(key)
	enc.AppendBool(val)
}

func (enc *jsonEncoder) AddComplex128(key string, val complex128) {
	enc.addKey(key)
	enc.AppendComplex128(val)
}

func (enc *jsonEncoder) AddDuration(key string, val time.Duration) {
	enc.addKey(key)
	enc.AppendDuration(val)
}

func (enc *jsonEncoder) AddFloat64(key string, val float64) {
	enc.addKey(key)
	enc.AppendFloat64(val)
}

func (enc *jsonEncoder) AddInt64(key string, val int64) {
	enc.addKey(key)
	enc.AppendInt64(val)
}

// reset reflect的buffer，如果为nil，通过pool新建一个，否则，reset一下
func (enc *jsonEncoder) resetReflectBuf() {
	if enc.reflectBuf == nil {
		enc.reflectBuf = bufferpool.Get()
		enc.reflectEnc = json.NewEncoder(enc.reflectBuf)
	} else {
		enc.reflectBuf.Reset()
	}
}

func (enc *jsonEncoder) AddReflected(key string, obj interface{}) error {
	debug.Println("jsonEncoder.AddReflected", key)

	enc.resetReflectBuf()
	err := enc.reflectEnc.Encode(obj) // 通过json自带的encode将 interface转成 string
	if err != nil {
		return err
	}
	enc.reflectBuf.TrimNewline() // 去除换行

	// 添加key 和 value
	enc.addKey(key)
	_, err = enc.buf.Write(enc.reflectBuf.Bytes())

	return err
}

// 添加左大括号 { ，用在namespce中，可以嵌套
func (enc *jsonEncoder) OpenNamespace(key string) {
	enc.addKey(key)
	enc.buf.AppendByte('{')
	enc.openNamespaces++
}

// string
func (enc *jsonEncoder) AddString(key, val string) {
	enc.addKey(key)
	enc.AppendString(val)
}

// time
func (enc *jsonEncoder) AddTime(key string, val time.Time) {
	enc.addKey(key)
	enc.AppendTime(val)
}

// uint64
func (enc *jsonEncoder) AddUint64(key string, val uint64) {
	enc.addKey(key)
	enc.AppendUint64(val)
}

// array
func (enc *jsonEncoder) AppendArray(arr ArrayMarshaler) error {
	enc.addElementSeparator()

	// [] 之间 添加 使用 ArrayMarshaler.MarshalLogArray
	enc.buf.AppendByte('[')
	err := arr.MarshalLogArray(enc)
	enc.buf.AppendByte(']')
	return err
}

// object
func (enc *jsonEncoder) AppendObject(obj ObjectMarshaler) error {
	enc.addElementSeparator()

	// 在 {} 之间添加
	enc.buf.AppendByte('{')
	err := obj.MarshalLogObject(enc)
	enc.buf.AppendByte('}')
	return err
}

// bool
func (enc *jsonEncoder) AppendBool(val bool) {
	enc.addElementSeparator()

	enc.buf.AppendBool(val)
}

// bytes 表示的 字符串
func (enc *jsonEncoder) AppendByteString(val []byte) {
	// 添加 bytes 表示的 string

	enc.addElementSeparator()

	// 将字符串在两个冒号中间写一下
	enc.buf.AppendByte('"')
	enc.safeAddByteString(val)
	enc.buf.AppendByte('"')
}

// 复数
func (enc *jsonEncoder) AppendComplex128(val complex128) {
	enc.addElementSeparator()

	// 复数的实部和虚部
	// Cast to a platform-independent, fixed-size type.
	r, i := float64(real(val)), float64(imag(val))

	// 写：`"r+bi"` 的格式
	enc.buf.AppendByte('"')
	// Because we're always in a quoted string, we can use strconv without
	// special-casing NaN and +/-Inf.
	enc.buf.AppendFloat(r, 64)
	enc.buf.AppendByte('+')
	enc.buf.AppendFloat(i, 64)
	enc.buf.AppendByte('i')
	enc.buf.AppendByte('"')
}

// 时间段
func (enc *jsonEncoder) AppendDuration(val time.Duration) {
	cur := enc.buf.Len()
	enc.EncodeDuration(val, enc)

	// 如果添加后长度没有变，添加时间段的nano表示
	if cur == enc.buf.Len() {
		// User-supplied EncodeDuration is a no-op. Fall back to nanoseconds to keep
		// JSON valid.
		enc.AppendInt64(int64(val))
	}
}

// int64
func (enc *jsonEncoder) AppendInt64(val int64) {
	enc.addElementSeparator()
	enc.buf.AppendInt(val)
}

// reflected
// 待续
func (enc *jsonEncoder) AppendReflected(val interface{}) error {
	enc.resetReflectBuf()
	err := enc.reflectEnc.Encode(val)
	if err != nil {
		return err
	}
	enc.reflectBuf.TrimNewline()
	enc.addElementSeparator()
	_, err = enc.buf.Write(enc.reflectBuf.Bytes())
	return err
}

// string
func (enc *jsonEncoder) AppendString(val string) {
	enc.addElementSeparator()
	enc.buf.AppendByte('"')
	enc.safeAddString(val)
	enc.buf.AppendByte('"')
}

// time
func (enc *jsonEncoder) AppendTime(val time.Time) {
	cur := enc.buf.Len()
	enc.EncodeTime(val, enc)
	// 如果长度没有发生变化，那么就添加纳秒的时间
	if cur == enc.buf.Len() {
		// User-supplied EncodeTime is a no-op. Fall back to nanos since epoch to keep
		// output JSON valid.
		enc.AppendInt64(val.UnixNano())
	}
}

// uint64
func (enc *jsonEncoder) AppendUint64(val uint64) {
	enc.addElementSeparator()
	enc.buf.AppendUint(val)
}

// 基本类型的append，类型转换
func (enc *jsonEncoder) AddComplex64(k string, v complex64) { enc.AddComplex128(k, complex128(v)) }
func (enc *jsonEncoder) AddFloat32(k string, v float32)     { enc.AddFloat64(k, float64(v)) }
func (enc *jsonEncoder) AddInt(k string, v int)             { enc.AddInt64(k, int64(v)) }
func (enc *jsonEncoder) AddInt32(k string, v int32)         { enc.AddInt64(k, int64(v)) }
func (enc *jsonEncoder) AddInt16(k string, v int16)         { enc.AddInt64(k, int64(v)) }
func (enc *jsonEncoder) AddInt8(k string, v int8)           { enc.AddInt64(k, int64(v)) }
func (enc *jsonEncoder) AddUint(k string, v uint)           { enc.AddUint64(k, uint64(v)) }
func (enc *jsonEncoder) AddUint32(k string, v uint32)       { enc.AddUint64(k, uint64(v)) }
func (enc *jsonEncoder) AddUint16(k string, v uint16)       { enc.AddUint64(k, uint64(v)) }
func (enc *jsonEncoder) AddUint8(k string, v uint8)         { enc.AddUint64(k, uint64(v)) }
func (enc *jsonEncoder) AddUintptr(k string, v uintptr)     { enc.AddUint64(k, uint64(v)) }
func (enc *jsonEncoder) AppendComplex64(v complex64)        { enc.AppendComplex128(complex128(v)) }
func (enc *jsonEncoder) AppendFloat64(v float64)            { enc.appendFloat(v, 64) }
func (enc *jsonEncoder) AppendFloat32(v float32)            { enc.appendFloat(float64(v), 32) }
func (enc *jsonEncoder) AppendInt(v int)                    { enc.AppendInt64(int64(v)) }
func (enc *jsonEncoder) AppendInt32(v int32)                { enc.AppendInt64(int64(v)) }
func (enc *jsonEncoder) AppendInt16(v int16)                { enc.AppendInt64(int64(v)) }
func (enc *jsonEncoder) AppendInt8(v int8)                  { enc.AppendInt64(int64(v)) }
func (enc *jsonEncoder) AppendUint(v uint)                  { enc.AppendUint64(uint64(v)) }
func (enc *jsonEncoder) AppendUint32(v uint32)              { enc.AppendUint64(uint64(v)) }
func (enc *jsonEncoder) AppendUint16(v uint16)              { enc.AppendUint64(uint64(v)) }
func (enc *jsonEncoder) AppendUint8(v uint8)                { enc.AppendUint64(uint64(v)) }
func (enc *jsonEncoder) AppendUintptr(v uintptr)            { enc.AppendUint64(uint64(v)) }

// 克隆
func (enc *jsonEncoder) Clone() Encoder {
	// 先clone receiver，然后复制bytes
	clone := enc.clone()
	clone.buf.Write(enc.buf.Bytes())
	return clone
}

// receiver的clone
func (enc *jsonEncoder) clone() *jsonEncoder {
	// 先通过pool获取encoder，然后复制配置（配置是值类型）

	clone := getJSONEncoder()
	clone.EncoderConfig = enc.EncoderConfig
	clone.spaced = enc.spaced
	clone.openNamespaces = enc.openNamespaces
	clone.buf = bufferpool.Get()
	return clone
}

// 这个就是encode位置，调用入口
func (enc *jsonEncoder) EncodeEntry(ent Entry, fields []Field) (*buffer.Buffer, error) {
	debug.Println("jsonEncoder.EncodeEntry")

	// 使用 buffer.Buffer 拼接字符串

	// 每次都clone一个 receiver
	final := enc.clone()

	// 第一个是 {
	final.buf.AppendByte('{')

	// logger-level
	if final.LevelKey != "" {
		// 添加level的key
		final.addKey(final.LevelKey)
		cur := final.buf.Len()
		// 添加level
		final.EncodeLevel(ent.Level, final)
		// 如果添加level后长度没变化，将lecel string一下，再添加
		if cur == final.buf.Len() {
			// User-supplied EncodeLevel was a no-op. Fall back to strings to keep
			// output JSON valid.
			final.AppendString(ent.Level.String())
		}
	}
	// final-time
	if final.TimeKey != "" {
		final.AddTime(final.TimeKey, ent.Time)
	}

	// logger-time
	if ent.LoggerName != "" && final.NameKey != "" {
		final.addKey(final.NameKey)
		cur := final.buf.Len()
		nameEncoder := final.EncodeName

		// if no name encoder provided, fall back to FullNameEncoder for backwards
		// compatibility
		if nameEncoder == nil {
			nameEncoder = FullNameEncoder
		}

		nameEncoder(ent.LoggerName, final)
		if cur == final.buf.Len() {
			// User-supplied EncodeName was a no-op. Fall back to strings to
			// keep output JSON valid.
			final.AppendString(ent.LoggerName)
		}
	}

	// caller
	if ent.Caller.Defined && final.CallerKey != "" {
		final.addKey(final.CallerKey)
		cur := final.buf.Len()
		final.EncodeCaller(ent.Caller, final)
		if cur == final.buf.Len() {
			// User-supplied EncodeCaller was a no-op. Fall back to strings to
			// keep output JSON valid.
			final.AppendString(ent.Caller.String())
		}
	}

	// message
	if final.MessageKey != "" {
		final.addKey(enc.MessageKey)
		final.AppendString(ent.Message)
	}

	if enc.buf.Len() > 0 {
		final.addElementSeparator()
		final.buf.Write(enc.buf.Bytes())
	}

	// fields
	addFields(final, fields)

	// 可能的右括号 }
	final.closeOpenNamespaces()

	// stack
	if ent.Stack != "" && final.StacktraceKey != "" {
		final.AddString(final.StacktraceKey, ent.Stack)
	}

	// 右括号
	final.buf.AppendByte('}')

	// ending，默认 \n
	if final.LineEnding != "" {
		final.buf.AppendString(final.LineEnding)
	} else {
		final.buf.AppendString(DefaultLineEnding)
	}

	ret := final.buf
	putJSONEncoder(final)
	return ret, nil
}

func (enc *jsonEncoder) truncate() {
	enc.buf.Reset()
}

func (enc *jsonEncoder) closeOpenNamespaces() {
	for i := 0; i < enc.openNamespaces; i++ {
		enc.buf.AppendByte('}')
	}
}

// 添加 key
func (enc *jsonEncoder) addKey(key string) {
	//
	enc.addElementSeparator()

	// 在两个双引号之间添加key
	enc.buf.AppendByte('"')
	enc.safeAddString(key)
	enc.buf.AppendByte('"')

	// 添加冒号(key后面需要)
	enc.buf.AppendByte(':')

	// 可选的空格
	if enc.spaced {
		enc.buf.AppendByte(' ')
	}
}

// 在两个元素之间添加逗号，以及可选的空格
// 如果最后一个元素是：
//   {[：最后一个元素，不用逗号
//   :：是冒号，也不需要
//   ,空格：以及有一个逗号了，不需要
//
// 学习：将逗号和空格的添加，加上了判断条件
func (enc *jsonEncoder) addElementSeparator() {
	// 添加 sep
	last := enc.buf.Len() - 1
	if last < 0 {
		return
	}

	// 判断最后一个byte
	switch enc.buf.Bytes()[last] {
	case '{', '[', ':', ',', ' ':
		// 如果
		return
	default:
		// 否则
		// 添加一个,再加一个
		enc.buf.AppendByte(',')

		// 逗号或者冒号后有一个空格
		if enc.spaced {
			enc.buf.AppendByte(' ')
		}
	}
}

func (enc *jsonEncoder) appendFloat(val float64, bitSize int) {
	enc.addElementSeparator()
	switch {
	case math.IsNaN(val):
		enc.buf.AppendString(`"NaN"`)
	case math.IsInf(val, 1):
		enc.buf.AppendString(`"+Inf"`)
	case math.IsInf(val, -1):
		enc.buf.AppendString(`"-Inf"`)
	default:
		enc.buf.AppendFloat(val, bitSize)
	}
}

// safeAddString JSON-escapes a string and appends it to the internal buffer.
// Unlike the standard library's encoder, it doesn't attempt to protect the
// user from browser vulnerabilities or JSONP-related problems.
func (enc *jsonEncoder) safeAddString(s string) {
	for i := 0; i < len(s); {
		if enc.tryAddRuneSelf(s[i]) {
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if enc.tryAddRuneError(r, size) {
			i++
			continue
		}
		enc.buf.AppendString(s[i : i+size])
		i += size
	}
}

// safeAddByteString is no-alloc equivalent of safeAddString(string(s)) for s []byte.
//
// no-alloc 的 safeAddString
// 学习
func (enc *jsonEncoder) safeAddByteString(s []byte) {
	// 遍历 s
	for i := 0; i < len(s); {
		// 将其append rune
		if enc.tryAddRuneSelf(s[i]) {
			i++
			continue
		}
		r, size := utf8.DecodeRune(s[i:])
		if enc.tryAddRuneError(r, size) {
			i++
			continue
		}
		enc.buf.Write(s[i : i+size])
		i += size
	}
}

// tryAddRuneSelf appends b if it is valid UTF-8 character represented in a single byte.
func (enc *jsonEncoder) tryAddRuneSelf(b byte) bool {
	if b >= utf8.RuneSelf {
		return false
	}
	if 0x20 <= b && b != '\\' && b != '"' {
		enc.buf.AppendByte(b)
		return true
	}
	switch b {
	case '\\', '"':
		enc.buf.AppendByte('\\')
		enc.buf.AppendByte(b)
	case '\n':
		enc.buf.AppendByte('\\')
		enc.buf.AppendByte('n')
	case '\r':
		enc.buf.AppendByte('\\')
		enc.buf.AppendByte('r')
	case '\t':
		enc.buf.AppendByte('\\')
		enc.buf.AppendByte('t')
	default:
		// Encode bytes < 0x20, except for the escape sequences above.
		enc.buf.AppendString(`\u00`)
		enc.buf.AppendByte(_hex[b>>4])
		enc.buf.AppendByte(_hex[b&0xF])
	}
	return true
}

func (enc *jsonEncoder) tryAddRuneError(r rune, size int) bool {
	// 当从某语言向Unicode转化时，如果在某语言中没有该字符，得到的将是Unicode的代码“\uffffd”（“\u”表示是Unicode编码，）

	if r == utf8.RuneError && size == 1 {
		enc.buf.AppendString(`\ufffd`)
		return true
	}
	return false
}

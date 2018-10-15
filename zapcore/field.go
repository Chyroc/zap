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
	"bytes"
	"fmt"
	"math"
	"reflect"
	"time"
)

// A FieldType indicates which member of the Field union struct should be used
// and how it should be serialized.
type FieldType uint8

// 字段类型
// 有build-in的，也有slicec和object，还有reflect和namespace
// 属性：Key，Type，Integer，String，Interface
// 实现了 ObjectEncoder 接口，都能添加
// 可以判断是否和另外一个field相等

const (
	// UnknownType is the default field type. Attempting to add it to an encoder will panic.
	UnknownType FieldType = iota // 未使用
	// ArrayMarshalerType indicates that the field carries an ArrayMarshaler.
	ArrayMarshalerType // 存于 Interface
	// ObjectMarshalerType indicates that the field carries an ObjectMarshaler.
	ObjectMarshalerType // 存于 Interface
	// BinaryType indicates that the field carries an opaque binary blob.
	BinaryType // 存于 Interface
	// BoolType indicates that the field carries a bool.
	BoolType // 存于 Integer
	// ByteStringType indicates that the field carries UTF-8 encoded bytes.
	ByteStringType // 存于 Interface
	// Complex128Type indicates that the field carries a complex128.
	Complex128Type // 存于 Interface
	// Complex64Type indicates that the field carries a complex128.
	Complex64Type // 存于 Interface
	// DurationType indicates that the field carries a time.Duration.
	DurationType // 存于 Integer
	// Float64Type indicates that the field carries a float64.
	Float64Type // 存于 Integer
	// Float32Type indicates that the field carries a float32.
	Float32Type // 存于 Integer
	// Int64Type indicates that the field carries an int64.
	Int64Type // 存于 Integer
	// Int32Type indicates that the field carries an int32.
	Int32Type // 存于 Integer
	// Int16Type indicates that the field carries an int16.
	Int16Type // 存于 Integer
	// Int8Type indicates that the field carries an int8.
	Int8Type // 存于 Integer
	// StringType indicates that the field carries a string.
	StringType // 存于 String
	// TimeType indicates that the field carries a time.Time.
	TimeType // 秒存于 Integer，Location存于interface
	// Uint64Type indicates that the field carries a uint64.
	Uint64Type // 存于 Integer
	// Uint32Type indicates that the field carries a uint32.
	Uint32Type // 存于 Integer
	// Uint16Type indicates that the field carries a uint16.
	Uint16Type // 存于 Integer
	// Uint8Type indicates that the field carries a uint8.
	Uint8Type // 存于 Integer
	// UintptrType indicates that the field carries a uintptr.
	UintptrType // 存于 Integer
	// ReflectType indicates that the field carries an interface{}, which should
	// be serialized using reflection.
	ReflectType // 存于 Interface
	// NamespaceType signals the beginning of an isolated namespace. All
	// subsequent fields should be added to the new namespace.
	NamespaceType // 没有valud
	// StringerType indicates that the field carries a fmt.Stringer.
	StringerType // 存于 Interface
	// ErrorType indicates that the field carries an error.
	ErrorType // 存于 Interface
	// SkipType indicates that the field is a no-op.
	SkipType // 没有valud
)

// A Field is a marshaling operation used to add a key-value pair to a logger's
// context. Most fields are lazily marshaled, so it's inexpensive to add fields
// to disabled debug-level log statements.
//
// 添加 k-v 到logger, marshaled 都是懒加载的
type Field struct {
	Key       string
	Type      FieldType
	Integer   int64
	String    string
	Interface interface{} //存储数据
}

// AddTo exports a field through the ObjectEncoder interface. It's primarily
// useful to library authors, and shouldn't be necessary in most applications.
//
// 定义 field 的add方法，将数据变成数据添加到 ObjectEncoder
func (f Field) AddTo(enc ObjectEncoder) {
	var err error

	switch f.Type {
	case ArrayMarshalerType:
		err = enc.AddArray(f.Key, f.Interface.(ArrayMarshaler))
	case ObjectMarshalerType:
		err = enc.AddObject(f.Key, f.Interface.(ObjectMarshaler))
	case BinaryType:
		enc.AddBinary(f.Key, f.Interface.([]byte))
	case BoolType:
		enc.AddBool(f.Key, f.Integer == 1) // 说明 bool 类型的数据也是用 integer 存储
	case ByteStringType:
		enc.AddByteString(f.Key, f.Interface.([]byte))
	case Complex128Type:
		enc.AddComplex128(f.Key, f.Interface.(complex128))
	case Complex64Type:
		enc.AddComplex64(f.Key, f.Interface.(complex64))
	case DurationType:
		enc.AddDuration(f.Key, time.Duration(f.Integer))
	case Float64Type:
		enc.AddFloat64(f.Key, math.Float64frombits(uint64(f.Integer)))
	case Float32Type:
		enc.AddFloat32(f.Key, math.Float32frombits(uint32(f.Integer)))
	case Int64Type:
		enc.AddInt64(f.Key, f.Integer)
	case Int32Type:
		enc.AddInt32(f.Key, int32(f.Integer))
	case Int16Type:
		enc.AddInt16(f.Key, int16(f.Integer))
	case Int8Type:
		enc.AddInt8(f.Key, int8(f.Integer))
	case StringType:
		enc.AddString(f.Key, f.String)
	case TimeType:
		if f.Interface != nil {
			enc.AddTime(f.Key, time.Unix(0, f.Integer).In(f.Interface.(*time.Location)))
		} else {
			// Fall back to UTC if location is nil.
			enc.AddTime(f.Key, time.Unix(0, f.Integer))
		}
	case Uint64Type:
		enc.AddUint64(f.Key, uint64(f.Integer))
	case Uint32Type:
		enc.AddUint32(f.Key, uint32(f.Integer))
	case Uint16Type:
		enc.AddUint16(f.Key, uint16(f.Integer))
	case Uint8Type:
		enc.AddUint8(f.Key, uint8(f.Integer))
	case UintptrType:
		enc.AddUintptr(f.Key, uintptr(f.Integer))
	case ReflectType:
		err = enc.AddReflected(f.Key, f.Interface)
	case NamespaceType:
		enc.OpenNamespace(f.Key)
	case StringerType:
		enc.AddString(f.Key, f.Interface.(fmt.Stringer).String())
	case ErrorType:
		encodeError(f.Key, f.Interface.(error), enc)
	case SkipType:
		break
	default:
		panic(fmt.Sprintf("unknown field type: %v", f))
	}

	if err != nil {
		enc.AddString(fmt.Sprintf("%sError", f.Key), err.Error())
	}
}

// Equals returns whether two fields are equal. For non-primitive types such as
// errors, marshalers, or reflect types, it uses reflect.DeepEqual.
func (f Field) Equals(other Field) bool {
	if f.Type != other.Type {
		return false
	}
	if f.Key != other.Key {
		return false
	}

	switch f.Type {
	case BinaryType, ByteStringType:
		return bytes.Equal(f.Interface.([]byte), other.Interface.([]byte))
	case ArrayMarshalerType, ObjectMarshalerType, ErrorType, ReflectType:
		return reflect.DeepEqual(f.Interface, other.Interface)
	default:
		return f == other
	}
}

// 遍历 field 添加到buffer
func addFields(enc ObjectEncoder, fields []Field) {
	for i := range fields {
		// 疑问：这样可以提供效率？
		fields[i].AddTo(enc)
	}
}

// Package amf0 implements Action Message Format (AMF) encoding and decoding.
// This package provides support for both AMF0 and AMF3 formats as specified
// in the Adobe Flash Video File Format Specification.
package amf0

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"time"
)

// AMF0 Data Types as defined in the AMF0 specification
//
//goland:noinspection ALL
const (
	AMF0TypeNumber      = 0x00
	AMF0TypeBoolean     = 0x01
	AMF0TypeString      = 0x02
	AMF0TypeObject      = 0x03
	AMF0TypeMovieClip   = 0x04 // Reserved, not supported
	AMF0TypeNull        = 0x05
	AMF0TypeUndefined   = 0x06
	AMF0TypeReference   = 0x07
	AMF0TypeEcmaArray   = 0x08
	AMF0TypeObjectEnd   = 0x09
	AMF0TypeStrictArray = 0x0A
	AMF0TypeDate        = 0x0B
	AMF0TypeLongString  = 0x0C
	AMF0TypeUnsupported = 0x0D
	AMF0TypeRecordset   = 0x0E // Reserved, not supported
	AMF0TypeXMLDocument = 0x0F
	AMF0TypeTypedObject = 0x10
	AMF0TypeAVMPlus     = 0x11 // Switch to AMF3
)

// AMF0Value represents any AMF0 value
//
//goland:noinspection ALL
type AMF0Value interface {
	Type() byte
}

// AMF0Number represents an AMF0 Number (IEEE-754 double precision floating point)
//
//goland:noinspection ALL
type AMF0Number float64

func (n AMF0Number) Type() byte { return AMF0TypeNumber }

// AMF0Boolean represents an AMF0 Boolean
//
//goland:noinspection ALL
type AMF0Boolean bool

func (b AMF0Boolean) Type() byte { return AMF0TypeBoolean }

// AMF0String represents an AMF0 String
//
//goland:noinspection ALL
type AMF0String string

func (s AMF0String) Type() byte { return AMF0TypeString }

// AMF0Object represents an AMF0 Object
//
//goland:noinspection ALL
type AMF0Object map[string]AMF0Value

func (o AMF0Object) Type() byte { return AMF0TypeObject }

// AMF0Null represents an AMF0 Null
//
//goland:noinspection ALL
type AMF0Null struct{}

func (n AMF0Null) Type() byte { return AMF0TypeNull }

// AMF0Undefined represents an AMF0 Undefined
//
//goland:noinspection ALL
type AMF0Undefined struct{}

func (u AMF0Undefined) Type() byte { return AMF0TypeUndefined }

// AMF0EcmaArray represents an AMF0 ECMA Array
//
//goland:noinspection ALL
type AMF0EcmaArray struct {
	Count      uint32
	Properties map[string]AMF0Value
}

func (a AMF0EcmaArray) Type() byte { return AMF0TypeEcmaArray }

// AMF0StrictArray represents an AMF0 Strict Array
//
//goland:noinspection ALL
type AMF0StrictArray []AMF0Value

func (a AMF0StrictArray) Type() byte { return AMF0TypeStrictArray }

// AMF0Date represents an AMF0 Date
//
//goland:noinspection ALL
type AMF0Date struct {
	Milliseconds float64
	Timezone     int16
}

func (d AMF0Date) Type() byte { return AMF0TypeDate }

// Time returns the time.Time representation of the AMF0Date
func (d AMF0Date) Time() time.Time {
	return time.Unix(int64(d.Milliseconds/1000), int64(d.Milliseconds)%1000*1000000)
}

// AMF0LongString represents an AMF0 Long String
//
//goland:noinspection ALL
type AMF0LongString string

func (s AMF0LongString) Type() byte { return AMF0TypeLongString }

// AMF0TypedObject represents an AMF0 Typed Object
//
//goland:noinspection ALL
type AMF0TypedObject struct {
	ClassName  string
	Properties map[string]AMF0Value
}

func (o AMF0TypedObject) Type() byte { return AMF0TypeTypedObject }

// AMF0Encoder encodes Go values to AMF0 format
//
//goland:noinspection ALL
type AMF0Encoder struct {
	writer io.Writer
}

// NewAMF0Encoder creates a new AMF0 encoder
func NewAMF0Encoder(w io.Writer) *AMF0Encoder {
	return &AMF0Encoder{writer: w}
}

// Encode encodes a value to AMF0 format
func (e *AMF0Encoder) Encode(value interface{}) error {
	return e.encodeValue(value)
}

// encodeValue encodes any Go value to AMF0
func (e *AMF0Encoder) encodeValue(value interface{}) error {
	switch v := value.(type) {
	case float64:
		return e.encodeNumber(v)
	case float32:
		return e.encodeNumber(float64(v))
	case int:
		return e.encodeNumber(float64(v))
	case int8:
		return e.encodeNumber(float64(v))
	case int16:
		return e.encodeNumber(float64(v))
	case int32:
		return e.encodeNumber(float64(v))
	case int64:
		return e.encodeNumber(float64(v))
	case uint:
		return e.encodeNumber(float64(v))
	case uint8:
		return e.encodeNumber(float64(v))
	case uint16:
		return e.encodeNumber(float64(v))
	case uint32:
		return e.encodeNumber(float64(v))
	case uint64:
		return e.encodeNumber(float64(v))
	case bool:
		return e.encodeBoolean(v)
	case string:
		return e.encodeString(v)
	case map[string]interface{}:
		return e.encodeObject(v)
	case []interface{}:
		return e.encodeStrictArray(v)
	case nil:
		return e.encodeNull()
	case AMF0Value:
		return e.encodeAMF0Value(v)
	default:
		return fmt.Errorf("unsupported type: %T", value)
	}
}

// encodeAMF0Value encodes an AMF0Value
func (e *AMF0Encoder) encodeAMF0Value(value AMF0Value) error {
	switch v := value.(type) {
	case AMF0Number:
		return e.encodeNumber(float64(v))
	case AMF0Boolean:
		return e.encodeBoolean(bool(v))
	case AMF0String:
		return e.encodeString(string(v))
	case AMF0Object:
		obj := make(map[string]interface{})
		for k, val := range v {
			obj[k] = val
		}
		return e.encodeObject(obj)
	case AMF0Null:
		return e.encodeNull()
	case AMF0Undefined:
		return e.encodeUndefined()
	case AMF0EcmaArray:
		return e.encodeEcmaArray(v)
	case AMF0StrictArray:
		arr := make([]interface{}, len(v))
		for i, val := range v {
			arr[i] = val
		}
		return e.encodeStrictArray(arr)
	case AMF0Date:
		return e.encodeDate(v)
	case AMF0LongString:
		return e.encodeLongString(string(v))
	case AMF0TypedObject:
		return e.encodeTypedObject(v)
	default:
		return fmt.Errorf("unsupported AMF0 type: %T", value)
	}
}

// encodeNumber encodes a number to AMF0
func (e *AMF0Encoder) encodeNumber(value float64) error {
	if err := e.writeByte(AMF0TypeNumber); err != nil {
		return err
	}
	return binary.Write(e.writer, binary.BigEndian, math.Float64bits(value))
}

// encodeBoolean encodes a boolean to AMF0
func (e *AMF0Encoder) encodeBoolean(value bool) error {
	if err := e.writeByte(AMF0TypeBoolean); err != nil {
		return err
	}
	if value {
		return e.writeByte(1)
	}
	return e.writeByte(0)
}

// encodeString encodes a string to AMF0
func (e *AMF0Encoder) encodeString(value string) error {
	if len(value) > 65535 {
		return e.encodeLongString(value)
	}

	if err := e.writeByte(AMF0TypeString); err != nil {
		return err
	}
	return e.writeUTF8(value, false)
}

// encodeLongString encodes a long string to AMF0
func (e *AMF0Encoder) encodeLongString(value string) error {
	if err := e.writeByte(AMF0TypeLongString); err != nil {
		return err
	}
	return e.writeUTF8(value, true)
}

// encodeObject encodes an object to AMF0
func (e *AMF0Encoder) encodeObject(value map[string]interface{}) error {
	if err := e.writeByte(AMF0TypeObject); err != nil {
		return err
	}

	for key, val := range value {
		// Write a property name (without a type marker)
		if err := e.writeUTF8(key, false); err != nil {
			return err
		}
		// Write property value
		if err := e.encodeValue(val); err != nil {
			return err
		}
	}

	// Write object end marker
	if err := e.writeUTF8("", false); err != nil {
		return err
	}
	return e.writeByte(AMF0TypeObjectEnd)
}

// encodeTypedObject encodes a typed object to AMF0
func (e *AMF0Encoder) encodeTypedObject(value AMF0TypedObject) error {
	if err := e.writeByte(AMF0TypeTypedObject); err != nil {
		return err
	}

	// Write class name
	if err := e.writeUTF8(value.ClassName, false); err != nil {
		return err
	}

	// Write properties
	for key, val := range value.Properties {
		if err := e.writeUTF8(key, false); err != nil {
			return err
		}
		if err := e.encodeAMF0Value(val); err != nil {
			return err
		}
	}

	// Write object end marker
	if err := e.writeUTF8("", false); err != nil {
		return err
	}
	return e.writeByte(AMF0TypeObjectEnd)
}

// encodeEcmaArray encodes an ECMA array to AMF0
func (e *AMF0Encoder) encodeEcmaArray(value AMF0EcmaArray) error {
	if err := e.writeByte(AMF0TypeEcmaArray); err != nil {
		return err
	}

	// Write count
	if err := binary.Write(e.writer, binary.BigEndian, value.Count); err != nil {
		return err
	}

	// Write properties
	for key, val := range value.Properties {
		if err := e.writeUTF8(key, false); err != nil {
			return err
		}
		if err := e.encodeAMF0Value(val); err != nil {
			return err
		}
	}

	// Write object end marker
	if err := e.writeUTF8("", false); err != nil {
		return err
	}
	return e.writeByte(AMF0TypeObjectEnd)
}

// encodeStrictArray encodes a strict array to AMF0
func (e *AMF0Encoder) encodeStrictArray(value []interface{}) error {
	if err := e.writeByte(AMF0TypeStrictArray); err != nil {
		return err
	}

	// Write count
	if err := binary.Write(e.writer, binary.BigEndian, uint32(len(value))); err != nil {
		return err
	}

	// Write elements
	for _, val := range value {
		if err := e.encodeValue(val); err != nil {
			return err
		}
	}

	return nil
}

// encodeDate encodes a date to AMF0
func (e *AMF0Encoder) encodeDate(value AMF0Date) error {
	if err := e.writeByte(AMF0TypeDate); err != nil {
		return err
	}

	// Write milliseconds
	if err := binary.Write(e.writer, binary.BigEndian, math.Float64bits(value.Milliseconds)); err != nil {
		return err
	}

	// Write timezone (should be 0x0000, according to spec)
	return binary.Write(e.writer, binary.BigEndian, value.Timezone)
}

// encodeNull encodes null to AMF0
func (e *AMF0Encoder) encodeNull() error {
	return e.writeByte(AMF0TypeNull)
}

// encodeUndefined encodes undefined to AMF0
func (e *AMF0Encoder) encodeUndefined() error {
	return e.writeByte(AMF0TypeUndefined)
}

// writeUTF8 writes a UTF-8 string with length prefix
func (e *AMF0Encoder) writeUTF8(s string, longString bool) error {
	data := []byte(s)

	if longString {
		// Long string uses 4-byte length
		if err := binary.Write(e.writer, binary.BigEndian, uint32(len(data))); err != nil {
			return err
		}
	} else {
		// Regular string uses 2-byte length
		if len(data) > 65535 {
			return fmt.Errorf("string too long for UTF-8: %d bytes", len(data))
		}
		if err := binary.Write(e.writer, binary.BigEndian, uint16(len(data))); err != nil {
			return err
		}
	}

	_, err := e.writer.Write(data)
	return err
}

// writeByte writes a single byte
func (e *AMF0Encoder) writeByte(b byte) error {
	_, err := e.writer.Write([]byte{b})
	return err
}

// AMF0Decoder decodes AMF0 format to Go values
//
//goland:noinspection ALL
type AMF0Decoder struct {
	reader io.Reader
}

// NewAMF0Decoder creates a new AMF0 decoder
func NewAMF0Decoder(r io.Reader) *AMF0Decoder {
	return &AMF0Decoder{reader: r}
}

// Decode decodes a value from AMF0 format
func (d *AMF0Decoder) Decode() (interface{}, error) {
	return d.decodeValue()
}

// DecodeAMF0 decodes a value as AMF0Value
func (d *AMF0Decoder) DecodeAMF0() (AMF0Value, error) {
	return d.decodeAMF0Value()
}

// decodeValue decodes any AMF0 value to Go types
func (d *AMF0Decoder) decodeValue() (interface{}, error) {
	typeByte, err := d.readByte()
	if err != nil {
		return nil, err
	}

	switch typeByte {
	case AMF0TypeNumber:
		return d.decodeNumber()
	case AMF0TypeBoolean:
		return d.decodeBoolean()
	case AMF0TypeString:
		return d.decodeString()
	case AMF0TypeObject:
		return d.decodeObject()
	case AMF0TypeNull:
		return nil, nil
	case AMF0TypeUndefined:
		return nil, nil // or could return a special undefined value
	case AMF0TypeEcmaArray:
		return d.decodeEcmaArray()
	case AMF0TypeStrictArray:
		return d.decodeStrictArray()
	case AMF0TypeDate:
		date, err := d.decodeDateValue()
		if err != nil {
			return nil, err
		}
		return date.Time(), nil
	case AMF0TypeLongString:
		return d.decodeLongString()
	case AMF0TypeTypedObject:
		return d.decodeTypedObject()
	default:
		return nil, fmt.Errorf("unsupported AMF0 type: 0x%02X", typeByte)
	}
}

// decodeAMF0Value decodes any AMF0 value to AMF0Value types
func (d *AMF0Decoder) decodeAMF0Value() (AMF0Value, error) {
	typeByte, err := d.readByte()
	if err != nil {
		return nil, err
	}

	switch typeByte {
	case AMF0TypeNumber:
		val, err := d.decodeNumber()
		if err != nil {
			return nil, err
		}
		return AMF0Number(val), nil
	case AMF0TypeBoolean:
		val, err := d.decodeBoolean()
		if err != nil {
			return nil, err
		}
		return AMF0Boolean(val), nil
	case AMF0TypeString:
		val, err := d.decodeString()
		if err != nil {
			return nil, err
		}
		return AMF0String(val), nil
	case AMF0TypeObject:
		val, err := d.decodeObjectValue()
		if err != nil {
			return nil, err
		}
		return val, nil
	case AMF0TypeNull:
		return AMF0Null{}, nil
	case AMF0TypeUndefined:
		return AMF0Undefined{}, nil
	case AMF0TypeEcmaArray:
		return d.decodeEcmaArrayValue()
	case AMF0TypeStrictArray:
		return d.decodeStrictArrayValue()
	case AMF0TypeDate:
		return d.decodeDateValue()
	case AMF0TypeLongString:
		val, err := d.decodeLongString()
		if err != nil {
			return nil, err
		}
		return AMF0LongString(val), nil
	case AMF0TypeTypedObject:
		return d.decodeTypedObjectValue()
	default:
		return nil, fmt.Errorf("unsupported AMF0 type: 0x%02X", typeByte)
	}
}

// decodeNumber decodes a number from AMF0
func (d *AMF0Decoder) decodeNumber() (float64, error) {
	var bits uint64
	if err := binary.Read(d.reader, binary.BigEndian, &bits); err != nil {
		return 0, err
	}
	return math.Float64frombits(bits), nil
}

// decodeBoolean decodes a boolean from AMF0
func (d *AMF0Decoder) decodeBoolean() (bool, error) {
	b, err := d.readByte()
	if err != nil {
		return false, err
	}
	return b != 0, nil
}

// decodeString decodes a string from AMF0
func (d *AMF0Decoder) decodeString() (string, error) {
	return d.readUTF8(false)
}

// decodeLongString decodes a long string from AMF0
func (d *AMF0Decoder) decodeLongString() (string, error) {
	return d.readUTF8(true)
}

// decodeObject decodes an object from AMF0
func (d *AMF0Decoder) decodeObject() (map[string]interface{}, error) {
	obj := make(map[string]interface{})

	for {
		key, err := d.readUTF8(false)
		if err != nil {
			return nil, err
		}

		if key == "" {
			// Check for object end marker
			marker, err := d.readByte()
			if err != nil {
				return nil, err
			}
			if marker == AMF0TypeObjectEnd {
				break
			}
			return nil, fmt.Errorf("expected object end marker, got 0x%02X", marker)
		}

		value, err := d.decodeValue()
		if err != nil {
			return nil, err
		}

		obj[key] = value
	}

	return obj, nil
}

// decodeObjectValue decodes an object from AMF0 as AMF0Object
func (d *AMF0Decoder) decodeObjectValue() (AMF0Object, error) {
	obj := make(AMF0Object)

	for {
		key, err := d.readUTF8(false)
		if err != nil {
			return nil, err
		}

		if key == "" {
			// Check for object end marker
			marker, err := d.readByte()
			if err != nil {
				return nil, err
			}
			if marker == AMF0TypeObjectEnd {
				break
			}
			return nil, fmt.Errorf("expected object end marker, got 0x%02X", marker)
		}

		value, err := d.decodeAMF0Value()
		if err != nil {
			return nil, err
		}

		obj[key] = value
	}

	return obj, nil
}

// decodeEcmaArray decodes an ECMA array from AMF0
func (d *AMF0Decoder) decodeEcmaArray() (map[string]interface{}, error) {
	var count uint32
	if err := binary.Read(d.reader, binary.BigEndian, &count); err != nil {
		return nil, err
	}

	obj := make(map[string]interface{})

	for {
		key, err := d.readUTF8(false)
		if err != nil {
			return nil, err
		}

		if key == "" {
			// Check for object end marker
			marker, err := d.readByte()
			if err != nil {
				return nil, err
			}
			if marker == AMF0TypeObjectEnd {
				break
			}
			return nil, fmt.Errorf("expected object end marker, got 0x%02X", marker)
		}

		value, err := d.decodeValue()
		if err != nil {
			return nil, err
		}

		obj[key] = value
	}

	return obj, nil
}

// decodeEcmaArrayValue decodes an ECMA array from AMF0 as AMF0EcmaArray
func (d *AMF0Decoder) decodeEcmaArrayValue() (AMF0EcmaArray, error) {
	var count uint32
	if err := binary.Read(d.reader, binary.BigEndian, &count); err != nil {
		return AMF0EcmaArray{}, err
	}

	properties := make(map[string]AMF0Value)

	for {
		key, err := d.readUTF8(false)
		if err != nil {
			return AMF0EcmaArray{}, err
		}

		if key == "" {
			// Check for object end marker
			marker, err := d.readByte()
			if err != nil {
				return AMF0EcmaArray{}, err
			}
			if marker == AMF0TypeObjectEnd {
				break
			}
			return AMF0EcmaArray{}, fmt.Errorf("expected object end marker, got 0x%02X", marker)
		}

		value, err := d.decodeAMF0Value()
		if err != nil {
			return AMF0EcmaArray{}, err
		}

		properties[key] = value
	}

	return AMF0EcmaArray{
		Count:      count,
		Properties: properties,
	}, nil
}

// decodeStrictArray decodes a strict array from AMF0
func (d *AMF0Decoder) decodeStrictArray() ([]interface{}, error) {
	var count uint32
	if err := binary.Read(d.reader, binary.BigEndian, &count); err != nil {
		return nil, err
	}

	arr := make([]interface{}, count)
	for i := uint32(0); i < count; i++ {
		value, err := d.decodeValue()
		if err != nil {
			return nil, err
		}
		arr[i] = value
	}

	return arr, nil
}

// decodeStrictArrayValue decodes a strict array from AMF0 as AMF0StrictArray
func (d *AMF0Decoder) decodeStrictArrayValue() (AMF0StrictArray, error) {
	var count uint32
	if err := binary.Read(d.reader, binary.BigEndian, &count); err != nil {
		return nil, err
	}

	arr := make(AMF0StrictArray, count)
	for i := uint32(0); i < count; i++ {
		value, err := d.decodeAMF0Value()
		if err != nil {
			return nil, err
		}
		arr[i] = value
	}

	return arr, nil
}

// decodeDateValue decodes a date from AMF0
func (d *AMF0Decoder) decodeDateValue() (AMF0Date, error) {
	var millisBits uint64
	if err := binary.Read(d.reader, binary.BigEndian, &millisBits); err != nil {
		return AMF0Date{}, err
	}

	var timezone int16
	if err := binary.Read(d.reader, binary.BigEndian, &timezone); err != nil {
		return AMF0Date{}, err
	}

	return AMF0Date{
		Milliseconds: math.Float64frombits(millisBits),
		Timezone:     timezone,
	}, nil
}

// decodeTypedObject decodes a typed object from AMF0
func (d *AMF0Decoder) decodeTypedObject() (map[string]interface{}, error) {
	className, err := d.readUTF8(false)
	if err != nil {
		return nil, err
	}

	obj := make(map[string]interface{})
	obj["__className"] = className

	for {
		key, err := d.readUTF8(false)
		if err != nil {
			return nil, err
		}

		if key == "" {
			// Check for object end marker
			marker, err := d.readByte()
			if err != nil {
				return nil, err
			}
			if marker == AMF0TypeObjectEnd {
				break
			}
			return nil, fmt.Errorf("expected object end marker, got 0x%02X", marker)
		}

		value, err := d.decodeValue()
		if err != nil {
			return nil, err
		}

		obj[key] = value
	}

	return obj, nil
}

// decodeTypedObjectValue decodes a typed object from AMF0 as AMF0TypedObject
func (d *AMF0Decoder) decodeTypedObjectValue() (AMF0TypedObject, error) {
	className, err := d.readUTF8(false)
	if err != nil {
		return AMF0TypedObject{}, err
	}

	properties := make(map[string]AMF0Value)

	for {
		key, err := d.readUTF8(false)
		if err != nil {
			return AMF0TypedObject{}, err
		}

		if key == "" {
			// Check for object end marker
			marker, err := d.readByte()
			if err != nil {
				return AMF0TypedObject{}, err
			}
			if marker == AMF0TypeObjectEnd {
				break
			}
			return AMF0TypedObject{}, fmt.Errorf("expected object end marker, got 0x%02X", marker)
		}

		value, err := d.decodeAMF0Value()
		if err != nil {
			return AMF0TypedObject{}, err
		}

		properties[key] = value
	}

	return AMF0TypedObject{
		ClassName:  className,
		Properties: properties,
	}, nil
}

// readUTF8 reads a UTF-8 string with length prefix
func (d *AMF0Decoder) readUTF8(longString bool) (string, error) {
	var length uint32

	if longString {
		// Long string uses 4-byte length
		if err := binary.Read(d.reader, binary.BigEndian, &length); err != nil {
			return "", err
		}
	} else {
		// Regular string uses 2-byte length
		var shortLength uint16
		if err := binary.Read(d.reader, binary.BigEndian, &shortLength); err != nil {
			return "", err
		}
		length = uint32(shortLength)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(d.reader, data); err != nil {
		return "", err
	}

	return string(data), nil
}

// readByte reads a single byte
func (d *AMF0Decoder) readByte() (byte, error) {
	data := make([]byte, 1)
	if _, err := io.ReadFull(d.reader, data); err != nil {
		return 0, err
	}
	return data[0], nil
}

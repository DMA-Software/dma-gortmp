// Package amf3 provides encoding and decoding of Action Message Format 3 (AMF3) data.
// AMF3 is a compact binary format used by Adobe Flash for serializing ActionScript objects.
package amf3

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"time"
)

// AMF3 Data Types as defined in the AMF3 specification
//
//goland:noinspection ALL
const (
	AMF3TypeUndefined    = 0x00
	AMF3TypeNull         = 0x01
	AMF3TypeFalse        = 0x02
	AMF3TypeTrue         = 0x03
	AMF3TypeInteger      = 0x04
	AMF3TypeDouble       = 0x05
	AMF3TypeString       = 0x06
	AMF3TypeXMLDocument  = 0x07
	AMF3TypeDate         = 0x08
	AMF3TypeArray        = 0x09
	AMF3TypeObject       = 0x0A
	AMF3TypeXML          = 0x0B
	AMF3TypeByteArray    = 0x0C
	AMF3TypeVectorInt    = 0x0D
	AMF3TypeVectorUInt   = 0x0E
	AMF3TypeVectorDouble = 0x0F
	AMF3TypeVectorObject = 0x10
	AMF3TypeDictionary   = 0x11
)

// AMF3Value represents any AMF3 value
//
//goland:noinspection ALL
type AMF3Value interface {
	Type() byte
}

// AMF3Undefined represents an AMF3 Undefined value
//
//goland:noinspection ALL
type AMF3Undefined struct{}

func (u AMF3Undefined) Type() byte { return AMF3TypeUndefined }

// AMF3Null represents an AMF3 Null value
//
//goland:noinspection ALL
type AMF3Null struct{}

func (n AMF3Null) Type() byte { return AMF3TypeNull }

// AMF3Boolean represents an AMF3 Boolean value (true/false)
//
//goland:noinspection ALL
type AMF3Boolean bool

func (b AMF3Boolean) Type() byte {
	if b {
		return AMF3TypeTrue
	}
	return AMF3TypeFalse
}

// AMF3Integer represents an AMF3 Integer (29-bit signed integer)
//
//goland:noinspection ALL
type AMF3Integer int32

func (i AMF3Integer) Type() byte { return AMF3TypeInteger }

// AMF3Double represents an AMF3 Double (IEEE-754 double precision floating point)
//
//goland:noinspection ALL
type AMF3Double float64

func (d AMF3Double) Type() byte { return AMF3TypeDouble }

// AMF3String represents an AMF3 String with reference support
//
//goland:noinspection ALL
type AMF3String struct {
	Value string
	Ref   int // Reference table index, -1 if not referenced
}

func (s AMF3String) Type() byte { return AMF3TypeString }

// AMF3Date represents an AMF3 Date
//
//goland:noinspection ALL
type AMF3Date struct {
	Time time.Time
	Ref  int // Reference table index, -1 if not referenced
}

func (d AMF3Date) Type() byte { return AMF3TypeDate }

// AMF3Array represents an AMF3 Array with dense and associative parts
//
//goland:noinspection ALL
type AMF3Array struct {
	Dense       []AMF3Value          // Dense array elements
	Associative map[string]AMF3Value // Associative properties
	Ref         int                  // Reference table index, -1 if not referenced
}

func (a AMF3Array) Type() byte { return AMF3TypeArray }

// AMF3Object represents an AMF3 Object with traits and properties
//
//goland:noinspection ALL
type AMF3Object struct {
	Traits     *AMF3Traits          // Object traits definition
	Properties map[string]AMF3Value // Object properties
	Ref        int                  // Reference table index, -1 if not referenced
}

func (o AMF3Object) Type() byte { return AMF3TypeObject }

// AMF3Traits represents object traits (class definition) in AMF3
//
//goland:noinspection ALL
type AMF3Traits struct {
	ClassName      string   // Class name (empty string for anonymous object)
	Dynamic        bool     // Whether object accepts dynamic properties
	Externalizable bool     // Whether object is externalizable
	Properties     []string // List of sealed property names
	Ref            int      // Traits reference table index, -1 if not referenced
}

// Reference tables for AMF3 encoding/decoding
// These are used to avoid duplicating strings, objects, and traits
type referenceContext struct {
	stringTable []string      // String reference table
	objectTable []AMF3Value   // Object reference table
	traitsTable []*AMF3Traits // Traits reference table
}

// AMF3Encoder provides encoding of Go values to AMF3 format
//
//goland:noinspection ALL
type AMF3Encoder struct {
	writer io.Writer
	refs   referenceContext
}

// NewAMF3Encoder creates a new AMF3 encoder that writes to the provided writer
func NewAMF3Encoder(w io.Writer) *AMF3Encoder {
	return &AMF3Encoder{
		writer: w,
		refs:   referenceContext{},
	}
}

// Encode encodes a Go value to AMF3 format
func (e *AMF3Encoder) Encode(value interface{}) error {
	return e.encodeValue(value)
}

// encodeValue encodes any Go value to AMF3 format
func (e *AMF3Encoder) encodeValue(value interface{}) error {
	switch v := value.(type) {
	case nil:
		return e.encodeNull()
	case bool:
		return e.encodeBoolean(v)
	case int:
		return e.encodeInteger(int32(v))
	case int8:
		return e.encodeInteger(int32(v))
	case int16:
		return e.encodeInteger(int32(v))
	case int32:
		return e.encodeInteger(v)
	case int64:
		// AMF3 integers are 29-bit, use double for larger values
		if v >= -268435456 && v <= 268435455 {
			return e.encodeInteger(int32(v))
		}
		return e.encodeDouble(float64(v))
	case uint:
		return e.encodeInteger(int32(v))
	case uint8:
		return e.encodeInteger(int32(v))
	case uint16:
		return e.encodeInteger(int32(v))
	case uint32:
		if v <= 268435455 {
			return e.encodeInteger(int32(v))
		}
		return e.encodeDouble(float64(v))
	case uint64:
		return e.encodeDouble(float64(v))
	case float32:
		return e.encodeDouble(float64(v))
	case float64:
		return e.encodeDouble(v)
	case string:
		return e.encodeString(v)
	case time.Time:
		return e.encodeDate(v)
	case []interface{}:
		return e.encodeArray(v, nil)
	case map[string]interface{}:
		return e.encodeObject(v)
	case AMF3Value:
		return e.encodeAMF3Value(v)
	default:
		return fmt.Errorf("unsupported type for AMF3 encoding: %T", value)
	}
}

// encodeAMF3Value encodes an AMF3Value to AMF3 format
func (e *AMF3Encoder) encodeAMF3Value(value AMF3Value) error {
	switch v := value.(type) {
	case AMF3Undefined:
		return e.encodeUndefined()
	case AMF3Null:
		return e.encodeNull()
	case AMF3Boolean:
		return e.encodeBoolean(bool(v))
	case AMF3Integer:
		return e.encodeInteger(int32(v))
	case AMF3Double:
		return e.encodeDouble(float64(v))
	case AMF3String:
		return e.encodeStringValue(v)
	case AMF3Date:
		return e.encodeDateValue(v)
	case AMF3Array:
		return e.encodeArrayValue(v)
	case AMF3Object:
		return e.encodeObjectValue(v)
	default:
		return fmt.Errorf("unsupported AMF3Value type: %T", value)
	}
}

// encodeUndefined encodes an undefined value
func (e *AMF3Encoder) encodeUndefined() error {
	return e.writeByte(AMF3TypeUndefined)
}

// encodeNull encodes a null value
func (e *AMF3Encoder) encodeNull() error {
	return e.writeByte(AMF3TypeNull)
}

// encodeBoolean encodes a boolean value
func (e *AMF3Encoder) encodeBoolean(value bool) error {
	if value {
		return e.writeByte(AMF3TypeTrue)
	}
	return e.writeByte(AMF3TypeFalse)
}

// encodeInteger encodes a 29-bit signed integer using variable-length encoding
func (e *AMF3Encoder) encodeInteger(value int32) error {
	if err := e.writeByte(AMF3TypeInteger); err != nil {
		return err
	}
	return e.writeU29(uint32(value))
}

// encodeDouble encodes an IEEE-754 double precision floating point number
func (e *AMF3Encoder) encodeDouble(value float64) error {
	if err := e.writeByte(AMF3TypeDouble); err != nil {
		return err
	}
	bits := math.Float64bits(value)
	return binary.Write(e.writer, binary.BigEndian, bits)
}

// encodeString encodes a string with reference support
func (e *AMF3Encoder) encodeString(value string) error {
	if err := e.writeByte(AMF3TypeString); err != nil {
		return err
	}
	return e.writeStringWithReference(value)
}

// encodeStringValue encodes an AMF3String value
func (e *AMF3Encoder) encodeStringValue(value AMF3String) error {
	if err := e.writeByte(AMF3TypeString); err != nil {
		return err
	}
	return e.writeStringWithReference(value.Value)
}

// encodeDate encodes a date value
func (e *AMF3Encoder) encodeDate(value time.Time) error {
	if err := e.writeByte(AMF3TypeDate); err != nil {
		return err
	}
	// Check if this date is already in the reference table
	for i, obj := range e.refs.objectTable {
		if dateObj, ok := obj.(AMF3Date); ok && dateObj.Time.Equal(value) {
			// Write reference (index << 1)
			return e.writeU29(uint32(i << 1))
		}
	}

	// Add to the reference table and write as a new object (1)
	e.refs.objectTable = append(e.refs.objectTable, AMF3Date{Time: value, Ref: len(e.refs.objectTable)})
	if err := e.writeU29(1); err != nil {
		return err
	}

	// Write timestamp as milliseconds since Unix epoch
	timestamp := float64(value.UnixNano()) / 1e6
	bits := math.Float64bits(timestamp)
	return binary.Write(e.writer, binary.BigEndian, bits)
}

// encodeDateValue encodes an AMF3Date value
func (e *AMF3Encoder) encodeDateValue(value AMF3Date) error {
	if err := e.writeByte(AMF3TypeDate); err != nil {
		return err
	}
	return e.encodeDate(value.Time)
}

// encodeArray encodes an array with dense and associative parts
func (e *AMF3Encoder) encodeArray(dense []interface{}, associative map[string]interface{}) error {
	if err := e.writeByte(AMF3TypeArray); err != nil {
		return err
	}

	// For now, encode as a new array (not implementing reference check for simplicity)
	// Real implementation would check the reference table here
	if err := e.writeU29(uint32(len(dense)<<1) | 1); err != nil {
		return err
	}

	// Write associative part (key-value pairs, empty key terminates)
	if associative != nil {
		for key, val := range associative {
			if err := e.writeStringWithReference(key); err != nil {
				return err
			}
			if err := e.encodeValue(val); err != nil {
				return err
			}
		}
	}
	// Write empty string to terminate the associative part
	if err := e.writeStringWithReference(""); err != nil {
		return err
	}

	// Write dense part
	for _, val := range dense {
		if err := e.encodeValue(val); err != nil {
			return err
		}
	}

	return nil
}

// encodeArrayValue encodes an AMF3Array value
func (e *AMF3Encoder) encodeArrayValue(value AMF3Array) error {
	if err := e.writeByte(AMF3TypeArray); err != nil {
		return err
	}

	// Convert AMF3Value slice to interface{} slice for dense part
	dense := make([]interface{}, len(value.Dense))
	for i, v := range value.Dense {
		dense[i] = v
	}

	// Convert associative part
	associative := make(map[string]interface{})
	for k, v := range value.Associative {
		associative[k] = v
	}

	return e.encodeArray(dense, associative)
}

// encodeObject encodes an object (anonymous object with dynamic properties)
func (e *AMF3Encoder) encodeObject(value map[string]interface{}) error {
	if err := e.writeByte(AMF3TypeObject); err != nil {
		return err
	}

	// Encode as anonymous dynamic object
	// Traits: (0 << 4) | (1 << 3) | (0 << 2) | (1 << 1) | 1 = 0x0B (dynamic, no sealed properties)
	if err := e.writeU29(0x0B); err != nil {
		return err
	}

	// Write an empty class name
	if err := e.writeStringWithReference(""); err != nil {
		return err
	}

	// Write dynamic properties
	for key, val := range value {
		if err := e.writeStringWithReference(key); err != nil {
			return err
		}
		if err := e.encodeValue(val); err != nil {
			return err
		}
	}

	// Write empty string to terminate properties
	return e.writeStringWithReference("")
}

// encodeObjectValue encodes an AMF3Object value
func (e *AMF3Encoder) encodeObjectValue(value AMF3Object) error {
	if err := e.writeByte(AMF3TypeObject); err != nil {
		return err
	}

	// For simplicity, treat as an anonymous dynamic object
	// Real implementation would handle traits properly
	if err := e.writeU29(0x0B); err != nil {
		return err
	}

	if err := e.writeStringWithReference(""); err != nil {
		return err
	}

	for key, val := range value.Properties {
		if err := e.writeStringWithReference(key); err != nil {
			return err
		}
		if err := e.encodeValue(val); err != nil {
			return err
		}
	}

	return e.writeStringWithReference("")
}

// Helper methods for encoding

// writeByte writes a single byte to the writer
func (e *AMF3Encoder) writeByte(b byte) error {
	_, err := e.writer.Write([]byte{b})
	return err
}

// writeU29 writes a 29-bit unsigned integer using variable-length encoding
// This is the core encoding method for AMF3 - most values use U29 encoding
// The encoding uses 1-4 bytes where the MSB indicates continuation
func (e *AMF3Encoder) writeU29(value uint32) error {
	// AMF3 integers are 29-bit, so mask off upper bits
	value &= 0x1FFFFFFF

	if value < 0x80 {
		// 1 byte: 0xxxxxxx
		return e.writeByte(byte(value))
	} else if value < 0x4000 {
		// 2 bytes: 1xxxxxxx 0xxxxxxx
		if err := e.writeByte(byte((value >> 7) | 0x80)); err != nil {
			return err
		}
		return e.writeByte(byte(value & 0x7F))
	} else if value < 0x200000 {
		// 3 bytes: 1xxxxxxx 1xxxxxxx 0xxxxxxx
		if err := e.writeByte(byte((value >> 14) | 0x80)); err != nil {
			return err
		}
		if err := e.writeByte(byte(((value >> 7) & 0x7F) | 0x80)); err != nil {
			return err
		}
		return e.writeByte(byte(value & 0x7F))
	} else {
		// 4 bytes: 1xxxxxxx 1xxxxxxx 1xxxxxxx xxxxxxxx
		if err := e.writeByte(byte((value >> 22) | 0x80)); err != nil {
			return err
		}
		if err := e.writeByte(byte(((value >> 15) & 0x7F) | 0x80)); err != nil {
			return err
		}
		if err := e.writeByte(byte(((value >> 8) & 0x7F) | 0x80)); err != nil {
			return err
		}
		return e.writeByte(byte(value & 0xFF))
	}
}

// writeStringWithReference writes a string with reference table support
// Empty strings are never added to the reference table
// If string is already in table, writes reference index
// Otherwise, adds to table and writes string content
func (e *AMF3Encoder) writeStringWithReference(s string) error {
	// Empty string is never referenced
	if s == "" {
		return e.writeU29(1) // length 0 with a reference bit set
	}

	// Check if string is already in the reference table
	for i, ref := range e.refs.stringTable {
		if ref == s {
			// Write reference: index << 1 (reference bit = 0)
			return e.writeU29(uint32(i << 1))
		}
	}

	// Add to the reference table
	e.refs.stringTable = append(e.refs.stringTable, s)

	// Write string length with a reference bit set: (length << 1) | 1
	if err := e.writeU29(uint32(len(s)<<1) | 1); err != nil {
		return err
	}

	// Write string content as UTF-8 bytes
	_, err := e.writer.Write([]byte(s))
	return err
}

// AMF3Decoder provides decoding of AMF3 format to Go values
//
//goland:noinspection ALL
type AMF3Decoder struct {
	reader io.Reader
	refs   referenceContext
}

// NewAMF3Decoder creates a new AMF3 decoder that reads from the provided reader
func NewAMF3Decoder(r io.Reader) *AMF3Decoder {
	return &AMF3Decoder{
		reader: r,
		refs:   referenceContext{},
	}
}

// Decode decodes an AMF3 value to a Go value
func (d *AMF3Decoder) Decode() (interface{}, error) {
	return d.decodeValue()
}

// DecodeAMF3 decodes an AMF3 value to an AMF3Value interface
func (d *AMF3Decoder) DecodeAMF3() (AMF3Value, error) {
	return d.decodeAMF3Value()
}

// decodeValue decodes any AMF3 value to a Go value
func (d *AMF3Decoder) decodeValue() (interface{}, error) {
	typeByte, err := d.readByte()
	if err != nil {
		return nil, err
	}

	switch typeByte {
	case AMF3TypeUndefined:
		return nil, nil // Go doesn't have undefined, use nil
	case AMF3TypeNull:
		return nil, nil
	case AMF3TypeFalse:
		return false, nil
	case AMF3TypeTrue:
		return true, nil
	case AMF3TypeInteger:
		return d.decodeInteger()
	case AMF3TypeDouble:
		return d.decodeDouble()
	case AMF3TypeString:
		return d.decodeString()
	case AMF3TypeDate:
		return d.decodeDate()
	case AMF3TypeArray:
		return d.decodeArray()
	case AMF3TypeObject:
		return d.decodeObject()
	default:
		return nil, fmt.Errorf("unsupported AMF3 type: 0x%02x", typeByte)
	}
}

// decodeAMF3Value decodes any AMF3 value to an AMF3Value interface
func (d *AMF3Decoder) decodeAMF3Value() (AMF3Value, error) {
	typeByte, err := d.readByte()
	if err != nil {
		return nil, err
	}

	switch typeByte {
	case AMF3TypeUndefined:
		return AMF3Undefined{}, nil
	case AMF3TypeNull:
		return AMF3Null{}, nil
	case AMF3TypeFalse:
		return AMF3Boolean(false), nil
	case AMF3TypeTrue:
		return AMF3Boolean(true), nil
	case AMF3TypeInteger:
		val, err := d.decodeInteger()
		if err != nil {
			return nil, err
		}
		return AMF3Integer(val.(int32)), nil
	case AMF3TypeDouble:
		val, err := d.decodeDouble()
		if err != nil {
			return nil, err
		}
		return AMF3Double(val.(float64)), nil
	case AMF3TypeString:
		val, err := d.decodeStringValue()
		if err != nil {
			return nil, err
		}
		return val, nil
	case AMF3TypeDate:
		val, err := d.decodeDateValue()
		if err != nil {
			return nil, err
		}
		return val, nil
	case AMF3TypeArray:
		val, err := d.decodeArrayValue()
		if err != nil {
			return nil, err
		}
		return val, nil
	case AMF3TypeObject:
		val, err := d.decodeObjectValue()
		if err != nil {
			return nil, err
		}
		return val, nil
	default:
		return nil, fmt.Errorf("unsupported AMF3 type: 0x%02x", typeByte)
	}
}

// Basic decoding methods

// readByte reads a single byte from the reader
func (d *AMF3Decoder) readByte() (byte, error) {
	buf := make([]byte, 1)
	_, err := io.ReadFull(d.reader, buf)
	return buf[0], err
}

// readU29 reads a 29-bit unsigned integer using variable-length decoding
// This is the core decoding method for AMF3 - most values use U29 encoding
func (d *AMF3Decoder) readU29() (uint32, error) {
	var result uint32
	var bytesRead int

	for i := 0; i < 4; i++ {
		b, err := d.readByte()
		if err != nil {
			return 0, err
		}
		bytesRead++

		if i < 3 {
			// For the first 3 bytes, MSB indicates continuation
			result = (result << 7) | uint32(b&0x7F)

			if (b & 0x80) == 0 {
				// No continuation bit, we're done
				break
			}
		} else {
			// 4th byte uses all 8 bits
			result = (result << 8) | uint32(b)
		}
	}

	// For integers, handle sign extension only for negative values when read as 29-bit signed
	// The MSB of the 29-bit value indicates sign
	if bytesRead == 4 && result&0x10000000 != 0 {
		result |= 0xE0000000 // Sign extend for negative values
	}

	return result, nil
}

// decodeInteger decodes an AMF3 integer
func (d *AMF3Decoder) decodeInteger() (interface{}, error) {
	value, err := d.readU29()
	if err != nil {
		return nil, err
	}
	// Convert back to signed 32-bit integer
	return int32(value), nil
}

// decodeDouble decodes an IEEE-754 double precision floating point number
func (d *AMF3Decoder) decodeDouble() (interface{}, error) {
	var bits uint64
	if err := binary.Read(d.reader, binary.BigEndian, &bits); err != nil {
		return nil, err
	}
	return math.Float64frombits(bits), nil
}

// decodeString decodes a string with reference support
func (d *AMF3Decoder) decodeString() (interface{}, error) {
	return d.readStringWithReference()
}

// decodeStringValue decodes a string into an AMF3String value
func (d *AMF3Decoder) decodeStringValue() (AMF3String, error) {
	str, err := d.readStringWithReference()
	if err != nil {
		return AMF3String{}, err
	}
	return AMF3String{Value: str, Ref: -1}, nil
}

// decodeDate decodes a date value
func (d *AMF3Decoder) decodeDate() (interface{}, error) {
	dateValue, err := d.decodeDateValue()
	if err != nil {
		return nil, err
	}
	return dateValue.Time, nil
}

// decodeDateValue decodes a date into an AMF3Date value
func (d *AMF3Decoder) decodeDateValue() (AMF3Date, error) {
	handle, err := d.readU29()
	if err != nil {
		return AMF3Date{}, err
	}

	// Check if this is a reference (LSB = 0)
	if (handle & 1) == 0 {
		// Reference to existing date
		index := handle >> 1
		if int(index) >= len(d.refs.objectTable) {
			return AMF3Date{}, fmt.Errorf("invalid date reference: %d", index)
		}
		if dateObj, ok := d.refs.objectTable[index].(AMF3Date); ok {
			return dateObj, nil
		}
		return AMF3Date{}, fmt.Errorf("referenced object is not a date")
	}

	// New date object - read timestamp
	var timestamp uint64
	if err := binary.Read(d.reader, binary.BigEndian, &timestamp); err != nil {
		return AMF3Date{}, err
	}

	// Convert from milliseconds since Unix epoch
	timestampFloat := math.Float64frombits(timestamp)
	timeValue := time.Unix(0, int64(timestampFloat*1e6))

	// Add to the reference table
	dateValue := AMF3Date{Time: timeValue, Ref: len(d.refs.objectTable)}
	d.refs.objectTable = append(d.refs.objectTable, dateValue)

	return dateValue, nil
}

// decodeArray decodes an array with dense and associative parts
func (d *AMF3Decoder) decodeArray() (interface{}, error) {
	arrayValue, err := d.decodeArrayValue()
	if err != nil {
		return nil, err
	}

	// Convert to Go types - return as a slice for dense arrays, map for associative
	if len(arrayValue.Associative) == 0 {
		// Pure dense array
		result := make([]interface{}, len(arrayValue.Dense))
		for i, v := range arrayValue.Dense {
			result[i] = v
		}
		return result, nil
	}

	// Mixed or pure associative array
	result := make(map[string]interface{})

	// Add associative properties
	for k, v := range arrayValue.Associative {
		result[k] = v
	}

	// Add a dense array as numbered keys
	for i, v := range arrayValue.Dense {
		result[fmt.Sprintf("%d", i)] = v
	}

	return result, nil
}

// decodeArrayValue decodes an array into an AMF3Array value
func (d *AMF3Decoder) decodeArrayValue() (AMF3Array, error) {
	handle, err := d.readU29()
	if err != nil {
		return AMF3Array{}, err
	}

	// Check if this is a reference (LSB = 0)
	if (handle & 1) == 0 {
		// Reference to an existing array
		index := handle >> 1
		if int(index) >= len(d.refs.objectTable) {
			return AMF3Array{}, fmt.Errorf("invalid array reference: %d", index)
		}
		if arrayObj, ok := d.refs.objectTable[index].(AMF3Array); ok {
			return arrayObj, nil
		}
		return AMF3Array{}, fmt.Errorf("referenced object is not an array")
	}

	// New array object
	denseLength := handle >> 1

	arrayValue := AMF3Array{
		Dense:       make([]AMF3Value, 0),
		Associative: make(map[string]AMF3Value),
		Ref:         len(d.refs.objectTable),
	}

	// Add to the reference table first (for self-references)
	d.refs.objectTable = append(d.refs.objectTable, arrayValue)

	// Read associative part (key-value pairs until empty string)
	for {
		key, err := d.readStringWithReference()
		if err != nil {
			return AMF3Array{}, err
		}
		if key == "" {
			break // Empty string terminates associative part
		}
		value, err := d.decodeAMF3Value()
		if err != nil {
			return AMF3Array{}, err
		}
		arrayValue.Associative[key] = value
	}

	// Read dense part
	for i := uint32(0); i < denseLength; i++ {
		value, err := d.decodeAMF3Value()
		if err != nil {
			return AMF3Array{}, err
		}
		arrayValue.Dense = append(arrayValue.Dense, value)
	}

	return arrayValue, nil
}

// decodeObject decodes an object (anonymous object with dynamic properties)
func (d *AMF3Decoder) decodeObject() (interface{}, error) {
	objectValue, err := d.decodeObjectValue()
	if err != nil {
		return nil, err
	}

	// Convert to Go map
	result := make(map[string]interface{})
	for k, v := range objectValue.Properties {
		result[k] = v
	}

	return result, nil
}

// decodeObjectValue decodes an object into an AMF3Object value
func (d *AMF3Decoder) decodeObjectValue() (AMF3Object, error) {
	handle, err := d.readU29()
	if err != nil {
		return AMF3Object{}, err
	}

	// Check if this is a reference (LSB = 0)
	if (handle & 1) == 0 {
		// Reference to an existing object
		index := handle >> 1
		if int(index) >= len(d.refs.objectTable) {
			return AMF3Object{}, fmt.Errorf("invalid object reference: %d", index)
		}
		if objectObj, ok := d.refs.objectTable[index].(AMF3Object); ok {
			return objectObj, nil
		}
		return AMF3Object{}, fmt.Errorf("referenced object is not an object")
	}

	// Check if this is a trait reference ((handle >> 1) & 1 == 0)
	if ((handle >> 1) & 1) == 0 {
		// Traits reference - for simplicity, we'll handle this as a new object
		// Real implementation would look up traits in the reference table
	}

	objectValue := AMF3Object{
		Properties: make(map[string]AMF3Value),
		Ref:        len(d.refs.objectTable),
	}

	// Add to the reference table first (for self-references)
	d.refs.objectTable = append(d.refs.objectTable, objectValue)

	// For simplicity, assume an anonymous dynamic object
	// Read class name (should be empty for anonymous)
	className, err := d.readStringWithReference()
	if err != nil {
		return AMF3Object{}, err
	}

	// Create simple traits
	objectValue.Traits = &AMF3Traits{
		ClassName:  className,
		Dynamic:    true,
		Properties: []string{},
		Ref:        -1,
	}

	// Read dynamic properties (key-value pairs until empty string)
	for {
		key, err := d.readStringWithReference()
		if err != nil {
			return AMF3Object{}, err
		}
		if key == "" {
			break // Empty string terminates properties
		}
		value, err := d.decodeAMF3Value()
		if err != nil {
			return AMF3Object{}, err
		}
		objectValue.Properties[key] = value
	}

	return objectValue, nil
}

// readStringWithReference reads a string with reference table support
func (d *AMF3Decoder) readStringWithReference() (string, error) {
	handle, err := d.readU29()
	if err != nil {
		return "", err
	}

	// Check if this is a reference (LSB = 0)
	if (handle & 1) == 0 {
		// Reference to an existing string
		index := handle >> 1
		if int(index) >= len(d.refs.stringTable) {
			return "", fmt.Errorf("invalid string reference: %d", index)
		}
		return d.refs.stringTable[index], nil
	}

	// New string - read length
	length := handle >> 1
	if length == 0 {
		return "", nil // Empty string is never added to the reference table
	}

	// Read string content
	buf := make([]byte, length)
	if _, err := io.ReadFull(d.reader, buf); err != nil {
		return "", err
	}

	str := string(buf)

	// Add to the reference table
	d.refs.stringTable = append(d.refs.stringTable, str)

	return str, nil
}

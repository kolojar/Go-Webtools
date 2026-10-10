package database

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"reflect"

	"github.com/kolojar/Go-Webtools/helpertools"
)

// ICustomDBType is interface for creating custom data types for DB
//
// It must be registered and it does not provide compatibility for fixing (when standard of the custom type changes, data will be lost). Registration using RegisterCustomDBType function or when encoding - it is added automatically
//
// CanParseDBToAny returns true if value can be parsed to any (not user initialized) object (created empty object with no value). -> False is when it needs prepared object (not all values are written in DB file) -> Examples: LimitedString X SmartDBString
//
// InteractiveRepairDB is called when parsing to any and interactive repair is enabled = when loading keys that got removed. Should use user input. Only experienced users or DB admins should use this
//
// ValueOnNilPointer should return nil or pointer that will be put instead for NULL read from DB
type ICustomDBType interface {
	ConvertToBytesDB(writer io.Writer) error
	ParseBytesDB(reader io.Reader) error
	ValueOnNilPointer() *ICustomDBType
	CanParseDBToAny() bool
	InteractiveRepairDB() (bool, error)
}

var registeredCustomTypes = helpertools.MakeSafeMapChecklist[reflect.Type]()

// RegisterCustomDBType registers type for database.
//
// These types do not provide compatibility for fixing (when standard of the custom type changes, data will be lost)
//
// It is recommended to use the stucts as much as possible
func RegisterCustomDBType[T ICustomDBType]() {
	RegisterCustomDBTypeReflect(reflect.TypeFor[T]())
}

// RegisterCustomDBTypeReflect registers type for database.
//
// These types do not provide compatibility for fixing (when standard of the custom type changes, data will be lost)
//
// It is recommended to use the stucts as much as possible
func RegisterCustomDBTypeReflect(t reflect.Type) {
	//Get normal value of pointer
	if t.Kind() == reflect.Pointer || t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	//Check if already registered
	if !registeredCustomTypes.AddOnce(t) {
		return
	}

	//Register pointer edition
	t = reflect.PointerTo(t)
	registeredCustomTypes.AddOnce(t)
}

// DBFieldType is type of DBField (number values are from markdown reference - better for byte ASCII reading)
type DBFieldType uint8

// NormalDataTypeFieldType is basic type like int, string, ... thad cant have children
const NormalDataTypeFieldType DBFieldType = 0

// MapFieldType is map data type
const MapFieldType DBFieldType = 1

// UserCustomDataTypeField is custom registered data type
const UserCustomDataTypeField DBFieldType = 33

// StructFieldType is struct data type
const StructFieldType DBFieldType = 123

// StructEndFieldType is struct end helper data type
const StructEndFieldType DBFieldType = 125

// StructEndFieldType is map end helper data type
const MapEndFieldType DBFieldType = 62

// DBFieldType is modifier of DBField (number values are from markdown reference - better for byte ASCII reading)
type DBFieldModifier uint8

// SliceFieldModifier is slice data type
const SliceFieldModifier DBFieldModifier = 36

// ArrayFieldModifier is array data type
const ArrayFieldModifier DBFieldModifier = 64

// PointerFieldModifier is pointer data type (can be nil)
const PointerFieldModifier DBFieldModifier = 42

// MapKeyFieldModifier is map key data type, has to be first modifier
const MapKeyFieldModifier DBFieldModifier = 60

// MapValueFieldModifier is map value data type, has to be first modifier
const MapValueFieldModifier DBFieldModifier = 94

// DBField is cache structure for faster reflection for dynamic database
type DBField struct {
	Name           string
	Index          int
	Type           reflect.Type
	ValueType      reflect.Type
	FieldType      DBFieldType
	FieldModifiers []DBFieldModifier
	Fields         []DBField
}

var dbFieldSchemas map[reflect.Type]helpertools.KeyValuePair[DBField, []byte] = map[reflect.Type]helpertools.KeyValuePair[DBField, []byte]{}

func buildDBSchemaField(t reflect.Type, name string, index int) DBField {
	//Unpack all modifiers
	modifiers := make([]DBFieldModifier, 0)
	tElem := t
	for true {
		if tElem.Kind() == reflect.Pointer || tElem.Kind() == reflect.Ptr {
			//Is pointer
			modifiers = append(modifiers, PointerFieldModifier)
			tElem = t.Elem()
			continue
		}
		if tElem.Kind() == reflect.Slice {
			// Is slice
			modifiers = append(modifiers, SliceFieldModifier)
			tElem = tElem.Elem()
			continue
		}
		if tElem.Kind() == reflect.Array {
			// Is array
			modifiers = append(modifiers, SliceFieldModifier)
			tElem = tElem.Elem()
			continue
		}
		break
	}

	//Sort type
	typ := NormalDataTypeFieldType
	if tElem.Kind() == reflect.Map {
		// Is map
		typ = MapFieldType
	}
	if tElem.Kind() == reflect.Struct {
		//Is struct
		if tElem.Implements(reflect.TypeFor[ICustomDBType]()) {
			//Is special data type
			RegisterCustomDBTypeReflect(tElem)
			typ = UserCustomDataTypeField
		} else {
			typ = StructFieldType
		}
	}

	//Make result
	return DBField{
		Name:           name,
		Index:          index,
		Type:           t,
		ValueType:      tElem,
		FieldType:      typ,
		FieldModifiers: modifiers,
	}
}

func convertReflectKindToByte(kind reflect.Kind) byte {
	switch kind {
	case reflect.Int:
		return 65
	case reflect.Int8:
		return 66
	case reflect.Int16:
		return 67
	case reflect.Int32:
		return 68
	case reflect.Int64:
		return 69
	case reflect.Uint:
		return 70
	case reflect.Uint8:
		return 71
	case reflect.Uint16:
		return 72
	case reflect.Uint32:
		return 73
	case reflect.Uint64:
		return 74
	case reflect.Float32:
		return 75
	case reflect.Float64:
		return 76
	case reflect.Complex64:
		return 77
	case reflect.Complex128:
		return 78
	case reflect.String:
		return 79
	case reflect.Bool:
		return 80
	}
	return 0
}

func BuildDBSchemaBytes(buffer *bytes.Buffer, field DBField) {
	//Write modifiers
	hasMapModifier := false
	for _, v := range field.FieldModifiers {
		if v == MapKeyFieldModifier || v == MapValueFieldModifier {
			if hasMapModifier {
				panic("cant have multiple map modifiers in one field")
			}
			hasMapModifier = true
		}
		buffer.WriteByte(byte(v))
	}

	//Write map field
	if field.FieldType == MapFieldType {
		BuildDBSchemaBytes(buffer, field.Fields[0])
		BuildDBSchemaBytes(buffer, field.Fields[1])
		if !hasMapModifier {
			ConvertStringToBytesDB(buffer, field.Name)
		}
		buffer.WriteByte(byte(MapEndFieldType))
		return
	}

	//Write user type
	if field.FieldType == UserCustomDataTypeField {
		buffer.WriteByte(byte(field.FieldType))
		ConvertStringToBytesDB(buffer, field.ValueType.String())
		if !hasMapModifier {
			ConvertStringToBytesDB(buffer, field.Name)
		}
		return
	}

	//Write struct type
	if field.FieldType == StructFieldType {
		buffer.WriteByte(byte(StructFieldType))
		if field.Fields != nil {
			for _, v := range field.Fields {
				BuildDBSchemaBytes(buffer, v)
			}
		}
		buffer.WriteByte(byte(StructEndFieldType))
		return
	}

	//Write normal type
	buffer.WriteByte(convertReflectKindToByte(field.ValueType.Kind()))
	if !hasMapModifier {
		ConvertStringToBytesDB(buffer, field.Name)
	}
}

/*
BuildDBSchema builds DB schema or reuses existing one from cache
*/
func BuildDBSchema(t reflect.Type) (DBField, []byte) {
	// Check cache
	get, has := dbFieldSchemas[t]
	if has {
		return get.Key, get.Value
	}

	// Generate structure
	schema := buildDBSchemaField(t, "", -1)
	fmt.Println("making " + schema.ValueType.Name())
	if schema.FieldType == StructFieldType {
		// Check for ICustomDBType
		if schema.ValueType.Implements(reflect.TypeFor[ICustomDBType]()) || reflect.PointerTo(schema.ValueType).Implements(reflect.TypeFor[ICustomDBType]()) {
			RegisterCustomDBTypeReflect(schema.ValueType)
			schema.FieldType = UserCustomDataTypeField
		} else {
			// Build struct
			count := schema.ValueType.NumField()
			schema.Fields = make([]DBField, count)
			for i := 0; i < count; i++ {
				field := schema.ValueType.Field(i)
				nameDB := field.Tag.Get("db")
				if nameDB == "-" {
					// Ignored
					continue
				} else if nameDB == "" {
					nameDB = field.Name
				}
				fieldDB, _ := BuildDBSchema(field.Type)
				fieldDB.Name = nameDB
				fieldDB.Index = i
				schema.Fields[i] = fieldDB
			}
		}
	}
	if schema.FieldType == MapFieldType {
		// Build map
		schema.Fields = make([]DBField, 0)
		fieldDB, _ := BuildDBSchema(schema.ValueType.Key())
		fieldDB.Name = "mapKey"
		fieldDB.Index = -10
		fieldDB.FieldModifiers = append(fieldDB.FieldModifiers, MapKeyFieldModifier)
		schema.Fields = append(schema.Fields, fieldDB)
		fieldDB, _ = BuildDBSchema(schema.ValueType.Elem())
		fieldDB.Name = "mapVal"
		fieldDB.Index = -11
		fieldDB.FieldModifiers = append(fieldDB.FieldModifiers, MapValueFieldModifier)
		schema.Fields = append(schema.Fields, fieldDB)
	}

	//Create schema bytes
	buffer := bytes.NewBuffer(make([]byte, 0))
	BuildDBSchemaBytes(buffer, schema)
	dbFieldSchemas[t] = helpertools.KeyValuePair[DBField, []byte]{Key: schema, Value: buffer.Bytes()}
	return schema, dbFieldSchemas[t].Value
}

func convertAnyValueToBytesDBValue(writer io.Writer, k reflect.Kind, v reflect.Value) error {
	switch k {
	case reflect.Bool:
		return ConvertBoolToBytesDB(writer, v.Bool())
	case reflect.Uint, reflect.Uint64:
		return ConvertDynamicUintToBytesDB(writer, v.Uint())
	case reflect.Int, reflect.Int64:
		return ConvertDynamicUintToBytesDB(writer, uint64(v.Int()))
	case reflect.Uint8:
		return ConvertUint8ToBytesDB(writer, uint8(v.Uint()))
	case reflect.Int8:
		return ConvertUint8ToBytesDB(writer, uint8(v.Int()))
	case reflect.String:
		return ConvertStringToBytesDB(writer, v.String())
	case reflect.Int16:
		return ConvertUint16ToBytesDB(writer, uint16(v.Int()))
	case reflect.Int32:
		return ConvertDynamicUintToBytesDB(writer, uint64(v.Int()))
	case reflect.Uint16:
		return ConvertUint16ToBytesDB(writer, uint16(v.Uint()))
	case reflect.Uint32:
		return ConvertDynamicUintToBytesDB(writer, v.Uint())
	default:
		return os.ErrInvalid
	}
}

func convertFieldValueToBytesDB(writer io.Writer, schema DBField, modifierOffset int, v reflect.Value) error {
	//fmt.Println("writing: " + BuildDBSchemaBytes(schema))
	// Process all modifiers and unpack
	for i := modifierOffset; i < len(schema.FieldModifiers); i++ {
		//Skip non processable
		mod := schema.FieldModifiers[i]
		if mod == MapKeyFieldModifier || mod == MapValueFieldModifier {
			continue
		}

		//Process
		if mod == PointerFieldModifier {
			//Process pointer
			if v.Kind() != reflect.Pointer {
				panic("types do not match")
			}
			if v.IsNil() {
				_, err := writer.Write([]byte{0})
				return err
			}
			v = v.Elem()
			continue
		} else if mod == SliceFieldModifier || mod == ArrayFieldModifier {
			// Write length
			err := ConvertDynamicUintToBytesDB(writer, uint64(v.Len()))
			if err != nil {
				return err
			}

			// Write data
			for i := 0; i < v.Len(); i++ {
				err = convertFieldValueToBytesDB(writer, schema, i+1, v.Index(i))
				if err != nil {
					return err
				}
			}
			return nil
		}
	}

	//Process map
	if schema.FieldType == MapFieldType {
		// Write length
		err := ConvertDynamicUintToBytesDB(writer, uint64(v.Len()))
		if err != nil {
			return err
		}

		// Write data
		for _, k := range v.MapKeys() {
			err := convertFieldValueToBytesDB(writer, schema.Fields[0], 0, k)
			if err != nil {
				return err
			}
			err = convertFieldValueToBytesDB(writer, schema.Fields[1], 0, v.MapIndex(k))
			if err != nil {
				return err
			}
		}
		return nil
	}

	// Process user defined type
	if schema.FieldType == UserCustomDataTypeField {
		fmt.Println(v.Type().String())

		//Convert using function
		convert, ok := v.Interface().(ICustomDBType)
		if ok {
			return convert.ConvertToBytesDB(writer)
		}
		if v.CanAddr() {
			convert, ok = v.Addr().Interface().(ICustomDBType)
			if ok {
				return convert.ConvertToBytesDB(writer)
			}
		}

		//Try to check for pointer value
		v2 := reflect.New(v.Type())
		v2.Elem().Set(v)
		fmt.Println(v2.Type().String())
		convert, ok = v2.Interface().(ICustomDBType)
		if ok {
			return convert.ConvertToBytesDB(writer)
		}
		if v2.CanAddr() {
			convert, ok = v2.Addr().Interface().(ICustomDBType)
			if ok {
				return convert.ConvertToBytesDB(writer)
			}
		}
		return os.ErrInvalid
	}

	//Check for struct
	if schema.FieldType == StructFieldType {
		for _, f := range schema.Fields {
			err := convertFieldValueToBytesDB(writer, f, 0, v.Field(f.Index))
			if err != nil {
				return err
			}
		}
		return nil
	}

	// Normal end value
	return convertAnyValueToBytesDBValue(writer, schema.ValueType.Kind(), v)
}

/*
ConvertAnyToBytesDB converts any value to bytes
*/
func ConvertAnyToBytesDB(writer io.Writer, data any) (err error) {
	//Convert to bufio
	writer, automaticFlush := ConvertToBufioWriter(writer, func(errFlush error) {
		err = errFlush
	})
	defer automaticFlush()

	// Try to convert some basic type - I do not think it is needed
	//v := reflect.ValueOf(data)
	//err = convertAnyValueToBytesDBValue(writer, v.Kind(), v)
	//if err != nil && !errors.Is(os.ErrInvalid, err) {
	//	return err
	//}

	// Generate schema
	t := reflect.TypeOf(data)
	schema, schemaBytes := BuildDBSchema(t)

	//Write schema header
	_, err = writer.Write([]byte{1})
	if err != nil {
		return err
	}

	//Write schema bytes
	_, err = writer.Write(schemaBytes)
	if err != nil {
		return err
	}

	//Write value start header
	_, err = writer.Write([]byte{2})
	if err != nil {
		return err
	}

	//Write data
	err = convertFieldValueToBytesDB(writer, schema, 0, reflect.ValueOf(data))
	if err != nil {
		return err
	}

	//Write value end header
	_, err = writer.Write([]byte{3})
	return err
}

func parseAnyValueToBytesDBValue(reader io.Reader, valType string, objectValue *reflect.Value, createdNew bool, interactiveRepair bool) (any, error) {
	var err error
	var result any
	switch valType {
	case "bool":
		result, err = ParseBoolDB(reader)
	case "uint":
		result, err = ParseDynamicUintBytesDB(reader)
		result = uint(result.(uint64))
	case "int":
		result, err = ParseDynamicUintBytesDB(reader)
		result = int(int64(result.(uint64)))
	case "uint64":
		result, err = ParseDynamicUintBytesDB(reader)
	case "int64":
		result, err = ParseDynamicUintBytesDB(reader)
		result = int64(result.(uint64))
	case "uint8":
		result, err = ParseUint8DB(reader)
	case "int8":
		result, err = ParseBoolDB(reader)
		result = int8(result.(uint8))
	case "string":
		result, err = ParseStringDB(reader)
	case "int16":
		result, err = ParseUint16DB(reader)
		result = int16(result.(uint16))
	case "int32":
		result, err = ParseDynamicUintBytesDB(reader)
		result = int32(result.(uint32))
	case "uint16":
		result, err = ParseUint16DB(reader)
	case "uint32":
		result, err = ParseDynamicUintBytesDB(reader)
	default:
		//Check user defined types
		var t reflect.Type = nil
		registeredCustomTypes.Range(func(key reflect.Type) (doBreak bool, delete bool, err error) {
			if key.String() == valType {
				t = key
				return true, false, nil
			}
			return false, false, nil
		})

		//Check if found
		if t == nil {
			return nil, os.ErrInvalid
		}

		//Parse
		if objectValue != nil {
			// User defined type - value
			convert, ok := objectValue.Interface().(ICustomDBType)
			if ok {
				if !convert.CanParseDBToAny() && createdNew {
					if interactiveRepair {
						//Try interactive repair
						repaired, err := convert.InteractiveRepairDB()
						if err != nil {
							return nil, err
						}
						if !repaired {
							fmt.Println("Can not parse: " + t.String() + " to any.")
							return nil, os.ErrNotExist
						}
					} else {
						fmt.Println("Can not parse: " + t.String() + " to any.")
						return nil, os.ErrNotExist
					}
				}
				err = convert.ParseBytesDB(reader)
				return convert, err
			}
			if objectValue.CanAddr() {
				convert, ok = objectValue.Addr().Interface().(ICustomDBType)
				if ok {
					if !convert.CanParseDBToAny() && createdNew {
						if interactiveRepair {
							//Try interactive repair
							repaired, err := convert.InteractiveRepairDB()
							if err != nil {
								return nil, err
							}
							if !repaired {
								fmt.Println("Can not parse: " + t.String() + " to any.")
								return nil, os.ErrNotExist
							}
						} else {
							fmt.Println("Can not parse: " + t.String() + " to any.")
							return nil, os.ErrNotExist
						}
					}
					err = convert.ParseBytesDB(reader)
					return convert, err
				}
			}

			//Try to check for pointer value
			v2 := reflect.New(objectValue.Type())
			v2.Elem().Set(*objectValue)
			fmt.Println(v2.Type().String())
			convert, ok = v2.Interface().(ICustomDBType)
			if ok {
				if !convert.CanParseDBToAny() && createdNew {
					if interactiveRepair {
						//Try interactive repair
						repaired, err := convert.InteractiveRepairDB()
						if err != nil {
							return nil, err
						}
						if !repaired {
							fmt.Println("Can not parse: " + t.String() + " to any.")
							return nil, os.ErrNotExist
						}
					} else {
						fmt.Println("Can not parse: " + t.String() + " to any.")
						return nil, os.ErrNotExist
					}
				}
				err = convert.ParseBytesDB(reader)
				return convert, err
			}
			if v2.CanAddr() {
				convert, ok = v2.Addr().Interface().(ICustomDBType)
				if ok {
					if !convert.CanParseDBToAny() && createdNew {
						if interactiveRepair {
							//Try interactive repair
							repaired, err := convert.InteractiveRepairDB()
							if err != nil {
								return nil, err
							}
							if !repaired {
								fmt.Println("Can not parse: " + t.String() + " to any.")
								return nil, os.ErrNotExist
							}
						} else {
							fmt.Println("Can not parse: " + t.String() + " to any.")
							return nil, os.ErrNotExist
						}
					}
					err = convert.ParseBytesDB(reader)
					return convert, err
				}
			}
			return nil, os.ErrNotExist
		}

		// User defined type - any
		v := reflect.New(t)
		convert, ok := v.Interface().(ICustomDBType)
		if ok {
			if !convert.CanParseDBToAny() {
				if interactiveRepair {
					//Try interactive repair
					repaired, err := convert.InteractiveRepairDB()
					if err != nil {
						return nil, err
					}
					if !repaired {
						fmt.Println("Can not parse: " + t.String() + " to any.")
						return nil, os.ErrNotExist
					}
				} else {
					fmt.Println("Can not parse: " + t.String() + " to any.")
					return nil, os.ErrNotExist
				}
			}
			err := convert.ParseBytesDB(reader)
			return convert, err
		}
		convert, ok = v.Addr().Interface().(ICustomDBType)
		if ok {
			if !convert.CanParseDBToAny() {
				fmt.Println("Can not parse: " + t.String() + " to any.")
				return nil, os.ErrNotExist
			}
			err := convert.ParseBytesDB(reader)
			return convert, err
		}
	}
	return result, err
}

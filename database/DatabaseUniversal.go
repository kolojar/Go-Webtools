package database

import (
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"webtools"
)

/*
ICustomDBType is interface for creating custom data types for DB
It must be registered and it does not provide compatibility for fixing (when standard of the custom type changes, data will be lost)
Registration using RegisterCustomDBType function or when encoding - it is added automatically
CanParseDBToAny returns true if value can be parsed to any (not user initialized) object (created empty object with no value). -> False is when it needs prepared object (not all values are written in DB file) -> Examples: LimitedString X SmartDBString
*/
type ICustomDBType interface {
	ConvertToBytesDB(writer io.Writer) error
	ParseBytesDB(reader io.Reader) error
	CanParseDBToAny() bool
}

var registeredCustomTypes = make([]reflect.Type, 0)

/*
RegisterCustomDBType registers type for database.
These types do not provide compatibility for fixing (when standard of the custom type changes, data will be lost)
It is recommended to use the stucts as much as possible
*/
func RegisterCustomDBType[T ICustomDBType]() {
	RegisterCustomDBTypeReflect(reflect.TypeFor[T]())
}

/*
RegisterCustomDBTypeReflect registers type for database.
These types do not provide compatibility for fixing (when standard of the custom type changes, data will be lost)
It is recommended to use the stucts as much as possible
*/
func RegisterCustomDBTypeReflect(t reflect.Type) {
	if t.Kind() == reflect.Pointer || t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	for i := 0; i < len(registeredCustomTypes); i++ {
		if t == registeredCustomTypes[i] {
			return
		}
	}
	registeredCustomTypes = append(registeredCustomTypes, t)
	t = reflect.PointerTo(t)
	for i := 0; i < len(registeredCustomTypes); i++ {
		if t == registeredCustomTypes[i] {
			return
		}
	}
	registeredCustomTypes = append(registeredCustomTypes, t)
}

/*
DBField is field holder for dynamic database object
*/
type DBField struct {
	Name           string
	Index          int
	IsSlice        bool
	IsMap          bool
	IsCustomDBType bool
	Type           reflect.Type
	ValueType      reflect.Type
	Fields         []DBField
}

var dbFieldSchemas map[reflect.Type]webtools.KeyValuePair[DBField, string] = map[reflect.Type]webtools.KeyValuePair[DBField, string]{}

func buildDBSchemaField(t reflect.Type, name string, index int) DBField {
	tElem := t
	if t.Kind() == reflect.Pointer || t.Kind() == reflect.Ptr {
		tElem = t.Elem()
	}
	isSlice := false
	isMap := false
	isCustomDBType := false
	if tElem.Kind() == reflect.Slice {
		//Is slice
		tElem = tElem.Elem()
		isSlice = true
	}
	if tElem.Kind() == reflect.Map {
		//Is map
		isMap = true
	}
	if tElem.Implements(reflect.TypeFor[ICustomDBType]()) {
		RegisterCustomDBTypeReflect(tElem)
		isCustomDBType = true
	}
	return DBField{
		Name:           name,
		Index:          index,
		Type:           t,
		ValueType:      tElem,
		IsSlice:        isSlice,
		IsMap:          isMap,
		IsCustomDBType: isCustomDBType,
	}
}

func BuildDBSchemaString(field DBField) string {
	//Resolve array
	result := ""
	if field.IsSlice {
		result += "[]"
	}
	//Write all fields
	result += "{"
	if field.Fields != nil {
		for _, v := range field.Fields {
			result += v.Name + ":"
			if !v.IsCustomDBType && v.ValueType.Kind() == reflect.Struct {
				result += BuildDBSchemaString(v)
			} else {
				if v.IsSlice {
					result += "[]"
				}
				if v.IsMap {
					//Is Map
					//panic("Fix map")
					result += "map<" + BuildDBSchemaString(v.Fields[0]) + "-" + BuildDBSchemaString(v.Fields[1]) + ">"
				} else {
					result += v.ValueType.String()
				}
			}
			result += "-"
		}
	} else {
		result += field.Name + ":" + field.ValueType.String()
	}
	result = strings.TrimSuffix(result, "-")
	result += "}"
	return result
}

/*
BuildDBSchema builds DB schema or reuses existing one from cache
*/
func BuildDBSchema(t reflect.Type) (DBField, string) {
	//Check cache
	get, has := dbFieldSchemas[t]
	if has {
		return get.Key, get.Value
	}

	//Generate structure
	schema := buildDBSchemaField(t, "", -1)
	fmt.Println("making " + schema.ValueType.Name())
	if !schema.IsCustomDBType && schema.ValueType.Kind() == reflect.Struct {
		//Check for ICustomDBType
		if schema.ValueType.Implements(reflect.TypeFor[ICustomDBType]()) || reflect.PointerTo(schema.ValueType).Implements(reflect.TypeFor[ICustomDBType]()) {
			RegisterCustomDBTypeReflect(schema.ValueType)
			schema.IsCustomDBType = true
		} else {
			//Build struct
			schema.Fields = make([]DBField, 0)
			for i := 0; i < schema.ValueType.NumField(); i++ {
				field := schema.ValueType.Field(i)
				nameDB := field.Tag.Get("db")
				if nameDB == "-" {
					//Ignored
					continue
				} else if nameDB == "" {
					nameDB = field.Name
				}
				fieldDB, _ := BuildDBSchema(field.Type)
				fieldDB.Name = nameDB
				fieldDB.Index = i
				schema.Fields = append(schema.Fields, fieldDB)
			}
		}
	}
	if schema.IsMap {
		//Build map
		schema.Fields = make([]DBField, 0)
		fieldDB, _ := BuildDBSchema(schema.ValueType.Key())
		fieldDB.Name = "mapKey"
		fieldDB.Index = -10
		schema.Fields = append(schema.Fields, fieldDB)
		fieldDB, _ = BuildDBSchema(schema.ValueType.Elem())
		fieldDB.Name = "mapVal"
		fieldDB.Index = -11
		schema.Fields = append(schema.Fields, fieldDB)
	}
	schemaString := BuildDBSchemaString(schema)
	dbFieldSchemas[t] = webtools.KeyValuePair[DBField, string]{Key: schema, Value: schemaString}
	return schema, schemaString
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

//	func convertFieldStructToBytesDB(writer io.Writer, schema *DBStruct, v reflect.Value) error {
//		for _, field := range schema.FieldsPrimitives {
//			err := convertFieldValueToBytesDB(writer, field, v)
//			if err != nil {
//				return err
//			}
//		}
//		return nil
//	}
func convertFieldValueToBytesDB(writer io.Writer, schema DBField, v reflect.Value) error {
	fmt.Println("writing: " + BuildDBSchemaString(schema))
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			panic("pointer is nil")
		} else {
			v = v.Elem()
		}
	}
	if schema.IsSlice {
		//Slice
		schemaLocal := schema
		schemaLocal.IsSlice = false

		//Write length
		err := ConvertDynamicUintToBytesDB(writer, uint64(v.Len()))
		if err != nil {
			return err
		}

		//Write data
		for i := 0; i < v.Len(); i++ {
			err = convertFieldValueToBytesDB(writer, schemaLocal, v.Index(i))
			if err != nil {
				return err
			}
		}
		return nil
	}
	if schema.IsMap {
		//Map
		//Write length
		err := ConvertDynamicUintToBytesDB(writer, uint64(v.Len()))
		if err != nil {
			return err
		}

		//Write data
		for _, k := range v.MapKeys() {
			err := convertFieldValueToBytesDB(writer, schema.Fields[0], k)
			if err != nil {
				return err
			}
			err = convertFieldValueToBytesDB(writer, schema.Fields[1], v.MapIndex(k))
			if err != nil {
				return err
			}
		}
		return nil
	}
	if schema.IsCustomDBType {
		//User defined type
		convert, ok := v.Interface().(ICustomDBType)
		if ok {
			return convert.ConvertToBytesDB(writer)
		}
		convert, ok = v.Addr().Interface().(ICustomDBType)
		if ok {
			return convert.ConvertToBytesDB(writer)
		}
		return os.ErrInvalid
	}
	if schema.Fields == nil {
		//Normal end value
		return convertAnyValueToBytesDBValue(writer, schema.ValueType.Kind(), v)

	}
	//Struct
	for _, f := range schema.Fields {
		err := convertFieldValueToBytesDB(writer, f, v.Field(f.Index))
		if err != nil {
			return err
		}
	}
	return nil
}

/*
ConvertAnyToBytesDB converts any value to bytes
*/
func ConvertAnyToBytesDB(writer io.Writer, data any) error {
	//Try to convert some basic type
	v := reflect.ValueOf(data)
	err := convertAnyValueToBytesDBValue(writer, v.Kind(), v)
	if err != nil && !errors.Is(os.ErrInvalid, err) {
		return err
	}

	//Convert complex types
	t := reflect.TypeOf(data)
	schema, schemaString := BuildDBSchema(t)
	err = ConvertStringToBytesDB(writer, schemaString)
	if err != nil {
		return err
	}
	err = convertFieldValueToBytesDB(writer, schema, reflect.ValueOf(data))
	if err != nil {
		return err
	}
	_, err = writer.Write([]byte("*EOF*"))
	return err
}

func parseAnyValueToBytesDBValue(reader io.Reader, k reflect.Kind) (reflect.Value, error) {
	var err error
	var result any
	switch k {
	case reflect.Bool:
		result, err = ParseBoolDB(reader)
		break
	case reflect.Uint:
		result, err = ParseUint64DB(reader)
		result = uint(result.(uint64))
		break
	case reflect.Int:
		result, err = ParseUint64DB(reader)
		result = int(int64(result.(uint64)))
		break
	case reflect.Uint64:
		result, err = ParseUint64DB(reader)
		break
	case reflect.Int64:
		result, err = ParseUint64DB(reader)
		result = int64(result.(uint64))
		break
	case reflect.Uint8:
		result, err = ParseUint8DB(reader)
		break
	case reflect.Int8:
		result, err = ParseBoolDB(reader)
		result = int8(result.(uint8))
		break
	case reflect.String:
		result, err = ParseStringDB(reader)
		break
	case reflect.Int16:
		result, err = ParseUint16DB(reader)
		result = int16(result.(uint16))
		break
	case reflect.Int32:
		result, err = ParseUint32DB(reader)
		result = int32(result.(uint32))
		break
	case reflect.Uint16:
		result, err = ParseUint16DB(reader)
		break
	case reflect.Uint32:
		result, err = ParseUint32DB(reader)
		break
	default:
		return reflect.ValueOf(nil), os.ErrInvalid
	}
	if err != nil {
		return reflect.ValueOf(nil), err
	}
	return reflect.ValueOf(result), nil
}

/*
ParseAnyDB parses bytes to generic T object (can parse any type)
*/
func ParseAnyDB[T any](reader io.Reader) (T, error) {
	//Try to convert some basic type
	val, err := parseAnyValueToBytesDBValue(reader, reflect.TypeFor[T]().Kind())
	if err == nil {
		//OK
		result := val.Interface()
		return result.(T), nil
	}
	if err != nil && !errors.Is(os.ErrInvalid, err) {
		//Invalid type (read error)
		var zero T
		return zero, err
	}

	//Convert complex types
	result := new(T)
	err = ParseAnyToObjectDB(reader, result)
	return *result, err
}

func buildDBSchemaFromString(schema string) DBField {
	braceDepth := 0
	braceStart := 0
	writingName := true
	typeName := ""
	currentField := DBField{Name: "", Index: -1}
	for i := 0; i < len(schema); i++ {
		if schema[i] == '[' && schema[i+1] == ']' {
			//Is slice
			currentField.IsSlice = true
			continue
		}
		if schema[i] == '{' {
			braceStart = i
			braceDepth++
			j := i
			for j < len(schema) {
				//Get whole brace content
				if schema[j] == '}' {
					braceDepth--
				}
				if braceDepth == 0 {
					//Finished, pass schema to recursion
					currentField.Fields = append(currentField.Fields, buildDBSchemaFromString(schema[braceStart+1:j]))
				}
				j++
			}
			i = j
			continue
		}
		if schema[i] == ']' {
			continue
		}
		if schema[i] == ':' {
			writingName = false
			continue
		}
		if schema[i] == '-' {
			writingName = true
			if typeName == "string" {
				currentField.ValueType = reflect.TypeFor[string]()
			}
			if typeName == "uint8" {
				currentField.ValueType = reflect.TypeFor[uint8]()
			}
			if typeName == "uint16" {
				currentField.ValueType = reflect.TypeFor[uint16]()
			}
			if typeName == "uint32" {
				currentField.ValueType = reflect.TypeFor[uint32]()
			}
			if typeName == "uint64" {
				currentField.ValueType = reflect.TypeFor[uint64]()
			}
			if typeName == "int8" {
				currentField.ValueType = reflect.TypeFor[int8]()
			}
			if typeName == "int16" {
				currentField.ValueType = reflect.TypeFor[int16]()
			}
			if typeName == "int32" {
				currentField.ValueType = reflect.TypeFor[int32]()
			}
			if typeName == "int64" {
				currentField.ValueType = reflect.TypeFor[int64]()
			}
			if typeName == "bool" {
				currentField.ValueType = reflect.TypeFor[bool]()
			}
			if currentField.IsSlice {
				currentField.Type = reflect.SliceOf(currentField.ValueType)
			} else {
				currentField.Type = currentField.ValueType
			}
		}
		if writingName {
			currentField.Name += string(schema[i])
		} else {
			typeName += string(schema[i])
		}
	}
	return currentField
}

/*
ParseAnyToObjectDB parses bytes to generic target object (can parse only complex types)
*/
func ParseAnyToObjectDB(reader io.Reader, target any) error {
	//Check if target is pointer
	if target == nil {
		return os.ErrInvalid
	}
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Pointer && v.Kind() != reflect.Ptr {
		return os.ErrInvalid
	}

	//Read structure string
	structString, err := ParseStringDB(reader)
	if err != nil {
		return err
	}
	fmt.Println(structString)
	schema := buildDBSchemaFromString(structString)
	fmt.Println(schema)
	return nil
}

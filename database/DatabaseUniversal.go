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
InteractiveRepairDB is called when parsing to any and interactive repair is enabled = when loading keys that got removed. Should use user input. Only experienced users or DB admins should use this
*/
type ICustomDBType interface {
	ConvertToBytesDB(writer io.Writer) error
	ParseBytesDB(reader io.Reader) error
	CanParseDBToAny() bool
	InteractiveRepairDB() (bool, error)
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
	fmt.Println("Registered: " + t.String())
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
	IsMapParam     bool
	IsStruct       bool
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
		// Is slice
		tElem = tElem.Elem()
		isSlice = true
	}
	if tElem.Kind() == reflect.Map {
		// Is map
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
	// Resolve array
	result := ""
	if field.IsSlice {
		result += "[]"
	}
	// Write all fields
	if field.IsStruct {
		//if !field.IsMapParam {
		result += "{"
		//}
	}
	if field.Fields != nil {
		for _, v := range field.Fields {
			if !v.IsMapParam {
				result += v.Name + ":"
			}
			if !v.IsCustomDBType && v.ValueType.Kind() == reflect.Struct {
				result += BuildDBSchemaString(v)
			} else {
				if v.IsSlice {
					result += "[]"
				}
				if v.IsMap {
					// Is Map
					// panic("Fix map")
					result += "<" + BuildDBSchemaString(v.Fields[0]) + "-" + BuildDBSchemaString(v.Fields[1]) + ">"
				} else {
					result += v.ValueType.String()
				}
			}
			result += "-"
		}
	} else {
		if !field.IsMapParam {
			result += field.Name + ":"
		}
		result += field.ValueType.String()
	}
	result = strings.TrimSuffix(result, "-")
	if field.IsStruct {
		//if !field.IsMapParam {
		result += "}"
		//}
	}
	return result
}

/*
BuildDBSchema builds DB schema or reuses existing one from cache
*/
func BuildDBSchema(t reflect.Type) (DBField, string) {
	// Check cache
	get, has := dbFieldSchemas[t]
	if has {
		return get.Key, get.Value
	}

	// Generate structure
	schema := buildDBSchemaField(t, "", -1)
	fmt.Println("making " + schema.ValueType.Name())
	if !schema.IsCustomDBType && schema.ValueType.Kind() == reflect.Struct {
		// Check for ICustomDBType
		if schema.ValueType.Implements(reflect.TypeFor[ICustomDBType]()) || reflect.PointerTo(schema.ValueType).Implements(reflect.TypeFor[ICustomDBType]()) {
			RegisterCustomDBTypeReflect(schema.ValueType)
			schema.IsCustomDBType = true
		} else {
			// Build struct
			schema.IsStruct = true
			schema.Fields = make([]DBField, 0)
			for i := 0; i < schema.ValueType.NumField(); i++ {
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
				schema.Fields = append(schema.Fields, fieldDB)
			}
		}
	}
	if schema.IsMap {
		// Build map
		schema.Fields = make([]DBField, 0)
		fieldDB, _ := BuildDBSchema(schema.ValueType.Key())
		fieldDB.Name = "mapKey"
		fieldDB.Index = -10
		fieldDB.IsMapParam = true
		schema.Fields = append(schema.Fields, fieldDB)
		fieldDB, _ = BuildDBSchema(schema.ValueType.Elem())
		fieldDB.Name = "mapVal"
		fieldDB.Index = -11
		fieldDB.IsMapParam = true
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
		// Slice
		schemaLocal := schema
		schemaLocal.IsSlice = false

		// Write length
		err := ConvertDynamicUintToBytesDB(writer, uint64(v.Len()))
		if err != nil {
			return err
		}

		// Write data
		for i := 0; i < v.Len(); i++ {
			err = convertFieldValueToBytesDB(writer, schemaLocal, v.Index(i))
			if err != nil {
				return err
			}
		}
		return nil
	}
	if schema.IsMap {
		// Map
		// Write length
		err := ConvertDynamicUintToBytesDB(writer, uint64(v.Len()))
		if err != nil {
			return err
		}

		// Write data
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
		// User defined type
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		fmt.Println(v.Type().String())

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
	if schema.Fields == nil {
		// Normal end value
		return convertAnyValueToBytesDBValue(writer, schema.ValueType.Kind(), v)
	}
	// Struct
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
	// Try to convert some basic type
	v := reflect.ValueOf(data)
	err := convertAnyValueToBytesDBValue(writer, v.Kind(), v)
	if err != nil && !errors.Is(os.ErrInvalid, err) {
		return err
	}

	// Convert complex types
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
		for _, t := range registeredCustomTypes {
			if t.String() == valType {
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
		}
		return nil, os.ErrInvalid
	}
	return result, err
}

func parseAnyValueKindToBytesDBValue(reader io.Reader, k reflect.Kind, objectValue *reflect.Value, createdNew bool, interactiveRepair bool) (reflect.Value, error) {
	//var err error
	//var result any
	//switch k {
	//case reflect.Bool:
	//	result, err = ParseBoolDB(reader)
	//	break
	//case reflect.Uint:
	//	result, err = ParseUint64DB(reader)
	//	result = uint(result.(uint64))
	//	break
	//case reflect.Int:
	//	result, err = ParseUint64DB(reader)
	//	result = int(int64(result.(uint64)))
	//	break
	//case reflect.Uint64:
	//	result, err = ParseUint64DB(reader)
	//	break
	//case reflect.Int64:
	//	result, err = ParseUint64DB(reader)
	//	result = int64(result.(uint64))
	//	break
	//case reflect.Uint8:
	//	result, err = ParseUint8DB(reader)
	//	break
	//case reflect.Int8:
	//	result, err = ParseBoolDB(reader)
	//	result = int8(result.(uint8))
	//	break
	//case reflect.String:
	//	result, err = ParseStringDB(reader)
	//	break
	//case reflect.Int16:
	//	result, err = ParseUint16DB(reader)
	//	result = int16(result.(uint16))
	//	break
	//case reflect.Int32:
	//	result, err = ParseUint32DB(reader)
	//	result = int32(result.(uint32))
	//	break
	//case reflect.Uint16:
	//	result, err = ParseUint16DB(reader)
	//	break
	//case reflect.Uint32:
	//	result, err = ParseUint32DB(reader)
	//	break
	//default:
	//	return reflect.ValueOf(nil), os.ErrInvalid
	//}
	result, err := parseAnyValueToBytesDBValue(reader, k.String(), objectValue, createdNew, interactiveRepair)
	if err != nil {
		return reflect.ValueOf(nil), err
	}
	return reflect.ValueOf(result), nil
}

var anyType = reflect.TypeFor[any]()

func checkIsAny(t reflect.Type) bool {
	if t.Kind() == reflect.Pointer || t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return anyType == t
}

func buildStructSchemaStringParts(schemaString string, newPos int) ([]string, int) {
	braceDepth := 1
	schemaParts := make([]string, 0)
	currentPart := ""
	for newPos < len(schemaString)-1 {
		newPos++
		if schemaString[newPos] == '{' || schemaString[newPos] == '<' {
			// Is opening brace
			braceDepth++
		}
		if schemaString[newPos] == '}' || schemaString[newPos] == '>' {
			// Is closing brace
			braceDepth--
			//if braceDepth == 1 {
			//	schemaParts = append(schemaParts, currentPart)
			//	currentPart = ""
			//	continue
			//}
		}
		if braceDepth > 1 {
			// Write whole child struct
			currentPart += string(schemaString[newPos])
		}
		if braceDepth == 1 {
			// Normal layer
			if schemaString[newPos] == '-' {
				schemaParts = append(schemaParts, currentPart)
				currentPart = ""
				continue
			}
			currentPart += string(schemaString[newPos])
		}
		if braceDepth == 0 {
			schemaParts = append(schemaParts, currentPart)
			break
		}
	}
	return schemaParts, newPos
}

func getSeekPos(reader io.ReadSeeker) {
	seek, _ := reader.Seek(0, io.SeekCurrent)
	fmt.Println("Pos at file:", seek)
}

func readDataDBAny(reader io.ReadSeeker, schemaString string, schemaStringPos int, interactiveRepair bool) (int, any, error) {
	fmt.Println("Reading data any:", schemaString, schemaStringPos)
	getSeekPos(reader)
	if schemaString[schemaStringPos] == '[' {
		// Is array
		schemaStringPos += 2
		count, err := ParseDynamicUintBytesDB(reader)
		if err != nil {
			return schemaStringPos, true, err
		}
		fmt.Println("Reading array any:", count)
		getSeekPos(reader)

		// Read items
		newPos := schemaStringPos
		result := make([]any, 0)
		for i := uint64(0); i < count; i++ {
			pos, val, err := readDataDBAny(reader, schemaString, schemaStringPos, interactiveRepair)
			if pos > newPos {
				newPos = pos
			}
			if err != nil {
				return schemaStringPos, result, err
			}
			result = append(result, val)
		}
		return newPos, result, nil
	}
	if schemaString[schemaStringPos] == '<' {
		// Is map
		schemaParts, newPos := buildStructSchemaStringParts(schemaString, schemaStringPos)
		count, err := ParseDynamicUintBytesDB(reader)
		if err != nil {
			return schemaStringPos, true, err
		}
		fmt.Println("Reading any map with schema parts:", schemaParts, "with count:", count)

		// Read items
		m := make(map[any]any, 0)
		for i := uint64(0); i < count; i++ {
			// Read key
			_, key, err := readDataDBAny(reader, schemaParts[0], 0, interactiveRepair)
			if err != nil {
				return schemaStringPos, true, err
			}

			// Read val
			_, val, err := readDataDBAny(reader, schemaParts[1], 0, interactiveRepair)
			if err != nil {
				return schemaStringPos, true, err
			}
			m[key] = val
		}
		return newPos, m, nil
	}
	if schemaString[schemaStringPos] == '{' {
		// Is struct
		schemaParts, newPos := buildStructSchemaStringParts(schemaString, schemaStringPos)
		fmt.Println("Reading any struct with schema parts:", schemaParts)

		// Run each subschema
		m := make(map[string]any, 0)
		for _, schema := range schemaParts {
			_, item, err := readDataDBAny(reader, schema, 0, interactiveRepair)
			if err != nil {
				return newPos, m, err
			}
			if item == nil {
				return newPos, m, os.ErrNotExist
			}
			//name := strings.SplitN(schema, "-", 2)[0]
			for k, v := range item.(map[string]any) {
				m[k] = v
			}
		}
		return newPos, m, nil
	}

	// Remove name and parse
	split := strings.SplitN(schemaString[schemaStringPos:], ":", 2)
	if len(split) == 2 {
		newPos, val, err := readDataDBAny(reader, split[1], 0, interactiveRepair)
		if err != nil {
			return schemaStringPos + newPos, nil, err
		}
		result := make(map[string]any, 0)
		result[split[0]] = val
		return schemaStringPos + newPos, result, err
	} else {
		// Normal type - do parse by string
		fmt.Println("Reading any value:", schemaString)
		val, err := parseAnyValueToBytesDBValue(reader, split[0], nil, true, interactiveRepair)
		getSeekPos(reader)
		return len(schemaString), val, err
	}
}

func readDataDB(reader io.ReadSeeker, target *reflect.Value, schemaString string, schemaStringPos int, createdNew bool, interactiveRepair bool) (int, error) {
	fmt.Println("Reading data:", schemaString, schemaStringPos)
	getSeekPos(reader)
	if schemaString[schemaStringPos] == '[' {
		//Target is nil
		if target == nil {
			//Move to any
			fmt.Println("Skipping array")
			newPos, _, err := readDataDBAny(reader, schemaString, schemaStringPos, interactiveRepair)
			return newPos, err
		}

		// Is array
		schemaStringPos += 2
		count, err := ParseDynamicUintBytesDB(reader)
		if err != nil {
			return schemaStringPos, err
		}
		fmt.Println("Reading array:", count)
		getSeekPos(reader)

		//Create slice
		slice := *target
		if slice.Kind() == reflect.Pointer {
			slice = slice.Elem()
		}
		fmt.Println(slice.Type().String())
		newSlice := slice
		if newSlice.Len() == 0 {
			newSlice = reflect.MakeSlice(slice.Type(), 0, int(count))
		}

		// Read items
		for i := uint64(0); i < count; i++ {
			var element reflect.Value
			var isNew bool = false
			if newSlice.Len() <= int(i) {
				element = reflect.New(slice.Type().Elem()).Elem()
				isNew = true
			} else {
				element = newSlice.Index(int(i))
			}
			fmt.Println(element.Kind(), element.String())
			_, err := readDataDB(reader, &element, schemaString, schemaStringPos, isNew || createdNew, interactiveRepair)
			if err != nil {
				return schemaStringPos, err
			}
			if isNew {
				newSlice = reflect.Append(newSlice, element)
			}
		}
		slice.Set(newSlice)
		return len(schemaString), nil
	}
	if schemaString[schemaStringPos] == '<' {
		//Target is nil
		if target == nil {
			//Move to any
			fmt.Println("Skipping map")
			newPos, _, err := readDataDBAny(reader, schemaString, schemaStringPos, interactiveRepair)
			return newPos, err
		}

		// Is map
		schemaParts, newPos := buildStructSchemaStringParts(schemaString, schemaStringPos)
		count, err := ParseDynamicUintBytesDB(reader)
		if err != nil {
			return schemaStringPos, err
		}
		fmt.Println("Reading map with schema parts:", schemaParts, "with count:", count)

		//Create map
		m := *target
		var isNewMap = false
		if m.Kind() == reflect.Pointer {
			if m.IsNil() {
				m.Set(reflect.New(m.Type().Elem()))
				isNewMap = true
			}
			m = m.Elem()
		}
		newMap := m
		if newMap.Len() == 0 {
			newMap = reflect.MakeMap(m.Type())
		}

		// Read items
		for i := uint64(0); i < count; i++ {
			// Read key
			key := reflect.New(m.Type().Key()).Elem()
			_, err := readDataDB(reader, &key, schemaParts[0], 0, true, interactiveRepair)
			if err != nil {
				return newPos, err
			}

			// Read val
			mapVal := newMap.MapIndex(key)
			var val reflect.Value
			var isNewVal = false
			if mapVal.IsValid() {
				val = reflect.New(mapVal.Type()).Elem()
				val.Set(mapVal)
			} else {
				val = reflect.New(newMap.Type().Elem()).Elem()
				isNewVal = true
			}
			_, err = readDataDB(reader, &val, schemaParts[1], 0, isNewMap || isNewVal || createdNew, interactiveRepair)
			if err != nil {
				return newPos, err
			}
			newMap.SetMapIndex(key, val)
		}
		m.Set(newMap)
		return newPos, nil
	}
	if schemaString[schemaStringPos] == '{' {
		//Target is nil
		if target == nil {
			//Move to any
			fmt.Println("Skipping struct")
			newPos, _, err := readDataDBAny(reader, schemaString, schemaStringPos, interactiveRepair)
			return newPos, err
		}

		// Is struct - Parse to field
		if target.Kind() == reflect.Pointer {
			if target.IsNil() {
				target.Set(reflect.New(target.Type().Elem()))
			}
			e := target.Elem()
			target = &e
		}

		//Get field
		field, _ := BuildDBSchema(target.Type())
		schemaParts, newPos := buildStructSchemaStringParts(schemaString, schemaStringPos)
		fmt.Println("Reading struct with schema parts:", schemaParts)

		// Run each subschema
		for _, schema := range schemaParts {
			split := strings.SplitN(schema, ":", 2)
			fieldFound := false
			for _, field := range field.Fields {
				if field.Name == split[0] {
					// Found field
					fieldFound = true
					field := target.Field(field.Index)
					_, err := readDataDB(reader, &field, split[1], 0, createdNew, interactiveRepair)
					if err != nil {
						return newPos, err
					}
					break
				}
			}

			//Field not found
			if !fieldFound {
				_, err := readDataDB(reader, nil, split[1], 0, createdNew, interactiveRepair)
				if err != nil {
					return newPos, err
				}
			}
		}
		return newPos, nil
	}

	// Normal type - do parse by string
	split := strings.SplitN(schemaString[schemaStringPos:], ":", 2)
	typeString := ""
	if len(split) == 1 {
		//Only one part = type
		typeString = split[0]
	} else {
		typeString = split[1]
	}
	if target != nil && typeString != target.Type().String() {
		fmt.Println("Types do not match:", typeString, target.Type().String())
		return len(schemaString), os.ErrInvalid
	}

	//Read value from binary
	val, err := parseAnyValueToBytesDBValue(reader, typeString, target, createdNew, interactiveRepair)
	if err != nil {
		return len(schemaString), err
	}
	if target == nil {
		return len(schemaString), nil
	}

	//Convert to valid format
	newVal := reflect.ValueOf(val)
	fmt.Println(target.Kind(), newVal.Kind(), target.Type().String(), newVal.Type().String())
	if target.Kind() != reflect.Pointer && newVal.Kind() == reflect.Pointer {
		newVal = newVal.Elem()
	}
	if target.Kind() == reflect.Pointer && newVal.Kind() != reflect.Pointer {
		newVal = newVal.Addr()
	}
	fmt.Println(target.Kind(), newVal.Kind(), target.Type().String(), newVal.Type().String())
	fmt.Println(target.CanSet(), createdNew)
	target.Set(newVal)
	return len(schemaString), nil
}

/*
ParseAnyDB parses bytes to generic T object (can parse any type)
*/
func ParseAnyDB[T any](reader io.ReadSeeker, interactiveRepair bool) (T, error) {
	// Try to convert some basic type
	val, err := parseAnyValueKindToBytesDBValue(reader, reflect.TypeFor[T]().Kind(), nil, true, interactiveRepair)
	if err == nil {
		// OK
		result := val.Interface()
		return result.(T), nil
	}
	if !errors.Is(os.ErrInvalid, err) {
		// Invalid type (read error)
		var zero T
		return zero, err
	}

	// Convert complex types
	result := new(T)
	err = ParseAnyToObjectDB(reader, result, interactiveRepair)
	return *result, err
}

/*
ParseAnyToObjectDB parses bytes to generic target object (can parse only complex types)
*/
func ParseAnyToObjectDB(reader io.ReadSeeker, target any, interactiveRepair bool) error {
	// Check if target is pointer
	if target == nil {
		return os.ErrInvalid
	}
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Pointer && v.Kind() != reflect.Ptr {
		return os.ErrInvalid
	}

	// Read structure string
	structString, err := ParseStringDB(reader)
	if err != nil {
		return err
	}
	fmt.Println(structString)
	if checkIsAny(reflect.TypeOf(target)) {
		return os.ErrInvalid
	} else {
		_, err = readDataDB(reader, &v, structString, 0, false, interactiveRepair)
		return err
	}
}

/*
ParseAnyToObjectDB parses bytes to generic target object (can parse only complex types) -> Returns map or array of maps
*/
func ParseAnyToValueMapDB(reader io.ReadSeeker, interactiveRepair bool) (any, error) {
	// Read structure string
	structString, err := ParseStringDB(reader)
	if err != nil {
		return nil, err
	}
	fmt.Println(structString)

	// Read data to map
	_, result, err := readDataDBAny(reader, structString, 0, interactiveRepair)
	return result, err
}

/*func buildDBSchemaFromString(schema string) DBField {
	braceDepth := 0
	braceStart := 0
	writingName := true
	typeName := ""
	currentField := DBField{Name: "", Index: -1}
	for i := 0; i < len(schema); i++ {
		if schema[i] == '[' && schema[i+1] == ']' {
			//Is slice
			currentField.IsSlice = true
			i++
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
			continue
		}
		if writingName {
			currentField.Name += string(schema[i])
		} else {
			typeName += string(schema[i])
		}
	}
	return currentField
}*/

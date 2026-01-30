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
DBField is field holder for dynamic database object
*/
type DBField struct {
	Name      string
	Index     int
	IsSlice   bool
	IsMap     bool
	Type      reflect.Type
	ValueType reflect.Type
	Fields    []DBField
}

var dbFieldSchemas map[reflect.Type]webtools.KeyValuePair[DBField, string] = map[reflect.Type]webtools.KeyValuePair[DBField, string]{}

func buildDBSchemaField(t reflect.Type, name string, index int) DBField {
	tElem := t
	if t.Kind() == reflect.Pointer || t.Kind() == reflect.Ptr {
		tElem = t.Elem()
	}
	isSlice := false
	isMap := false
	if tElem.Kind() == reflect.Slice {
		//Is slice
		tElem = tElem.Elem()
		isSlice = true
	}
	if tElem.Kind() == reflect.Map {
		//Is map
		isMap = true
	}
	return DBField{
		Name:      name,
		Index:     index,
		Type:      t,
		ValueType: tElem,
		IsSlice:   isSlice,
		IsMap:     isMap,
	}
}

func buildDBSchemaString(field DBField) string {
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
			if v.ValueType.Kind() == reflect.Struct {
				result += buildDBSchemaString(v)
			} else {
				if v.IsSlice {
					result += "[]"
				}
				if v.IsMap {
					//Is Map
					//panic("Fix map")
					result += "map<" + buildDBSchemaString(v.Fields[0]) + "-" + buildDBSchemaString(v.Fields[1]) + ">"
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
	fmt.Println("making")
	schema := buildDBSchemaField(t, "", -1)
	if schema.ValueType.Kind() == reflect.Struct {
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
	schemaString := buildDBSchemaString(schema)
	dbFieldSchemas[t] = webtools.KeyValuePair[DBField, string]{Key: schema, Value: schemaString}
	return schema, schemaString
}

func convertAnyValueToBytesDBValue(writer io.Writer, k reflect.Kind, v reflect.Value) error {
	switch k {
	case reflect.Bool:
		return ConvertBoolToBytesDB(writer, v.Bool())
	case reflect.Uint, reflect.Uint64:
		return ConvertUint64ToBytesDB(writer, v.Uint())
	case reflect.Int, reflect.Int64:
		return ConvertUint64ToBytesDB(writer, uint64(v.Int()))
	case reflect.Uint8:
		return ConvertUint8ToBytesDB(writer, uint8(v.Uint()))
	case reflect.Int8:
		return ConvertUint8ToBytesDB(writer, uint8(v.Int()))
	case reflect.String:
		return ConvertStringToBytesDB(writer, v.String())
	case reflect.Int16:
		return ConvertUint16ToBytesDB(writer, uint16(v.Int()))
	case reflect.Int32:
		return ConvertUint32ToBytesDB(writer, uint32(v.Int()))
	case reflect.Uint16:
		return ConvertUint16ToBytesDB(writer, uint16(v.Uint()))
	case reflect.Uint32:
		return ConvertUint32ToBytesDB(writer, uint32(v.Uint()))
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
	fmt.Println("writing: " + buildDBSchemaString(schema))
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
		err := ConvertUint64ToBytesDB(writer, uint64(v.Len()))
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
		err := ConvertUint64ToBytesDB(writer, uint64(v.Len()))
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
	case reflect.Uint, reflect.Uint64:
		result, err = ParseUint64DB(reader)
		break
	case reflect.Int, reflect.Int64:
		result, err = ParseUint64DB(reader)
		result = result.(int64)
		break
	case reflect.Uint8:
		result, err = ParseUint8DB(reader)
		break
	case reflect.Int8:
		result, err = ParseBoolDB(reader)
		result = result.(int8)
		break
	case reflect.String:
		result, err = ParseStringDB(reader)
		break
	case reflect.Int16:
		result, err = ParseUint16DB(reader)
		result = result.(int16)
		break
	case reflect.Int32:
		result, err = ParseUint32DB(reader)
		result = result.(int32)
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
func ParseAnyDB[T any](writer io.Writer) (T, error) {

}

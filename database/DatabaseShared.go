/*
Package database provides some databases for some kinds of use cases
Please keep in mind that these databases are really simple
*/
package database

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"time"

	webtools "github.com/kolojar/Go-Webtools"
	"github.com/kolojar/Go-Webtools/encryption"
)

/*
DBField is field holder
*/
/*type DBField struct {
	Name    string
	Index   int
	Type    reflect.Type
	IsSlice bool
	Fields  []*DBField
}*/

//var DBFieldSchemas map[reflect.Type]webtools.KeyValuePair[DBField, string] = make(map[reflect.Type]webtools.KeyValuePair[DBField, string])

/*func buildDBSchemaStringOLD(field *DBField) string {
	//Resolve array
	result := ""
	if field.IsSlice {
		result += "[]"
	}
	//Write all fields
	result += "{"
	for _, v := range field.Fields {
		result += v.Name + ":"
		if v.Type.Kind() == reflect.Struct {
			result += buildDBSchemaString(v)
		} else {
			if v.IsSlice {
				result += "[]"
			}
			result += v.Type.String()
			if v.Type.Kind() == reflect.Map {
				//Is Map
				result += "<" + v.Fields[0].Type.String() + "," + v.Fields[1].Type.String() + ">"
			}
		}
		result += "-"
	}
	result = strings.TrimSuffix(result, "-")
	result += "}"
	return result
}*/

/*
BuildDBSchema creates schema of object and saves it in cache
*/
/*func BuildDBSchemaOLD(t reflect.Type) (*DBField, string) {
	fieldGet, has := DBFieldSchemas[t]
	if has {
		return &fieldGet.Key, fieldGet.Value
	}
	fmt.Println("making")
	schema := DBField{Fields: make([]*DBField, 0), Type: t, Name: t.Name()}

	if t.Kind() == reflect.Struct {
		//Go trought all fields
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			nameDB := field.Tag.Get("db")
			if nameDB == "-" {
				//Ignored
				continue
			} else if nameDB == "" {
				nameDB = field.Name
			}

			//Create field
			var fieldChild *DBField
			var isSlice bool = false
			fieldType := field.Type
			if fieldType.Kind() == reflect.Slice {
				isSlice = true
				fieldType = fieldType.Elem()
			}
			if fieldType.Kind() == reflect.Struct && field.Type.String() != "time.Time" {
				//Create subschema
				fieldChild, _ = BuildDBSchema(fieldType)
				fieldChild.Name = nameDB
				fieldChild.Index = i
				fieldChild.IsSlice = isSlice
			} else if fieldType.Kind() == reflect.Map {
				//Map
				fieldChild = &DBField{
					Name:    nameDB,
					Index:   i,
					Type:    fieldType,
					IsSlice: isSlice,
					Fields:  make([]*DBField, 0),
				}
				fieldChild.Fields = append(fieldChild.Fields, &DBField{
					Name:    "key",
					Index:   -1,
					Type:    fieldType.Key(),
					IsSlice: false,
					Fields:  nil,
				})
				fieldChild.Fields = append(fieldChild.Fields, &DBField{
					Name:    "value",
					Index:   -1,
					Type:    fieldType.Elem(),
					IsSlice: false,
					Fields:  nil,
				})
			} else {
				//Add end field
				fieldChild = &DBField{
					Name:    nameDB,
					Type:    fieldType,
					Index:   i,
					Fields:  nil,
					IsSlice: isSlice,
				}
			}
			schema.Fields = append(schema.Fields, fieldChild)
		}
	} else {
		//Not pure struct
		var isSlice = false
		if t.Kind() == reflect.Slice {
			isSlice = true
			t = t.Elem()
		}
		if t.Kind() == reflect.Struct {
			//Struct
			schemaPointer, _ := BuildDBSchema(t)
			schema = *schemaPointer
			schema.IsSlice = isSlice
		} else if t.Kind() == reflect.Map {
			//Map
			schema = DBField{
				Name:    "field",
				Index:   0,
				Type:    t,
				IsSlice: isSlice,
				Fields:  make([]*DBField, 2),
			}
			schema.Fields = append(schema.Fields, &DBField{
				Name:    "key",
				Index:   -1,
				Type:    t.Key(),
				IsSlice: false,
				Fields:  nil,
			})
			schema.Fields = append(schema.Fields, &DBField{
				Name:    "value",
				Index:   -1,
				Type:    t.Elem(),
				IsSlice: false,
				Fields:  nil,
			})
		} else {
			//Normal value
			schema = DBField{
				Name:    "field",
				Index:   0,
				Type:    t,
				IsSlice: isSlice,
				Fields:  nil,
			}
		}
	}
	//Build string
	schemaString := buildDBSchemaString(&schema)
	DBFieldSchemas[t] = webtools.KeyValuePair[DBField, string]{Key: schema, Value: schemaString}
	return &schema, schemaString
}*/

//func convertAnyToBytesDBValue(writer io.Writer, v reflect.Value) error {
//	switch v.Kind() {
//	case reflect.Int, reflect.Uint, reflect.Int64, reflect.Uint64:
//		{
//			return ConvertUint64ToBytesDB(writer, v.Uint())
//		}
//	case reflect.Int8, reflect.Uint8:
//		{
//			return ConvertUint8ToBytesDB(writer, uint8(v.Uint()))
//		}
//	case reflect.String:
//		{
//			return ConvertStringToBytesDB(writer, v.String())
//		}
//	case reflect.Bool:
//		{
//			return ConvertBoolToBytesDB(writer, v.Bool())
//		}
//	case reflect.Map:
//		{
//			m := reflect.MakeMap(v.Type())
//			for _, k := range v.MapKeys() {
//				m.SetMapIndex(k, v.MapIndex(k))
//			}
//			return ConvertMapToBytesDB(writer, m.Interface().(map[any]any), convertAnyValueToBytesDBValue, convertAnyValueToBytesDBValue)
//		}
//	case reflect.Array:
//		{
//			return ConvertSliceToBytesDB(writer, v.Interface().([]any), convertAnyValueToBytesDBValue)
//		}
//	default:
//		{
//			panic("unknow type or unsupported: " + v.Kind().String())
//		}
//	}
//}

/*func convertAnyToBytesDBValues(writer io.Writer, v reflect.Value, schema *DBField) error {
	if schema.IsSlice {
		//Value is slice
		err := ConvertUint64ToBytesDB(writer, uint64(v.Len()))
		if err != nil {
			return err
		}
		for i := 0; i < v.Len(); i++ {
			//Write each field
			fieldVal := v.Index(i)
			if schema.Fields != nil {
				//Write struct
				err := convertAnyToBytesDBValues(writer, fieldVal, schema.Fields[i])
				if err != nil {
					return err
				}
			}
			//Write clasic field
			err := convertAnyToBytesDBValue(writer, fieldVal)
			if err != nil {
				return err
			}
		}
	}
	for _, field := range schema.Fields {
		//Write each field
		fieldVal := v.Field(field.Index)
		if field.Fields != nil {
			//Write struct
			err := convertAnyToBytesDBValues(writer, fieldVal, field)
			if err != nil {
				return err
			}
		}
		//Write clasic field
		err := convertAnyToBytesDBValue(writer, fieldVal)
		if err != nil {
			return err
		}
	}
	return nil
}*/

/*func ConvertAnyToBytesDB(writer io.Writer, data any) error {
	schema, schemaString := BuildDBSchema(reflect.TypeOf(data))
	//Write schema string
	_, err := writer.Write([]byte(schemaString))
	if err != nil {
		return err
	}

	//Write fields
	return convertAnyToBytesDBValues(writer, reflect.ValueOf(data), schema)
}*/

/*
ConvertAnyToBytesDB converts any to bytes
*/
/*func ConvertAnyToBytesDB(writer io.Writer, data any) error {
	valueOf := reflect.ValueOf(data)
	if valueOf.Kind() == reflect.String {
		return ConvertStringToBytesDB(writer, data.(string))
	}
	if valueOf.Kind() == reflect.Uint64 || valueOf.Kind() == reflect.Int64 {
		return ConvertUint64ToBytesDB(writer, data.(uint64))
	}
	if valueOf.Kind() == reflect.Uint8 || valueOf.Kind() == reflect.Int8 {
		return ConvertUint8ToBytesDB(writer, data.(uint8))
	}
	if valueOf.Kind() == reflect.Bool {
		return ConvertBoolToBytesDB(writer, data.(bool))
	}
	if valueOf.Kind() == reflect.Slice {
		return ConvertSliceToBytesDB(writer, data.([]any), ConvertAnyToBytesDB)
	}
	if valueOf.Kind() == reflect.Map {
		return ConvertMapToBytesDB(writer, data.(map[any]any), ConvertAnyToBytesDB, ConvertAnyToBytesDB)
	}
	if valueOf.Kind() == reflect.Func {
		fmt.Println("Converting FUNC")
		return os.ErrInvalid
	}
	if valueOf.Kind() == reflect.Chan {
		fmt.Println("Converting CHAN")
		return os.ErrInvalid
	}
	if valueOf.Kind() == reflect.Invalid {
		fmt.Println("Converting INVALID")
		return os.ErrInvalid
	}
	if valueOf.Kind() == reflect.Interface {
		fmt.Println("Converting INTERFACE")
		return os.ErrInvalid
	}
	if valueOf.Kind() == reflect.Pointer {
		fmt.Println("Converting POINTER")
		return os.ErrInvalid
	}
	if valueOf.Kind() == reflect.Array {
		fmt.Println("Converting ARRAY")
		return os.ErrInvalid
	}
	if valueOf.Kind() == reflect.Struct {
		//Struct, pass each values
		fieldNames := []string{}
		buf := bytes.Buffer{}
		typeOf := valueOf.Type()
		for i := 0; i < valueOf.NumField(); i++ {
			val := valueOf.Field(i)
			field := typeOf.Field(i)
			fieldNames = append(fieldNames, field)
			if val.CanInterface() {
				err := ConvertAnyToBytesDB(&buf, val.Interface())
				if err != nil {
					return err
				}
			}
		}
	}
}*/

/*
IDatabaseObject adds support for databases to use this objects
*/
type IDatabaseObject interface {
	ConvertToBytesDB(writer io.Writer) error
	ParseBytesDB(reader io.Reader) error
}

/*
ConvertStringToBytesDB converts string to bytes
*/
func ConvertStringToBytesDB(writer io.Writer, data string) error {
	dataString := []byte(data)

	//Write length
	err := ConvertDynamicUintToBytesDB(writer, uint64(len(dataString)))
	if err != nil {
		return err
	}

	//Write data
	_, err = writer.Write(dataString)
	return err
}

/*
ParseStringDB parses bytes from reader to string
*/
func ParseStringDB(reader io.Reader) (string, error) {
	//Read length
	lenght, err := ParseDynamicUintBytesDB(reader)
	if err != nil {
		return "", err
	}

	//Read data
	data := make([]byte, lenght)
	_, err = reader.Read(data)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

/*
ConvertUintXToBytesDB converts uint64 to X/8 bytes. Parameter size is X = 8 bites = 1 byte = uint8, ...
*/
func ConvertUintXToBytesDB(writer io.Writer, data uint64, size uint8) error {
	//Write number
	dataByte := make([]byte, 8)
	binary.LittleEndian.PutUint64(dataByte, data)
	_, err := writer.Write(dataByte[0:(webtools.CeilDivision(size, 8))])
	return err
}

/*
ParseUintXDB parses X/8 bytes from reader to uint64. Parameter size is X = 8 bites = 1 byte = uint8, ...
*/
func ParseUintXDB(reader io.Reader, size uint8) (uint64, error) {
	//Read number
	dataByte := make([]byte, webtools.CeilDivision(size, 8))
	_, err := reader.Read(dataByte)
	if err != nil {
		return 0, err
	}

	//Convert number
	parseByte := make([]byte, 8)
	copy(parseByte, dataByte)
	return binary.LittleEndian.Uint64(parseByte), nil
}

/*
ConvertDynamicUintToBytesDB converts uint64 to dynamic count of bytes
*/
func ConvertDynamicUintToBytesDB(writer io.Writer, data uint64) error {
	//Get byte size
	size := webtools.FormatByBool(data == 0, 0, calculateByteSizeFromInt(uint(data)))
	if size == 255 {
		return os.ErrInvalid
	}

	//Write byte size of length
	err := ConvertUint8ToBytesDB(writer, size)
	if err != nil || size == 0 {
		return err
	}

	//Write number
	return ConvertUintXToBytesDB(writer, data, size)
}

/*
ParseDynamicUintBytesDB parses to uint64 using dynamic count of bytes
*/
func ParseDynamicUintBytesDB(reader io.Reader) (uint64, error) {
	//Read byte size of length
	size, err := ParseUint8DB(reader)
	if err != nil || size == 0 {
		return 0, err
	}

	//Write number
	return ParseUintXDB(reader, size)
}

func calculateByteSizeFromInt(value uint) uint8 {
	for i := uint8(1); i < 9; i++ {
		if value <= (1 << (8 * i)) {
			return i
		}
	}
	return 255
}

/*
ConvertUint64ToBytesDB converts uint64 to bytes
*/
func ConvertUint64ToBytesDB(writer io.Writer, data uint64) error {
	//Write number
	dataByte := make([]byte, 8)
	binary.BigEndian.PutUint64(dataByte, data)
	_, err := writer.Write(dataByte)
	return err
}

/*
ParseUint64DB parses bytes from reader to uint64
*/
func ParseUint64DB(reader io.Reader) (uint64, error) {
	//Read number
	dataByte := make([]byte, 8)
	_, err := reader.Read(dataByte)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(dataByte), nil
}

/*
ConvertUint16ToBytesDB converts uint16 to bytes
*/
func ConvertUint16ToBytesDB(writer io.Writer, data uint16) error {
	//Write number
	dataByte := make([]byte, 2)
	binary.BigEndian.PutUint16(dataByte, data)
	_, err := writer.Write(dataByte)
	return err
}

/*
ParseUint16DB parses bytes from reader to uint16
*/
func ParseUint16DB(reader io.Reader) (uint16, error) {
	//Read number
	dataByte := make([]byte, 2)
	_, err := reader.Read(dataByte)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(dataByte), nil
}

/*
ConvertUint32ToBytesDB converts uint32 to bytes
*/
func ConvertUint32ToBytesDB(writer io.Writer, data uint32) error {
	//Write number
	dataByte := make([]byte, 4)
	binary.BigEndian.PutUint32(dataByte, data)
	_, err := writer.Write(dataByte)
	return err
}

/*
ParseUint32DB parses bytes from reader to uint32
*/
func ParseUint32DB(reader io.Reader) (uint32, error) {
	//Read number
	dataByte := make([]byte, 4)
	_, err := reader.Read(dataByte)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(dataByte), nil
}

/*
ConvertUint8ToBytesDB converts uint8 to bytes
*/
func ConvertUint8ToBytesDB(writer io.Writer, data uint8) error {
	//Write number
	_, err := writer.Write([]byte{data})
	return err
}

/*
ParseUint8DB parses bytes from reader to uint8
*/
func ParseUint8DB(reader io.Reader) (uint8, error) {
	//Read number
	dataByte := make([]byte, 1)
	_, err := reader.Read(dataByte)
	if err != nil {
		return 0, err
	}
	return dataByte[0], nil
}

///*
//LimitedStringDB is special data type for database that is string with maximum length, same as VARCHAR(n)
//*/
//type LimitedStringDB struct {
//	data      string
//	maxLength int32
//}
//
///*
//MakeLimitedStringDB creates new LimitedStringDB, if maxLangth is negative, than limit is not set
//*/
//func MakeLimitedStringDB(maxLength int32) LimitedStringDB {
//	return LimitedStringDB{maxLength: maxLength, data: ""}
//}
//
///*
//Get gets string
//*/
//func (str *LimitedStringDB) Get() string {
//	return str.data
//}
//
///*
//Set sets data, returns true when OK
//*/
//func (str *LimitedStringDB) Set(data string) bool {
//	if str.maxLength >= 0 && len(data) > int(str.maxLength) {
//		return false
//	}
//	str.data = data
//	return true
//}
//
///*
//ConvertToBytesDB converts string to data
//*/
//func (str *LimitedStringDB) ConvertToBytesDB(buffer *bytes.Buffer) error {
//	if str.maxLength < 0 {
//		ConvertStringToBytesDB(buffer, str.data)
//		return nil
//	}
//
//	buffer.WriteString()
//	return nil
//}
//
///*
//ParseBytesDB parses bytes from reader to string
//*/
//func (str *LimitedStringDB) ParseBytesDB(reader io.Reader) error {
//	if str.maxLength < 0 {
//		var err error
//		str.data, err = ParseStringDB(reader)
//		return err
//	}
//	data := make([]byte, str.maxLength)
//	_, err := reader.Read(data)
//	str.data = string(data)
//	return err
//}

/*
ParsePasswordObjectDB parses bytes from reader to PasswordObject
*/
func ParsePasswordObjectDB(reader io.Reader) (encryption.PasswordObject, error) {
	//Read salt
	var salt = make([]byte, 64)
	_, err := reader.Read(salt)
	if err != nil {
		return encryption.PasswordObject{}, err
	}

	//Read hash
	var hash = make([]byte, 64)
	_, err = reader.Read(hash)
	if err != nil {
		return encryption.PasswordObject{}, err
	}

	//Prepare salt
	saltString := hex.EncodeToString(salt)

	//Prepare hash
	hashString := hex.EncodeToString(hash)

	//Make object
	return encryption.PasswordObject{Salt: saltString, Hash: hashString}, nil
}

/*
ConvertPasswordObjectToBytesDB converts PasswordObject to bytes
*/
func ConvertPasswordObjectToBytesDB(writer io.Writer, data encryption.PasswordObject) error {
	//Prepare salt
	salt, err := hex.DecodeString(data.Salt)
	if err != nil {
		return err
	}
	if len(salt) != 64 {
		return errors.New("invalid salt size")
	}

	//Prepare hash
	hash, err := hex.DecodeString(data.Hash)
	if err != nil {
		return err
	}

	//Write salt
	writer.Write(salt)

	//Write hash
	writer.Write(hash)
	return nil
}

/*
ConvertBoolToBytesDB converts bool to bytes
*/
func ConvertBoolToBytesDB(writer io.Writer, data bool) error {
	//Write bool
	dataByte := make([]byte, 1)
	dataByte[0] = 1
	if data {
		dataByte[0] = 2
	}
	_, err := writer.Write(dataByte)
	return err
}

/*
ParseBoolDB parses bytes from reader to bool
*/
func ParseBoolDB(reader io.Reader) (bool, error) {
	//Read bool
	dataByte := make([]byte, 1)
	_, err := reader.Read(dataByte)
	if err != nil {
		return false, err
	}
	return dataByte[0] == 2, nil
}

/*
ConvertTimeToBytesDB converts time to bytes
*/
func ConvertTimeToBytesDB(writer io.Writer, data time.Time) {
	//Write time
	ConvertUint64ToBytesDB(writer, uint64(data.UnixNano()))
}

/*
ParseTimeDB parses bytes from reader to time
*/
func ParseTimeDB(reader io.Reader) (time.Time, error) {
	//Read time
	timeNum, err := ParseUint64DB(reader)
	if err != nil {
		return time.Unix(0, 0), err
	}
	return time.Unix(0, int64(timeNum)), nil
}

/*
ConvertMapToBytesDB converts map to bytes
*/
func ConvertMapToBytesDB[K comparable, V any](writer io.Writer, data map[K]V, keyConvertDBFunc func(writer io.Writer, data K) error, valueConvertDBFunc func(writer io.Writer, data V) error) error {
	if keyConvertDBFunc == nil || valueConvertDBFunc == nil {
		return os.ErrInvalid
	}
	err := ConvertDynamicUintToBytesDB(writer, uint64(len(data)))
	if err != nil {
		return err
	}
	for k, v := range data {
		err = keyConvertDBFunc(writer, k)
		if err != nil {
			return err
		}
		err = valueConvertDBFunc(writer, v)
		if err != nil {
			return err
		}
	}
	return nil
}

/*
ParseMapDB parses bytes from reader to map
*/
func ParseMapDB[K comparable, V any](reader io.Reader, keyParseDBFunc func(reader io.Reader) (K, error), valueParseDBFunc func(reader io.Reader) (V, error)) (map[K]V, error) {
	if keyParseDBFunc == nil || valueParseDBFunc == nil {
		return nil, os.ErrInvalid
	}

	//Read count
	count, err := ParseDynamicUintBytesDB(reader)
	if err != nil {
		return nil, err
	}

	//Read rows
	data := map[K]V{}
	for i := 0; i < int(count); i++ {
		key, err := keyParseDBFunc(reader)
		if err != nil {
			return data, err
		}
		value, err := valueParseDBFunc(reader)
		if err != nil {
			return data, err
		}
		data[key] = value
	}
	return data, nil
}

/*
ConvertSafeMapToBytesDB converts safeMap to bytes
*/
func ConvertSafeMapToBytesDB[K comparable, V any](writer io.Writer, data webtools.SafeMap[K, V], keyConvertDBFunc func(writer io.Writer, data K) error, valueConvertDBFunc func(writer io.Writer, data V) error) error {
	if keyConvertDBFunc == nil || valueConvertDBFunc == nil {
		return os.ErrInvalid
	}
	err := ConvertDynamicUintToBytesDB(writer, uint64(data.Len()))
	if err != nil {
		return err
	}
	for _, v := range data.GetData() {
		err = keyConvertDBFunc(writer, v.Key)
		if err != nil {
			return err
		}
		err = valueConvertDBFunc(writer, v.Value)
		if err != nil {
			return err
		}
	}
	return nil
}

/*
ParseSafeMapDB parses bytes from reader to safeMap
*/
func ParseSafeMapDB[K comparable, V any](reader io.Reader, keyParseDBFunc func(reader io.Reader) (K, error), valueParseDBFunc func(reader io.Reader) (V, error)) (webtools.SafeMap[K, V], error) {
	data := webtools.MakeSafeMap[K, V]()
	if keyParseDBFunc == nil || valueParseDBFunc == nil {
		return data, os.ErrInvalid
	}

	//Read count
	count, err := ParseDynamicUintBytesDB(reader)
	if err != nil {
		return data, err
	}

	//Read rows
	for i := 0; i < int(count); i++ {
		key, err := keyParseDBFunc(reader)
		if err != nil {
			return data, err
		}
		value, err := valueParseDBFunc(reader)
		if err != nil {
			return data, err
		}
		data.Set(key, value)
	}
	return data, nil
}

/*
ConvertSliceToBytesDB converts array to bytes
*/
func ConvertSliceToBytesDB[V any](writer io.Writer, data []V, convertDBFunc func(writer io.Writer, data V) error) error {
	if convertDBFunc == nil {
		return os.ErrInvalid
	}
	err := ConvertDynamicUintToBytesDB(writer, uint64(len(data)))
	if err != nil {
		return err
	}
	for _, v := range data {
		err = convertDBFunc(writer, v)
		if err != nil {
			return err
		}
	}
	return nil
}

/*
ParseArrayDB parses bytes from reader to array
*/
func ParseArrayDB[V any](reader io.Reader, parseDBFunc func(reader io.Reader) (V, error)) ([]V, error) {
	data := make([]V, 0)
	if parseDBFunc == nil {
		return data, os.ErrInvalid
	}

	//Read count
	count, err := ParseDynamicUintBytesDB(reader)
	if err != nil {
		return data, err
	}

	//Read rows
	for i := 0; i < int(count); i++ {
		val, err := parseDBFunc(reader)
		if err != nil {
			return nil, err
		}
		data = append(data, val)
	}
	return data, nil
}

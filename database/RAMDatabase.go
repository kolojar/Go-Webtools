package database

import (
	"bytes"
	"io"
	"os"
	"webtools"
)

/*
RAMDatabase is database that is completly stored in RAM and loaded from disk on start
*/
type RAMDatabase[T IDatabaseObject] struct {
	emptyObject T
	//oneValueLength uint64
	data   webtools.SafeMap[string, T]
	path   string
	Logger *webtools.ConsoleLogger
}

/*
NewRAMDatabase creates new RAM Database
*/
func NewRAMDatabase[T IDatabaseObject](path string, emptyObject T) (*RAMDatabase[T], error) {
	//Calculate one valueLength
	//emptyObjectBytes := bytes.NewBuffer(nil)
	//err := emptyObject.ConvertToBytesDB(emptyObjectBytes)
	//if err != nil {
	//	return nil, err
	//}

	//Create object
	var inst = RAMDatabase[T]{}
	//inst.oneValueLength = uint64(emptyObjectBytes.Len())
	inst.data = webtools.MakeSafeMap[string, T]()
	inst.path = path
	inst.Logger = webtools.NewConsoleLoggerForTraffic("RAMDB", false)
	inst.emptyObject = emptyObject
	return &inst, nil
}

/*
Get gets value from database
*/
func (db *RAMDatabase[T]) Get(key string) T {
	return db.data.Get(key)
}

/*
Set set value to database
*/
func (db *RAMDatabase[T]) Set(key string, value T) {
	db.data.Set(key, value)
}

/*
Delete deletes value from database
*/
func (db *RAMDatabase[T]) Delete(key string) {
	db.data.Delete(key)
}

/*
Save saves data of database to disk
*/
func (db *RAMDatabase[T]) Save() error {
	//Create DB file
	db.Logger.Log(2, "Saving database, please wait...")
	file, err := os.Create(db.path)
	if err != nil {
		db.Logger.Log(3, "Error saving database: "+err.Error())
		return err
	}
	defer file.Close()

	//Write length of one value
	//binary.Write(file, binary.BigEndian, db.oneValueLength)

	//Write map values
	for _, v := range db.data.GetData() {
		result := bytes.NewBuffer(nil)
		ConvertStringToBytesDB(result, v.Key)
		v.Value.ConvertToBytesDB(result)
		result.WriteTo(file)
	}
	db.Logger.Log(2, "Database saved.")
	return nil
}

/*
Load Load data of database from disk
*/
func (db *RAMDatabase[T]) Load() error {
	//Open DB file
	db.Logger.Log(2, "Loading database, please wait...")
	file, err := os.Open(db.path)
	if err != nil {
		db.Logger.Log(3, "Error loading database: "+err.Error())
		return err
	}
	defer file.Close()

	//Read length of one value
	//err = binary.Read(file, binary.BigEndian, db.oneValueLength)
	//if err != nil {
	//	db.Logger.Log(3, "Error loading database: "+err.Error())
	//	return err
	//}

	//Read map values
	for {
		//Read key
		key, err := ParseStringDB(file)
		if err != nil {
			if err == io.EOF {
				break
			}
			db.Logger.Log(3, "Error loading database key: "+err.Error())
			return err
		}

		//Read value
		value := db.emptyObject
		err = value.ParseBytesDB(file)
		if err != nil {
			if err == io.EOF {
				break
			}
			db.Logger.Log(3, "Error loading database value: "+err.Error())
			return err
		}

		//Set to map
		db.data.Set(key, value)
	}

	db.Logger.Log(2, "Database loaded.")
	return nil
}

package database

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"webtools"
)

/*
RAMDatabase is database that is completly stored in RAM and loaded from disk on start and saved everytime some change is made
*/
type RAMDatabase[T IDatabaseObject] struct {
	emptyObject    T
	oneValueLength uint64
	keyLength      int32
	data           webtools.SafeMap[string, T]
	path           string
	Logger         *webtools.ConsoleLogger
}

/*
NewRAMDatabase creates new RAM Database, set keyLength to negative to disable limit
*/
func NewRAMDatabase[T IDatabaseObject](keyLength int32, path string, emptyObject T) *RAMDatabase[T] {
	var inst = RAMDatabase[T]{}
	inst.oneValueLength = 0
	inst.keyLength = keyLength
	inst.data = webtools.MakeSafeMap[string, T]()
	inst.path = path
	inst.Logger = webtools.NewConsoleLoggerForTraffic("RAMDB", false)
	inst.emptyObject = emptyObject
	return &inst
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
	db.Logger.Log(1, "Saving database, please wait...")
	file, err := os.Create(db.path)
	if err != nil {
		db.Logger.Log(3, "Error saving database: "+err.Error())
		return err
	}
	defer file.Close()

	//Write length of one value
	binary.Write(file, binary.BigEndian, db.oneValueLength)

	//Write map values
	for _, v := range db.data.GetData() {
		result := bytes.NewBuffer(nil)
		limitedString := MakeLimitedStringDB(db.keyLength)
		limitedString.Set(v.Key)
		limitedString.ConvertToBytesDB(result)
		v.Value.ConvertToBytesDB(result)
		result.WriteTo(file)
	}
	db.Logger.Log(1, "Database saved.")
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
	err = binary.Read(file, binary.BigEndian, db.oneValueLength)
	if err != nil {
		db.Logger.Log(3, "Error loading database: "+err.Error())
		return err
	}

	//Read map values
	for {
		//Read key
		limitedString := MakeLimitedStringDB(db.keyLength)
		err := limitedString.ParseBytesDB(file)
		if err != nil {
			if err == io.EOF {
				break
			}
			db.Logger.Log(3, "Error loading database: "+err.Error())
			return err
		}

		//Read value
		value, err := db.emptyObject.ParseBytesDB(file)
		if err != nil {
			if err == io.EOF {
				break
			}
			db.Logger.Log(3, "Error loading database: "+err.Error())
			return err
		}

		//Set to map
		db.data.Set(limitedString.Get(), value.(T))
	}

	db.Logger.Log(1, "Database loaded.")
	return nil
}

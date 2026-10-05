package database

import (
	"bytes"
	"errors"
	"io"
	"os"
	"sync/atomic"
	"time"

	"github.com/kolojar/Go-Webtools/helpertools"
)

/*
 * journalEntry is entry of journal for JournalingDatabase
 */
type journalEntry[T any] struct {
	key      string
	isDelete bool
	value    T
}

/*
JournalingDatabase is database that is completly stored in RAM and loaded from disk on start (if lazyLoading is false). After inactivity is offloaded to disk. Data is saved only after amount of operations or on save. Otherwise everything is stored in Journal.
*/
type JournalingDatabase[T any] struct {
	//emptyObject T
	//oneValueLength uint64
	lazyLoading                    bool
	automaticUnload                time.Duration
	journalItemCountBeforeAutosave uint32
	journalLen                     atomic.Uint32
	data                           helpertools.SafeMap[string, T]
	path                           string
	Logger                         helpertools.ConsoleLogger
	convertToBytesDBFunc           func(writer io.Writer, data T) error
	parseDBFunc                    func(reader io.Reader) (T, error)
}

/*
NewJournalingRAMDatabase creates new Journaling RAM Database. Set journalItemCountBeforeAutosave to 0 for disabled autosave
*/
func NewJournalingRAMDatabase[T any](path string, journalItemCountBeforeAutosave uint32, convertToBytesDBFunc func(writer io.Writer, data T) error, parseDBFunc func(reader io.Reader) (T, error)) (*JournalingDatabase[T], error) {
	//Calculate one valueLength
	//emptyObjectBytes := bytes.NewBuffer(nil)
	//err := emptyObject.ConvertToBytesDB(emptyObjectBytes)
	//if err != nil {
	//	return nil, err
	//}

	if convertToBytesDBFunc == nil || parseDBFunc == nil {
		return nil, os.ErrInvalid
	}

	//Create object
	var inst = JournalingDatabase[T]{convertToBytesDBFunc: convertToBytesDBFunc, parseDBFunc: parseDBFunc, journalItemCountBeforeAutosave: journalItemCountBeforeAutosave}
	//inst.oneValueLength = uint64(emptyObjectBytes.Len())
	inst.data = helpertools.MakeSafeMap[string, T]()
	inst.path = path
	inst.Logger = helpertools.MakeConsoleLoggerForTraffic("JRAMDB", false)
	inst.journalLen.Store(0)
	return &inst, nil
}

/*
Get gets value from database
*/
func (db *JournalingDatabase[T]) Get(key string) T {
	return db.data.Get(key)
}

/*
GetData gets all data from database
*/
func (db *JournalingDatabase[T]) GetData() []helpertools.KeyValuePair[string, T] {
	return db.data.GetData()
}

/*
Set set value to database
*/
func (db *JournalingDatabase[T]) Set(key string, value T) {
	db.data.Set(key, value)
	db.appendToJournal(journalEntry[T]{isDelete: false, key: key, value: value})
}

/*
Delete deletes value from database
*/
func (db *JournalingDatabase[T]) Delete(key string) {
	db.data.Delete(key)
	db.appendToJournal(journalEntry[T]{isDelete: true, key: key})
}

/*
Len gets lenght of database
*/
func (db *JournalingDatabase[T]) Len() int {
	return db.data.Len()
}

/*
 * appendToJournal appends to journal and autosaves if needed
 */
func (db *JournalingDatabase[T]) appendToJournal(operation journalEntry[T]) {
	//Open journal
	journal, err := os.OpenFile(db.path+".journal", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		db.Logger.Log(helpertools.LogError, "Failed writing to journal: "+err.Error())
		return
	}
	defer journal.Close()

	//Convert key to bytes
	buffer := bytes.NewBuffer(nil)
	err = ConvertStringToBytesDB(buffer, operation.key)
	if err != nil {
		db.Logger.Log(helpertools.LogError, "Failed writing key to journal: "+err.Error())
		return
	}

	//Convert isDelete to bytes
	err = ConvertBoolToBytesDB(buffer, operation.isDelete)
	if err != nil {
		db.Logger.Log(helpertools.LogError, "Failed writing isDelete to journal: "+err.Error())
		return
	}

	//Convert value to bytes
	err = db.convertToBytesDBFunc(buffer, operation.value)
	if err != nil {
		db.Logger.Log(helpertools.LogError, "Failed writing value to journal: "+err.Error())
		return
	}

	//Append buffer data
	_, err = buffer.WriteTo(journal)
	if err != nil {
		db.Logger.Log(helpertools.LogError, "Failed writing data to journal: "+err.Error())
		return
	}
	count := db.journalLen.Add(1)

	//Check if ready for writing
	if db.journalItemCountBeforeAutosave == 0 || db.journalItemCountBeforeAutosave > count {
		return
	}
	db.Logger.Log(helpertools.LogWarning, "Saving DB due to full journal.")

	//Save DB
	err = db.Save()
	if err != nil {
		db.Logger.Log(helpertools.LogError, "Failed saving DB: "+err.Error())
		return
	}
}

/*
Save saves data of database to disk and clears journal
*/
func (db *JournalingDatabase[T]) Save() error {
	//Delete file if exists
	path := db.path + ".tmp"
	db.Logger.Log(helpertools.LogWarning, "Saving database, please wait...")
	os.Remove(path)

	//Create DB file
	file, err := os.Create(path)
	if err != nil {
		db.Logger.Log(helpertools.LogError, "Error saving database: "+err.Error())
		return err
	}
	defer file.Close()

	//Write length of one value
	//binary.Write(file, binary.BigEndian, db.oneValueLength)

	//Write map values
	for _, v := range db.data.GetData() {
		ConvertStringToBytesDB(file, v.Key)
		db.convertToBytesDBFunc(file, v.Value)
		file.Sync()
	}

	//Move database
	err = os.Rename(path, db.path)
	if err != nil {
		db.Logger.Log(helpertools.LogError, "Error renaming database: "+err.Error())
		return err
	}

	//Delete journal
	db.journalLen.Store(0)
	os.Remove(db.path + ".journal")

	//Finish
	db.Logger.Log(helpertools.LogWarning, "Database saved.")
	return nil
}

/*
Load loads data of database from disk and checks journal
*/
func (db *JournalingDatabase[T]) Load() error {
	//Open DB file
	db.Logger.Log(helpertools.LogWarning, "Loading database, please wait...")
	file, err := os.Open(db.path)
	if err != nil {
		db.Logger.Log(helpertools.LogError, "Error loading database: "+err.Error())
	} else {
		defer file.Close()

		//Read length of one value
		//err = binary.Read(file, binary.BigEndian, db.oneValueLength)
		//if err != nil {
		//	db.Logger.Log(helpertools.LogError, "Error loading database: "+err.Error())
		//	return err
		//}

		//Read map values
		//file.Read(make([]byte, 1))
		for {
			//Read key
			key, err := ParseStringDB(file)
			if err != nil {
				if err == io.EOF {
					break
				}
				db.Logger.Log(helpertools.LogError, "Error loading database key: "+err.Error())
				return err
			}

			//Read value
			value, err := db.parseDBFunc(file)
			if err != nil {
				if err == io.EOF {
					break
				}
				db.Logger.Log(helpertools.LogError, "Error loading database value: "+err.Error())
				return err
			}

			//Set to map
			db.data.Set(key, value)
		}

		//Log succeess
		db.Logger.Log(helpertools.LogWarning, "Database loaded.")
	}

	//Check journal
	journal, err := os.Open(db.path + ".journal")
	if err != nil && !os.IsNotExist(err) {
		db.Logger.Log(helpertools.LogError, "Error loading database journal: "+err.Error())
		return err
	}
	defer journal.Close()

	//Parse journal if possible
	if journal != nil {
		db.Logger.Log(helpertools.LogWarning, "Found journal! Database was not closed successfully last time.")
		for true {
			//Parse key of entry
			key, err := ParseStringDB(journal)
			if err != nil {
				if errors.Is(err, io.EOF) {
					//Break on valid error
					break
				}
				db.Logger.Log(helpertools.LogError, "Error loading database journal key: "+err.Error())
				return err
			}

			//Parse isDelete of entry
			isDelete, err := ParseBoolDB(journal)
			if err != nil {
				db.Logger.Log(helpertools.LogError, "Error loading database journal isDelete: "+err.Error())
				return err
			}

			//Parse value of entry
			value, err := db.parseDBFunc(journal)
			if err != nil {
				db.Logger.Log(helpertools.LogError, "Error loading database journal value: "+err.Error())
				return err
			}

			//Process value
			if isDelete {
				db.data.Delete(key)
			} else {
				db.data.Set(key, value)
			}
		}

		//Log succeess
		journal.Close()
		db.Logger.Log(helpertools.LogWarning, "Journal processed.")

		//Save DB
		err = db.Save()
		if err != nil {
			db.Logger.Log(helpertools.LogError, "Error saving database after journal process: "+err.Error())
			return err
		}
	}
	return nil
}

package webtools

import (
	"encoding/json"
	"os"
	"strings"
)

type ConfigManager struct {
	path   string
	values *SafeMap[string, configManagerValue]
}

type ConfigManagerValueType uint8
const AnyType ConfigManagerValueType = 0
const BoolType ConfigManagerValueType = 1
const IntType ConfigManagerValueType = 2
const FloatType ConfigManagerValueType = 3
const StringType ConfigManagerValueType = 4

type configManagerValue struct {
	option          ConfigManagerOption
	setInConfigFile bool
	setInArgument   bool
	value           any
}

/*
ConfigManagerOption is an option of ConfigManager
*/
type ConfigManagerOption struct {
	// DefaultValue is default value of option if not provided
	DefaultValue any
	// Description is description for argument help list or for JSON
	Description string
	// ConfigurableViaArgument specifies if argument can be configured by argument
	ConfigurableViaArgument bool
	// ConfigurableViaConfigFile specifies if argument can be configured by config file
	ConfigurableViaConfigFile bool
	// ValueType is type of value
	ValueType ConfigManagerValueType
}

/*
Load loads config manager values from JSON stored at configManager.path
*/
func (configManager *ConfigManager) Load() error {
	return configManager.LoadFrom(configManager.path)
}

/*
Load loads config manager values from JSON stored at path
*/
func (configManager *ConfigManager) LoadFrom(path string) error {
	//Read file
	valuesJsonC, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	valuesJson := JSONCToJSON(valuesJsonC)

	//Decode JSON
	readValues := make(map[string]any)
	err = json.Unmarshal(valuesJson, &readValues)
	if err != nil {
		return err
	}

	//Load arguments
	args := make(map[string]string)
	for i := 0; i < len(os.Args); i++ {
		if !strings.HasPrefix(os.Args[i], "--") {
			continue
		}
		if configManager.values.Has(os.Args[i][2:]) {
			if()
		}
	}

	//Parse valid options
	for _, k := range configManager.values.GetData() {
		if k.Value.option.ConfigurableViaConfigFile {
			v, ok := readValues[k.Key]
			if ok {
				val := configManager.values.Get(k.Key)
				val.setInConfigFile = true
				val.value = v
				configManager.values.Set(k.Key, val)
			}
		}
		//Parse arguments

	}

	return nil
}

/*
Save saves config manager values to JSON stored at configManager.path
*/
func (configManager *ConfigManager) Save() error {
	return configManager.SaveAs(configManager.path)
}

/*
SaveAs saves config manager values to JSON stored at path
*/
func (configManager *ConfigManager) SaveAs(path string) error {
	//Create file
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	//Write comment header
	_, err = file.WriteString("/*Config options:\nWarning: All edited comments will get removed after save.\n")
	if err != nil {
		return err
	}

	//Write comments
	writeValues := make(map[string]any)
	for _, v := range configManager.values.GetData() {
		if !v.Value.option.ConfigurableViaConfigFile {
			continue
		}
		if !v.Value.setInArgument && v.Value.setInConfigFile {
			writeValues[v.Key] = v.Value.value
		}
		defaultValue, err := json.Marshal(v.Value.option.DefaultValue)
		if err != nil {
			return err
		}
		file.WriteString("\"" + v.Key + "\": [" + string(defaultValue) + "] = " + v.Value.option.Description + "\n")
	}
	_, err = file.WriteString("*/\n")
	if err != nil {
		return err
	}

	//Encode JSON
	values, err := json.Marshal(writeValues)
	if err != nil {
		return err
	}
	_, err = file.Write(values)
	return err
}

/*
JSONCToJSON converts JSONC (commented JSON) to JSON. It destroyes all comments
*/
func JSONCToJSON(jsonc []byte) []byte {
	isInStarComment := false
	isInInlineCommment := false
	isInString := false
	result := make([]byte, 0)
	for i := range jsonc {
		// Get previous char
		var prev byte
		if i > 0 {
			prev = jsonc[i-1]
		}

		// Sort commnet cases
		if prev != '\\' && jsonc[i] == '"' && !isInInlineCommment && !isInStarComment {
			isInString = !isInString
		}
		if prev == '/' && jsonc[i] == '*' && !isInString && !isInInlineCommment {
			isInStarComment = true
		}
		if prev == '*' && jsonc[i] == '/' && !isInString && !isInInlineCommment {
			isInStarComment = false
		}
		if !isInStarComment && !isInString && prev == '/' && jsonc[i] == '/' {
			isInInlineCommment = true
		}
		if isInInlineCommment && jsonc[i] == '\n' {
			isInInlineCommment = false
		}

		// Write to buffer
		if jsonc[i] == '\n' {
			result = append(result, '\n')
		} else if isInInlineCommment || isInStarComment {
			result = append(result, ' ')
		} else {
			result = append(result, jsonc[i])
		}
	}
	return result
}

func (configManager *ConfigManager) AddOption(key string, option ConfigManagerOption) {
	configManager.values.Set(key, configManagerValue{
		option:          option,
		setInConfigFile: false,
		setInArgument:   false,
		value:           nil,
	})
}

func (configManager *ConfigManager) RemoveOption(key string) {
	configManager.values.Delete(key)
}

func (configManager *ConfigManager) GetValue(key string) (setInConfigFile bool, setInArgument bool, value any) {
	val := configManager.values.Get(key)
	return val.setInConfigFile, val.setInArgument, val.value
}

func (configManager *ConfigManager) SetValue(key string, value any) {
	val := configManager.values.Get(key)
	val.value = value
	val.setInConfigFile = true
	val.setInArgument = false
	configManager.values.Set(key, val)
}

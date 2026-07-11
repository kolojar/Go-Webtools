package helpertools

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	webtools "github.com/kolojar/Go-Webtools"
)

type ConfigManager struct {
	path   string
	values webtools.SafeMap[string, configManagerValue]
}

/*
NewConfigManager creates new instance of ConfigManager
*/
func NewConfigManager(path string) *ConfigManager {
	return &ConfigManager{
		path:   path,
		values: webtools.MakeSafeMap[string, configManagerValue](),
	}
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
func (configManager *ConfigManager) Load(argStart int) error {
	return configManager.LoadFrom(argStart, configManager.path)
}

/*
Load loads config manager values from JSON stored at path
*/
func (configManager *ConfigManager) LoadFrom(argStart int, path string) error {
	if argStart < 0 {
		argStart = 0
	}

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
	args := make(map[string]any)
	for i := argStart + 1; i < len(os.Args); i++ {
		if !strings.HasPrefix(os.Args[i], "--") {
			continue
		}
		optionName := os.Args[i][2:]
		option, hasOption := configManager.values.GetHas(optionName)
		hasValueAfter := len(os.Args) > i+1 && !strings.HasPrefix(os.Args[i+1], "--")
		if hasOption {
			//Check if configurable
			if !option.option.ConfigurableViaArgument {
				if hasValueAfter {
					i++
				}
				continue
			}

			//Write option to buffer
			if !hasValueAfter {
				if option.option.ValueType == BoolType {
					args[optionName] = true
				}
				i++
			} else {
				//TODO: DO PARSING LOGIC
				switch option.option.ValueType {
				case AnyType:
					args[optionName] = os.Args[i+1]
				case BoolType:
					value, err := strconv.ParseBool(os.Args[i+1])
					if err != nil {
						fmt.Println("Option " + optionName + " has invalid value: " + os.Args[i+1] + " - can be false/true")
						return err
					}
					args[optionName] = value
				case IntType:
					value, err := strconv.Atoi(os.Args[i+1])
					if err != nil {
						fmt.Println("Option " + optionName + " has invalid value: " + os.Args[i+1] + " - can be integer (whole number)")
						return err
					}
					args[optionName] = value
				case FloatType:
					value, err := strconv.ParseFloat(os.Args[i+1], 32)
					if err != nil {
						fmt.Println("Option " + optionName + " has invalid value: " + os.Args[i+1] + " - can be integer (whole number)")
						return err
					}
					args[optionName] = (float32)(value)
				case StringType:
					args[optionName] = os.Args[i+1]
				default:
					fmt.Println("Option " + optionName + " has invalid value type: " + strconv.FormatUint((uint64)(option.option.ValueType), 10))
					return os.ErrInvalid
				}
				i += 2
			}
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
		if k.Value.option.ConfigurableViaArgument {
			v, ok := args[k.Key]
			if ok {
				val := configManager.values.Get(k.Key)
				val.setInArgument = true
				val.value = v
				configManager.values.Set(k.Key, val)
			}
		}
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
AddOption adds option to configManager
*/
func (configManager *ConfigManager) AddOption(key string, option ConfigManagerOption) {
	configManager.values.Set(key, configManagerValue{
		option:          option,
		setInConfigFile: false,
		setInArgument:   false,
		value:           nil,
	})
}

/*
RemoveOption removes option
*/
func (configManager *ConfigManager) RemoveOption(key string) {
	configManager.values.Delete(key)
}

/*
GetPureValue gets value
*/
func (configManager *ConfigManager) GetPureValue(key string) any {
	val := configManager.values.Get(key)
	return val.value
}

/*
GetValue gets value and its settings
*/
func (configManager *ConfigManager) GetValue(key string) (setInConfigFile bool, setInArgument bool, value any) {
	val := configManager.values.Get(key)
	return val.setInConfigFile, val.setInArgument, val.value
}

/*
SetValue sets value in configFile (will be written to file if saved)
*/
func (configManager *ConfigManager) SetValue(key string, value any) {
	val := configManager.values.Get(key)
	val.value = value
	val.setInConfigFile = true
	val.setInArgument = false
	configManager.values.Set(key, val)
}

/*
Copy creates copy of configManager
*/
func (configManager *ConfigManager) Copy(newPath string) *ConfigManager {
	result := NewConfigManager(newPath)
	for _, v := range configManager.values.GetData() {
		result.values.Set(v.Key, v.Value)
	}
	return result
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

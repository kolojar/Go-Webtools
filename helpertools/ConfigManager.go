package helpertools

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var ErrConfigUsedHelpArgument = errors.New("used help argument")
var ErrConfigMissingArgumentParameter = errors.New("missing argument parameter")

type ConfigManager struct {
	path                        string
	values                      SafeMap[string, configManagerValue]
	helpRequiredValues          []string
	allowSaveArgsToFileArgument bool
}

// NewConfigManager creates new instance of ConfigManager
func NewConfigManager(path string, allowSaveArgsToFileArgument bool, helpRequiredValues ...string) *ConfigManager {
	return &ConfigManager{
		path:                        path,
		values:                      MakeSafeMap[string, configManagerValue](),
		helpRequiredValues:          helpRequiredValues,
		allowSaveArgsToFileArgument: allowSaveArgsToFileArgument,
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
	valueInArgument any
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
	saveArgsAfterLoad := false
	args := make(map[string]any)
	for i := argStart + 1; i < len(os.Args); i++ {
		if !strings.HasPrefix(os.Args[i], "--") {
			continue
		}
		optionName := os.Args[i][2:]
		option, hasOption := configManager.values.GetHas(optionName)
		hasValueAfter := len(os.Args) > i+1 && (!strings.HasPrefix(os.Args[i+1], "--") || option.option.ValueType == StringType || option.option.ValueType == AnyType)
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
					continue
				}

				//Missing argument
				fmt.Println("Argument --" + optionName + " is missing parameter, see --help for more information.")
				return ErrConfigMissingArgumentParameter

				//i++
			} else {
				//Parse
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
				i++
			}
		} else {
			//Invalid user argument
			if optionName == "saveArgsToConfig" {
				saveArgsAfterLoad = true
				continue
			}

			//Show error message
			if optionName != "help" {
				fmt.Println("Error: --" + optionName + " is invalid argument.")
			}

			//Print usage
			fmt.Print("Usage: " + os.Args[0])
			for _, v := range configManager.helpRequiredValues {
				fmt.Print(" <" + v + ">")
			}
			fmt.Println(" [arguments]")
			fmt.Println("Arguments:")

			//Print static arguments
			fmt.Println(" --help Displays this help")
			if configManager.allowSaveArgsToFileArgument {
				fmt.Println(" --saveArgsToConfig Saves all possible arguments to config file")
			}

			//Print arguments
			configManager.values.Range(func(key string, value configManagerValue) (doBreak bool) {
				//Check if valid argument
				if !value.option.ConfigurableViaArgument {
					return
				}

				//Start line
				fmt.Print(" --" + key + " <")

				//Write type
				switch value.option.ValueType {
				case AnyType:
					{
						fmt.Print("any")
					}
				case BoolType:
					{
						fmt.Print("bool (true/false, if only argument is present, it is parsed as true)")
					}
				case IntType:
					{
						fmt.Print("int")
					}
				case FloatType:
					{
						fmt.Print("float32")
					}
				case StringType:
					{
						fmt.Print("string")
					}
				}

				//Write description
				fmt.Println("> " + value.option.Description)
				return false
			})
			return ErrConfigUsedHelpArgument
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
				val.valueInArgument = v
				configManager.values.Set(k.Key, val)
			}
		}
	}

	//Save if needed
	if saveArgsAfterLoad {
		for _, v := range configManager.values.GetData() {
			//Check if can copy
			if !v.Value.option.ConfigurableViaArgument || !v.Value.option.ConfigurableViaConfigFile || !v.Value.setInArgument {
				continue
			}

			//Copy
			v.Value.value = v.Value.valueInArgument
			v.Value.setInConfigFile = true
			configManager.values.Set(v.Key, v.Value)
		}

		//Save
		return configManager.Save()
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
		if v.Value.setInConfigFile {
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
	_, _, val := configManager.GetValue(key)
	return val
}

/*
GetValue gets value and its settings
*/
func (configManager *ConfigManager) GetValue(key string) (setInConfigFile bool, setInArgument bool, value any) {
	val := configManager.values.Get(key)
	return val.setInConfigFile, val.setInArgument, FormatByBool(val.setInArgument, val.valueInArgument, FormatByBool(val.setInArgument, val.value, val.option.DefaultValue))
}

/*
SetValue sets value in configFile (will be written to file if saved and overwrites argument setting)
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
	result := NewConfigManager(newPath, configManager.allowSaveArgsToFileArgument)
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
	skip := 0
	for i := range jsonc {
		//Skip
		if skip > 0 {
			skip--
			continue
		}

		// Get previous char
		var next byte
		if i < len(jsonc)-1 {
			next = jsonc[i+1]
		}

		// Sort commnet cases
		if jsonc[i] != '\\' && next == '"' && !isInInlineCommment && !isInStarComment {
			isInString = !isInString
		}
		if jsonc[i] == '/' && next == '*' && !isInString && !isInInlineCommment {
			isInStarComment = true
			skip++
		}
		if jsonc[i] == '*' && next == '/' && !isInString && !isInInlineCommment {
			isInStarComment = false
			skip++
			continue
		}
		if !isInStarComment && !isInString && jsonc[i] == '/' && next == '/' {
			isInInlineCommment = true
			skip++
		}
		skipNewLine := isInInlineCommment && jsonc[i] == '\n'
		if skipNewLine {
			isInInlineCommment = false
			skip++
		}
		skipNewLine = skipNewLine || (jsonc[i] == '\n' && isInStarComment)

		// Write to buffer
		if jsonc[i] == '\n' && !skipNewLine {
			result = append(result, '\n')
		} else if isInInlineCommment || isInStarComment {
			//result = append(result, ' ')
			continue
		} else {
			result = append(result, jsonc[i])
		}
	}
	return result
}

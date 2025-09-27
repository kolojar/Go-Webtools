package webtools

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	ANSII_TOTAL_RESET_SEQUENCE            = "\033[0m"
	ANSII_TEXT_COLOR_RESET_SEQUENCE       = "\033[39m"
	ANSII_BACKGROUND_COLOR_RESET_SEQUENCE = "\033[49m"
	ANSII_STRIKED_OUT_SEQUENCE            = "\033[9m"
	ANSII_SET_TEXT_COLOR_SEQUENCE         = "\033[38;5;" //Add m at the end
	ANSII_SET_BACKGROUND_COLOR_SEQUENCE   = "\033[48;5;" //Add m at the end
)

/*
Uint8 = type -> 0 = Nonspecific log; 1 = Information; 2 = Warning; 3 = Error;
String = message;
String = formated message
String = source identifier
*/
type LogReportFunc func(uint8, string, string, string)

type ConsoleLogger struct {
	LogReportFunction LogReportFunc
	//saveToFile        bool
	Prefix        string
	minPrintLevel uint8
}

/*
Creates new logger class. Report error level: 0 = Nonspecific log; 1 = Information; 2 = Warning; 3 = Error;
*/
func NewConsoleLogger(Prefix string, minPrintLevel uint8) *ConsoleLogger {
	return &ConsoleLogger{Prefix: Prefix, LogReportFunction: nil, minPrintLevel: minPrintLevel}
}

/*
logType -> 0 = Nonspecific log; 1 = Information; 2 = Warning; 3 = Error
*/
func (logger *ConsoleLogger) Log(logType uint8, message string) {
	logger.LogWithSourceId(logType, message, "")
}

/*
logType -> 0 = Nonspecific log; 1 = Information; 2 = Warning; 3 = Error
*/
func (logger *ConsoleLogger) LogWithSourceId(logType uint8, message string, sourceId string) {
	colorlogTypePrefix := ""
	logTypePrefix := ""
	switch logType {
	case 1:
		logTypePrefix = "INFO"
		colorlogTypePrefix = ANSII_SET_TEXT_COLOR_SEQUENCE + "27m"
	case 2:
		logTypePrefix = "WARN"
		colorlogTypePrefix = ANSII_SET_TEXT_COLOR_SEQUENCE + "214m"
	case 3:
		logTypePrefix = "ERROR"
		colorlogTypePrefix = ANSII_SET_TEXT_COLOR_SEQUENCE + "15m" + ANSII_SET_BACKGROUND_COLOR_SEQUENCE + "9m"
	default:
		logTypePrefix = "GENERAL"
		colorlogTypePrefix = ANSII_SET_TEXT_COLOR_SEQUENCE + "34m"
	}
	logMsg := "[" + time.Now().Format("02/01/2006 15:04:05.000") + " - " + logTypePrefix + " - " + logger.Prefix + "]: " + message
	if logType >= logger.minPrintLevel {
		fmt.Println(colorlogTypePrefix + logMsg + ANSII_TOTAL_RESET_SEQUENCE)
	}
	if logger.LogReportFunction != nil {
		logger.LogReportFunction(logType, message, logMsg, sourceId)
	}
}

func FormatByBool[T any](b bool, trueVal T, falseVal T) T {
	if b {
		return trueVal
	} else {
		return falseVal
	}
}

func MapToString[K comparable, V any](m map[K]V) string {
	result := "{"
	for k, v := range m {
		result += "[" + fmt.Sprint(k) + ", " + fmt.Sprint(v) + "], "
	}
	result = strings.TrimSuffix(result, ", ")
	result += "}"
	return result
}

/*
Creates new ConsoleLogger with option to disable traffic report. Traffic reports are reports with 0 level
*/
func NewConsoleLoggerForTraffic(prefix string, reportTraffic bool) *ConsoleLogger {
	return NewConsoleLogger(prefix, FormatByBool[uint8](reportTraffic, 0, 1))
}

/*
Reads line from console
*/
func ReadLineFromConsole(message string) ([]byte, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(message)
	return reader.ReadBytes(byte('\n'))
}

/*
Reads choice from console
*/
func ReadChoiceFromConsole[T comparable](message string, choices map[string]T, defaultChoice string) (T, error) {
	//Print choices
	i := 0
	for k, _ := range choices {
		i++
		fmt.Println(strconv.Itoa(i) + ": " + k)
	}
	fmt.Println(strings.Repeat("=", 20))

	//Set user select
	sel, err := ReadLineFromConsole(message)
	if err != nil {
		return choices[""], err
	}

	//Sort choices
	selString := string(sel)
	selectOption, err := strconv.Atoi(strings.Replace(strings.Replace(selString, ".", "", 1), ":", "", 1))
	if err != nil {
		if selectOption <= i {
			j := 0
			for _, v := range choices {
				j++
				if j == selectOption {
					return v, nil
				}
			}
		}
	}
	j := 0
	for k, v := range choices {
		if k == selString {
			return v, nil
		}
		j++
		if k == strconv.Itoa(j)+": "+selString {
			return v, nil
		}
	}

	//Invalid choice
	if defaultChoice != "" {
		fmt.Println("Selected default option: " + defaultChoice)
		return choices[defaultChoice], nil
	}
	fmt.Println("Invalid choice!")
	return choices[""], os.ErrInvalid
}

/*
Reads choice from console until it is not correct
*/
func ReadChoiceFromConsoleValid[T comparable](message string, choices map[string]T, defaultChoice string) (T, error) {
	val, err := ReadChoiceFromConsole(message, choices, defaultChoice)
	if err == os.ErrInvalid {
		fmt.Println()
		fmt.Println()
		return ReadChoiceFromConsole(message, choices, defaultChoice)
	}
	return val, err
}

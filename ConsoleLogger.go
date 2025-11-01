/*
Package webtools provides generic tools for working with subpackages. The main thing of this package are the subpackages
*/
package webtools

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

/*
ANSIITotalResetSequence is total reset sequence for all text formating
*/
const ANSIITotalResetSequence = "\033[0m"

/*
ANSIISetTextColorSequence issequence for setting text color, do not forget to add m at the end
*/
const ANSIISetTextColorSequence = "\033[38;5;"

/*
ANSIISetBackgroundColorSequence issequence for setting background color, do not forget to add m at the end
*/
const ANSIISetBackgroundColorSequence = "\033[48;5;"

/*
LogReportFunc used for event reporting of Logger: 0 = Nonspecific log; 1 = Information; 2 = Warning; 3 = Error;
*/
type LogReportFunc func(eventType uint8, message string, formatedMessage string, sourceId string)

/*
ConsoleLogger is simple logger
*/
type ConsoleLogger struct {
	LogReportFunction LogReportFunc
	//saveToFile        bool
	Prefix        string
	minPrintLevel uint8
}

/*
NewConsoleLogger creates new logger class. Report error level: 0 = Nonspecific log; 1 = Information; 2 = Warning; 3 = Error;
*/
func NewConsoleLogger(Prefix string, minPrintLevel uint8) *ConsoleLogger {
	return &ConsoleLogger{Prefix: Prefix, LogReportFunction: nil, minPrintLevel: minPrintLevel}
}

/*
Log logs message, logType -> 0 = Nonspecific log; 1 = Information; 2 = Warning; 3 = Error
*/
func (logger *ConsoleLogger) Log(logType uint8, message string) {
	logger.LogWithSourceID(logType, message, "")
}

/*
LogWithSourceID logs message, logType -> 0 = Nonspecific log; 1 = Information; 2 = Warning; 3 = Error
*/
func (logger *ConsoleLogger) LogWithSourceID(logType uint8, message string, sourceID string) {
	colorlogTypePrefix := ""
	logTypePrefix := ""
	switch logType {
	case 1:
		logTypePrefix = "INFO"
		colorlogTypePrefix = ANSIISetTextColorSequence + "27m"
	case 2:
		logTypePrefix = "WARN"
		colorlogTypePrefix = ANSIISetTextColorSequence + "214m"
	case 3:
		logTypePrefix = "ERROR"
		colorlogTypePrefix = ANSIISetTextColorSequence + "15m" + ANSIISetBackgroundColorSequence + "9m"
	default:
		logTypePrefix = "GENERAL"
		colorlogTypePrefix = ANSIISetTextColorSequence + "34m"
	}
	logMsg := "[" + time.Now().Format("02/01/2006 15:04:05.000") + " - " + logTypePrefix + " - " + logger.Prefix + "]: " + message
	if logType >= logger.minPrintLevel {
		fmt.Println(colorlogTypePrefix + logMsg + ANSIITotalResetSequence)
	}
	if logger.LogReportFunction != nil {
		logger.LogReportFunction(logType, message, logMsg, sourceID)
	}
}

/*
FormatByBool returns value by bool
*/
func FormatByBool[T any](b bool, trueVal T, falseVal T) T {
	if b {
		return trueVal
	}
	return falseVal
}

/*
MapToString converts map to string
*/
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
NewConsoleLoggerForTraffic creates new ConsoleLogger with option to disable traffic report. Traffic reports are reports with 0 level
*/
func NewConsoleLoggerForTraffic(prefix string, reportTraffic bool) *ConsoleLogger {
	return NewConsoleLogger(prefix, FormatByBool[uint8](reportTraffic, 0, 1))
}

/*
ReadLineFromConsole reads line from console
*/
func ReadLineFromConsole(message string) ([]byte, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(message)
	return reader.ReadBytes(byte('\n'))
}

package helpertools

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// LogType is type of log
type LogType uint8

// LogNonspecific is nonspecific log (green)
const LogNonspecific LogType = 0

// LogTraffic is log for traffic (green)
const LogTraffic LogType = 1

// LogInfo is log for informations (blue)
const LogInfo LogType = 2

// LogWarning is log for warnings (yellow)
const LogWarning LogType = 3

// LogError is log for errors (red)
const LogError LogType = 4

// ANSIITotalResetSequence is total reset sequence for all text formating
const ANSIITotalResetSequence = "\033[0m"

// ANSIISetTextColorSequence issequence for setting text color, do not forget to add m at the end
const ANSIISetTextColorSequence = "\033[38;5;"

// ANSIISetBackgroundColorSequence issequence for setting background color, do not forget to add m at the end
const ANSIISetBackgroundColorSequence = "\033[48;5;"

// LogReportFunc used for event reporting of Logger: 0 = Nonspecific log; 1 = Information; 2 = Warning; 3 = Error;
type LogReportFunc func(logType LogType, message string, formatedMessage string, sourceId string)

// ConsoleLogger is simple logger
type ConsoleLogger struct {
	LogReportFunction LogReportFunc
	// saveToFile        bool
	Prefix      string
	Preprefix   string
	IgnoredLogs map[LogType]struct{}
}

// MakeConsoleLogger creates new logger class.
//
// Set LogToConsole to false to disable logging to Console.
//
// To ignore specific LogType use IgnoredLogs
func MakeConsoleLogger(prefix string) ConsoleLogger {
	return ConsoleLogger{Prefix: prefix, LogReportFunction: nil, IgnoredLogs: make(map[LogType]struct{})}
}

// Log logs message
func (logger *ConsoleLogger) Log(logType LogType, message string) {
	logger.LogWithSourceID(logType, message, "")
}

// LogWithSourceID logs message, logType -> 0 = Nonspecific log; 1 = Information; 2 = Warning; 3 = Error
func (logger *ConsoleLogger) LogWithSourceID(logType LogType, message string, sourceID string) {
	colorlogTypePrefix := ""
	logTypePrefix := ""
	switch logType {
	case LogInfo:
		logTypePrefix = "INFO"
		colorlogTypePrefix = ANSIISetTextColorSequence + "27m"
	case LogWarning:
		logTypePrefix = "WARN"
		colorlogTypePrefix = ANSIISetTextColorSequence + "214m"
	case LogError:
		logTypePrefix = "ERROR"
		colorlogTypePrefix = ANSIISetTextColorSequence + "15m" + ANSIISetBackgroundColorSequence + "9m"
	case LogTraffic:
		logTypePrefix = "TRAFFIC"
		colorlogTypePrefix = ANSIISetTextColorSequence + "34m"
	default:
		logTypePrefix = "GENERIC"
		colorlogTypePrefix = ANSIISetTextColorSequence + "34m"
	}
	logMsg := "[" + time.Now().Format("02/01/2006 15:04:05.000") + " - " + logTypePrefix + " - " + FormatByBool(logger.Preprefix != "", logger.Preprefix+" - ", "") + logger.Prefix + "]: " + message
	_, ignore := logger.IgnoredLogs[logType]
	if !ignore {
		fmt.Println(colorlogTypePrefix + logMsg + ANSIITotalResetSequence)
	}
	if logger.LogReportFunction != nil {
		logger.LogReportFunction(logType, message, logMsg, sourceID)
	}
}

// FormatByBool returns value by bool
func FormatByBool[T any](b bool, trueVal T, falseVal T) T {
	if b {
		return trueVal
	}
	return falseVal
}

// MapToString converts map to string
func MapToString[K comparable, V any](m map[K]V) string {
	result := "{"
	for k, v := range m {
		result += "[" + fmt.Sprint(k) + ", " + fmt.Sprint(v) + "], "
	}
	result = strings.TrimSuffix(result, ", ")
	result += "}"
	return result
}

// MakeConsoleLoggerForTraffic creates new ConsoleLogger with option to disable traffic report.
func MakeConsoleLoggerForTraffic(prefix string, reportTraffic bool) ConsoleLogger {
	logger := MakeConsoleLogger(prefix)
	if !reportTraffic {
		logger.IgnoredLogs[LogTraffic] = struct{}{}
	}
	return logger
}

// ReadLineFromConsole reads line from console
func ReadLineFromConsole(message string) ([]byte, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(message)
	return reader.ReadBytes(byte('\n'))
}

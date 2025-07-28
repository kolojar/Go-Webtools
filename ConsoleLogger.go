package webtools

import (
	"fmt"
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
	Prefix string
}

// Creates new logger class
func MakeConsoleLogger(Prefix string) ConsoleLogger {
	return ConsoleLogger{Prefix: Prefix, LogReportFunction: nil}
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
	fmt.Println(colorlogTypePrefix + logMsg + ANSII_TOTAL_RESET_SEQUENCE)
	if logger.LogReportFunction != nil {
		logger.LogReportFunction(logType, message, logMsg, sourceId)
	}
}

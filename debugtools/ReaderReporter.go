package debugtools

import (
	"fmt"
	"io"
	"os"
)

// ReaderReporter is debug tool for writing how many bytes have been read from reader and how far reader is
type ReaderReporter struct {
	reader                  io.Reader
	distance                int
	reportDistanceToConsole bool
}

// NewReaderReporter wraps io.Reader to ReaderReporter
func NewReaderReporter(reader io.Reader, reportDistanceToConsole bool) *ReaderReporter {
	return &ReaderReporter{reader: reader, reportDistanceToConsole: reportDistanceToConsole, distance: 0}
}

// Read reads data from reader and increases distance by n and reports it to console
func (reader *ReaderReporter) Read(p []byte) (n int, err error) {
	//Check if reader is valid
	if reader.reader == nil {
		return 0, os.ErrInvalid
	}

	//Read data
	n, err = reader.reader.Read(p)
	if err != nil {
		return n, err
	}

	//Move distance
	reader.distance += n
	if reader.reportDistanceToConsole {
		fmt.Println("Read bytes:", n, "Current position in file:", reader.distance)
	}
	return n, nil
}

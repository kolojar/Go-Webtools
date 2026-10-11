package debugtools

import (
	"fmt"
	"io"
	"os"
)

// ReadSeekerReporter is debug tool for writing how many bytes have been read from reader and how far reader is
type ReadSeekerReporter struct {
	reader                  io.Reader
	distance                int
	reportDistanceToConsole bool
}

// NewReadSeekerReporter wraps io.ReadSeeker to ReadSeekerReporter
func NewReadSeekerReporter(reader io.ReadSeeker, reportDistanceToConsole bool) *ReadSeekerReporter {
	return &ReadSeekerReporter{reader: reader, reportDistanceToConsole: reportDistanceToConsole, distance: 0}
}

// Read reads data from reader and increases distance by n and reports it to console
func (reader *ReadSeekerReporter) Read(p []byte) (n int, err error) {
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

// Seek seeks from reader
func (reader *ReadSeekerReporter) Seek(offset int64, whence int) (int64, error) {
	//Check if reader is valid
	if reader.reader == nil {
		return 0, os.ErrInvalid
	}

	//Seek
	return reader.Seek(offset, whence)
}

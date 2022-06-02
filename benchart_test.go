package main

import (
	"bufio"
	"errors"
	"fmt"
	"hash/crc64"
	"io"
	"os"
	"testing"
)

var filesToClean []string

func TestMain(m *testing.M) {
	exitVal := m.Run()

	for _, filePath := range filesToClean {
		errFileDeletion := os.Remove(filePath)
		if errFileDeletion != nil {
			fmt.Printf("Can't delete test file: %v: %v\n", filePath, errFileDeletion)
		}
	}

	os.Exit(exitVal)
}

func Test_RunCommand(t *testing.T) {
	errCommand := RunCommand([]string{
		"benchart",
		"Hash;xAxisName=bytes size;title=Benchmark of hash functions",
		"PoolOverhead;xAxisType=log;yAxisType=log",
		"RateLimiter;xAxisType=log;xAxisName=goroutines",
		"testdata/input.csv", "result.html",
	})
	if errCommand != nil {
		t.Fatal(errCommand)
	}

	resultChecksum := checksumFromFile(t, "result.html")
	expectedChecksum := checksumFromFile(t, "testdata/etalon.html")

	if resultChecksum != expectedChecksum {
		t.Fatal("testdata/etalon.html is different from result.html")
	}
}

func checksumFromFile(t *testing.T, filepath string) uint64 {
	t.Helper()

	file, errOpenResultFile := os.Open(filepath)
	if errOpenResultFile != nil {
		t.Fatal(errOpenResultFile)
	}

	defer func(resultFile *os.File) {
		errClose := resultFile.Close()
		if errClose != nil {
			t.Fatal(errClose)
		}
	}(file)

	hashCrc64 := crc64.New(crc64.MakeTable(crc64.ISO))

	_, errCopyResultFile := io.Copy(hashCrc64, file)
	if errCopyResultFile != nil {
		t.Fatal(errCopyResultFile)
	}

	return hashCrc64.Sum64()
}

func Test_RunCommand_Negative(t *testing.T) {
	type args struct {
		arguments []string
	}

	input := func(input ...string) args {
		return args{append([]string{"benchart"}, input...)}
	}

	tests := []struct {
		name          string
		args          args
		expectedError error
	}{
		{
			"ErrCantOpenInputFile",
			input("nonExistentInputFile", "someOutput"),
			ErrCantOpenInputFile,
		},
		{
			"ErrCantOpenOrCreateOutputFile",
			input("testdata/input.csv", "/nonExistentOutputPathFile/nonExistentOutputFile.html"),
			ErrCantOpenOrCreateOutputFile,
		},
		{
			"ErrNotEnoughInputArguments",
			input("notEnoughInputArguments"),
			ErrNotEnoughInputArguments,
		},
		{
			"ErrCantParseChartOptions",
			input("", "testdata/input.csv", "etalon.html"),
			ErrCantParseChartOptions,
		},
		{
			"ErrCantParseOption",
			input(";", "testdata/input.csv", "etalon.html"),
			ErrCantParseOption,
		},
		{
			"ErrOptionIsNotSupported",
			input(";someOption=", "testdata/input.csv", "etalon.html"),
			ErrOptionIsNotSupported,
		},
		{
			"ErrOptionTypeIsWrong",
			input(";xAxisType=", "testdata/input.csv", "etalon.html"),
			ErrOptionTypeIsWrong,
		},
		{
			"ErrOptionTypeIsWrong",
			input(";xAxisType=exp", "testdata/input.csv", "etalon.html"),
			ErrOptionTypeIsWrong,
		},
		{
			"ErrNotEnoughColumns",
			input(
				";xAxisType=log",
				caseFile(t, `
						name,time/op (ns/op)±
						Hash/type:crc32;bytes:4-8,4.13067E+00,1%
				`),
				"etalon.html",
			),
			ErrNotEnoughColumns,
		},
		{
			"ErrNotEnoughColumns",
			input(
				";xAxisType=log",
				caseFile(t, `
						name,time/op (ns/op),±
						Hash/type:crc32;bytes:4-8,4.13067E+00|1%
				`),
				"etalon.html",
			),
			ErrNotEnoughColumns,
		},
		{
			"ErrCantParseBenchmarkName",
			input(
				";xAxisType=log",
				caseFile(t, `
						name,time/op (ns/op),±
						Hash\type:crc32;bytes:4-8,4.13067E+00,1%
				`),
				"etalon.html",
			),
			ErrCantParseBenchmarkName,
		},
		{
			"ErrCantParseMeasurementAttributes",
			input(
				";xAxisType=log",
				caseFile(t, `
						name,time/op (ns/op),±
						Hash/type:crc32;bytes:4|8,4.13067E+00,1%
				`),
				"etalon.html",
			),
			ErrCantParseMeasurementAttributes,
		},
		{
			"ErrMeasurementLineHasNoTypeAttribute",
			input(
				";xAxisType=log",
				caseFile(t, `
						name,time/op (ns/op),±
						Hash/types:crc32;bytes:4-8,4.13067E+00,1%
				`),
				"etalon.html",
			),
			ErrMeasurementLineHasNoTypeAttribute,
		},
		{
			"ErrCantParseYValue",
			input(
				";xAxisType=log",
				caseFile(t, `
						name,time/op (ns/op),±
						Hash/type:crc32;bytes:4-8,4.GG13067E+00,1%
				`),
				"etalon.html",
			),
			ErrCantParseYValue,
		},
		{
			"ErrCantParseErrorRate",
			input(
				";xAxisType=log",
				caseFile(t, `
						name,time/op (ns/op),±
						Hash/type:crc32;bytes:4-8,4.13067E+00,1
				`),
				"etalon.html",
			),
			ErrCantParseErrorRate,
		},
		{
			"ErrCantParseErrorRate",
			input(
				";xAxisType=log",
				caseFile(t, `
						name,time/op (ns/op),±
						Hash/type:crc32;bytes:4-8,4.13067E+00,1GG%
				`),
				"etalon.html",
			),
			ErrCantParseErrorRate,
		},
		{
			"ErrOptionChartNameNotFound",
			input(
				"HashesBench;xAxisType=log",
				caseFile(t, `
						name,time/op (ns/op),±
						Hash/type:crc32;bytes:4-8,4.13067E+00,1%
				`),
				"etalon.html",
			),
			ErrOptionChartNameNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errCommand := RunCommand(tt.args.arguments)
			if tt.expectedError != nil && !errors.Is(errCommand, tt.expectedError) {
				t.Fatalf("\nExp: `%T(%+v)`\nAct: `%T(%+v)`\n", tt.expectedError, tt.expectedError, errCommand, errCommand)
			}
		})
	}
}

func caseFile(t *testing.T, body string) string {
	t.Helper()

	file, errFileCreation := os.CreateTemp("", "input-*.csv")
	if errFileCreation != nil {
		t.Fatalf(errFileCreation.Error())
	}

	filesToClean = append(filesToClean, file.Name())

	writer := bufio.NewWriter(file)
	defer func(writer *bufio.Writer) {
		errFlushWrite := writer.Flush()
		if errFlushWrite != nil {
			t.Fatalf(errFlushWrite.Error())
		}
	}(writer)

	_, errWriteFile := writer.Write([]byte(body))
	if errWriteFile != nil {
		t.Fatalf(errWriteFile.Error())
	}

	return file.Name()
}

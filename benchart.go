package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

//go:embed "template.html"
var template []byte

type BenchartError string

func (b BenchartError) Error() string {
	return string(b)
}

const (
	ErrCantCloseOutputFile               BenchartError = "can't close output file"
	ErrCantFlushOutputFile               BenchartError = "can't flush output file"
	ErrCantMarshalChartBenchmarkResults  BenchartError = "can't marshal chart benchmarkResults"
	ErrCantOpenInputFile                 BenchartError = "can't open input file"
	ErrCantOpenOrCreateOutputFile        BenchartError = "can't open or create output file"
	ErrCantParseBenchmarkName            BenchartError = "can't parse benchmark name"
	ErrCantParseChartOptions             BenchartError = "can't parse chart options"
	ErrCantParseErrorRate                BenchartError = "can't parse error rate"
	ErrCantParseMeasurementAttributes    BenchartError = "can't parse measurement attributes"
	ErrCantParseOption                   BenchartError = "can't parse option"
	ErrCantParseYValue                   BenchartError = "can't parse y value"
	ErrCantWriteOutputFile               BenchartError = "can't write output file"
	ErrMeasurementLineHasNoTypeAttribute BenchartError = "measurement line has no 'type' attribute"
	ErrNotEnoughColumns                  BenchartError = "not enough columns"
	ErrNotEnoughInputArguments           BenchartError = "not enough input arguments"
	ErrOptionChartNameNotFound           BenchartError = "option chart name not found"
	ErrOptionIsNotSupported              BenchartError = "option is not supported"
	ErrOptionTypeIsWrong                 BenchartError = "option type is wrong"
)

type (
	ChartName          string
	ChartOption        string
	ChartOptionType    string
	ChartOptionTypeSet map[ChartOptionType]struct{}
)

type Point struct {
	X     string
	Y     string
	Error string
}

type BenchmarkResults struct {
	ID         string
	Name       ChartName
	YAxisLabel string
	Cases      map[string][]Point
	Options    map[ChartOption]string
}

func main() {
	errOutput := RunCommand(os.Args)
	if errOutput != nil {
		fmt.Println("Command finished with error: " + errOutput.Error())
		os.Exit(-1)
	}
}

//nolint:nonamedreturns // here named return required to propagate error from defer
func RunCommand(arguments []string) (errResponse error) {
	inputFilePath, outputFilePath, options, errOptionsParsing := parseOptions(arguments)
	if errOptionsParsing != nil {
		return errOptionsParsing
	}

	inputFile, errFileOpen := os.Open(inputFilePath)
	if errFileOpen != nil {
		return fmt.Errorf("%w: %v: %v", ErrCantOpenInputFile, inputFilePath, errFileOpen)
	}

	benchmarkResults, errParsingResults := parseBenchmarkResultsFromInputFile(inputFile, options)
	if errParsingResults != nil {
		return errParsingResults
	}

	f, errOutputFileCreation := os.Create(outputFilePath)
	if errOutputFileCreation != nil {
		return fmt.Errorf("%w: %v: %v", ErrCantOpenOrCreateOutputFile, outputFilePath, errOutputFileCreation)
	}

	defer func(f *os.File) {
		errCloseOutputFile := f.Close()
		if errCloseOutputFile != nil {
			errResponse = fmt.Errorf("%w: %v: %v", ErrCantCloseOutputFile, outputFilePath, errCloseOutputFile)
			return
		}
	}(f)

	benchmarkResultsJSON, errMarshalData := json.MarshalIndent(&benchmarkResults, "", "  ")
	if errMarshalData != nil {
		return fmt.Errorf("%w: %v", ErrCantMarshalChartBenchmarkResults, errMarshalData)
	}

	outputWriter := bufio.NewWriter(f)
	defer func(outputWriter *bufio.Writer) {
		errFileFlush := outputWriter.Flush()
		if errFileFlush != nil {
			errResponse = fmt.Errorf("%w: %v", ErrCantFlushOutputFile, errFileFlush)
			return
		}
	}(outputWriter)

	_, errWriteFile := outputWriter.Write(bytes.Replace(template, []byte("goCLIInput"), benchmarkResultsJSON, 1))
	if errWriteFile != nil {
		return fmt.Errorf("%w: %v", ErrCantWriteOutputFile, errWriteFile)
	}

	return nil
}

func parseBenchmarkResultsFromInputFile(
	inputFile *os.File, options map[ChartName]map[ChartOption]string,
) ([]*BenchmarkResults, error) {
	inputScanner := bufio.NewScanner(inputFile)
	inputScanner.Split(bufio.ScanLines)

	benchmarkToCases := make(map[ChartName]*BenchmarkResults)
	benchmarkNamesOrder := make([]ChartName, 0)
	firstLine := ""
	yAxisLabel := ""

	lineCount := 0
	for inputScanner.Scan() {
		lineCount++

		line := strings.TrimSpace(inputScanner.Text())
		if len(line) == 0 {
			continue
		}

		if strings.HasPrefix(line, "name") && firstLine != "" {
			break // TODO support separate graphs for allocations
		}

		const minNumberOfCellsInOneRow = 3

		if firstLine == "" {
			firstLine = line

			cells := strings.Split(line, ",")
			if len(cells) < minNumberOfCellsInOneRow {
				return nil, fmt.Errorf("%w: on line [%v]: `%v`", ErrNotEnoughColumns, lineCount, line)
			}

			yAxisLabel = cells[1]

			continue
		}

		cells := strings.Split(line, ",")
		if len(cells) < minNumberOfCellsInOneRow {
			return nil, fmt.Errorf("%w: on line [%v]: `%v`", ErrNotEnoughColumns, lineCount, line)
		}

		chartName, caseName, xValueStr, xAxisName, errParsingAttributes := parseAttributes(cells[0], options)
		if errParsingAttributes != nil {
			return nil, fmt.Errorf("%w: on line [%v]: `%v`", errParsingAttributes, lineCount, line)
		}

		yValueStr, errorValueStr := cells[1], cells[2]

		point, errPointParsing := parsePoint(xValueStr, yValueStr, errorValueStr)
		if errPointParsing != nil {
			return nil, fmt.Errorf("%w: on line [%v]: `%v`", errPointParsing, lineCount, line)
		}

		cases, ok := benchmarkToCases[chartName]
		if !ok {
			benchmarkNamesOrder = append(benchmarkNamesOrder, chartName)
			cases = initBenchResult(chartName, xAxisName, yAxisLabel)
		}

		cases.Cases[caseName] = append(cases.Cases[caseName], point)
		benchmarkToCases[chartName] = cases
	}

	errApplyingOption := applyOptionsToResults(options, benchmarkToCases)
	if errApplyingOption != nil {
		return nil, errApplyingOption
	}

	benchmarkResults := make([]*BenchmarkResults, 0, len(benchmarkToCases))
	for _, benchmarkName := range benchmarkNamesOrder {
		benchmarkResults = append(benchmarkResults, benchmarkToCases[benchmarkName])
	}

	return benchmarkResults, nil
}

func applyOptionsToResults(
	options map[ChartName]map[ChartOption]string, benchmarkToCases map[ChartName]*BenchmarkResults,
) error {
	for optionsBenchName, options := range options {
		optionApplied := false

		for benchName, cases := range benchmarkToCases {
			if strings.HasPrefix(string(benchName), string(optionsBenchName)) {
				optionApplied = true

				for optionKey, optionValue := range options {
					cases.Options[optionKey] = optionValue
				}
			}
		}

		if !optionApplied {
			return fmt.Errorf(
				"%w: %v: `%v`, %v",
				ErrOptionChartNameNotFound, "you've passed options for chart with name",
				optionsBenchName, "but we didn't find such benchmark within input file",
			)
		}
	}

	return nil
}

func initBenchResult(chartName ChartName, xAxisName string, yAxisLabel string) *BenchmarkResults {
	nameHash := sha256.Sum256([]byte(chartName))

	return &BenchmarkResults{
		ID:         hex.EncodeToString(nameHash[:]),
		Name:       chartName,
		Options:    map[ChartOption]string{"xAxisName": xAxisName},
		YAxisLabel: yAxisLabel,
		Cases:      make(map[string][]Point),
	}
}

func parsePoint(xValueStr string, yValueStr string, errorValueStr string) (Point, error) {
	const percents float64 = 100

	yValue, errParsingYValue := parseYValue(yValueStr)
	if errParsingYValue != nil {
		return Point{}, errParsingYValue
	}

	errorRate, errParsingErrorRate := parseErrorRate(errorValueStr)
	if errParsingErrorRate != nil {
		return Point{}, errParsingErrorRate
	}

	point := Point{
		X: xValueStr, Y: yValueStr, Error: fmt.Sprintf("%v", yValue*(errorRate/percents)),
	}

	return point, nil
}

func parseAttributes(
	firstCell string, options map[ChartName]map[ChartOption]string,
) (ChartName, string, string, string, error) {
	benchmarkName, restOfTheCell, ok := strings.Cut(firstCell, "/")
	if !ok {
		return "", "", "", "", fmt.Errorf("%w: `%v`", ErrCantParseBenchmarkName, firstCell)
	}

	measurementAttributesString, _, ok := strings.Cut(restOfTheCell, "-")
	if !ok {
		return "", "", "", "", fmt.Errorf("%w: `%v`", ErrCantParseMeasurementAttributes, restOfTheCell)
	}

	caseName := ""
	xValue := ""
	xAxisName, xAxisNameDefined := options[ChartName(benchmarkName)]["xAxisName"]
	benchmarkAttributes := []string{benchmarkName}
	measurementAttributes := strings.Split(measurementAttributesString, ";")

	for i, attribute := range measurementAttributes {
		attributeKeyAndValue := strings.Split(attribute, ":")
		if xAxisNameDefined && xAxisName == attributeKeyAndValue[0] {
			xValue = attributeKeyAndValue[1]
			continue
		}

		if i == len(measurementAttributes)-1 {
			xAxisName, xValue = attributeKeyAndValue[0], attributeKeyAndValue[1]
			continue
		}

		if attributeKeyAndValue[0] == "type" {
			caseName = attributeKeyAndValue[1]
			continue
		}

		benchmarkAttributes = append(benchmarkAttributes, attributeKeyAndValue[0]+"="+attributeKeyAndValue[1])
	}

	if caseName == "" {
		return "", "", "", "", fmt.Errorf(
			"%w: `%v`", ErrMeasurementLineHasNoTypeAttribute, measurementAttributesString,
		)
	}

	chartName := ChartName(strings.Join(benchmarkAttributes, " "))

	return chartName, caseName, xValue, xAxisName, nil
}

func parseYValue(secondCell string) (float64, error) {
	bitSize := 64

	yValue, errYValue := strconv.ParseFloat(secondCell, bitSize)
	if errYValue != nil {
		return 0, fmt.Errorf("%w: `%v`", ErrCantParseYValue, secondCell)
	}

	return yValue, nil
}

func parseErrorRate(thirdCell string) (float64, error) {
	bitSize := 64

	if len(thirdCell) < 2 || thirdCell[len(thirdCell)-1] != '%' {
		return 0, fmt.Errorf("%w: error rate cell should end with percent symbol `%v`", ErrCantParseErrorRate, thirdCell)
	}

	errorRateString := thirdCell[:len(thirdCell)-1]

	errorRate, errParsingErrorRate := strconv.ParseFloat(errorRateString, bitSize)
	if errParsingErrorRate != nil {
		return 0, fmt.Errorf("%w: `%v`", ErrCantParseErrorRate, thirdCell)
	}

	return errorRate, nil
}

const helpMessage = `
Not enough arguments. specify input file path first and output file path second
example:
> benchart input.csv result.html

you can also specify some options for charts with name of the chart at the beginning:
> benchart 'PoolOverhead;title=Overhead;xAxisName=Number of tasks;xAxisType=log;yAxisType=log' input.csv result.html

list of supported chart options: `

func parseOptions(cliArguments []string) (string, string, map[ChartName]map[ChartOption]string, error) {
	supportedChartOptions := map[ChartOption]ChartOptionTypeSet{
		"title":     {"string": struct{}{}},
		"xAxisName": {"string": struct{}{}},
		"xAxisType": {"log": struct{}{}},
		"yAxisType": {"log": struct{}{}},
	}

	minArgumentsCount := 3
	if len(cliArguments) < minArgumentsCount {
		return "", "", nil, fmt.Errorf("%w: %v%v", ErrNotEnoughInputArguments, helpMessage, supportedChartOptions)
	}

	inputFilePath := cliArguments[len(cliArguments)-2]
	outputFilePath := cliArguments[len(cliArguments)-1]
	options := make(map[ChartName]map[ChartOption]string)

	if len(cliArguments) == minArgumentsCount {
		return inputFilePath, outputFilePath, options, nil
	}

	optionArguments := cliArguments[1 : len(cliArguments)-2]
	for _, optionsString := range optionArguments {
		chartName, otherOptions, ok := strings.Cut(optionsString, ";")
		if !ok {
			return "", "", nil, fmt.Errorf("%w: %+v", ErrCantParseChartOptions, optionsString)
		}

		chartOptions, ok := options[ChartName(chartName)]
		if !ok {
			chartOptions = make(map[ChartOption]string)
		}

		for _, optionString := range strings.Split(otherOptions, ";") {
			optionSlice := strings.Split(optionString, "=")
			optionPartsCount := 2

			if len(optionSlice) != optionPartsCount {
				return "", "", nil, fmt.Errorf("%w: %+v", ErrCantParseOption, optionString)
			}

			chartOption := ChartOption(optionSlice[0])

			optionTypeSet, ok := supportedChartOptions[chartOption]
			if !ok {
				return "", "", nil, fmt.Errorf(
					"%w: %+v; List of suported options: %v",
					ErrOptionIsNotSupported, optionString, supportedChartOptions,
				)
			}

			chartOptionValue := optionSlice[1]
			_, stringAllowed := optionTypeSet["string"]
			_, thisOptionAllowed := optionTypeSet[ChartOptionType(chartOptionValue)]

			if stringAllowed || thisOptionAllowed {
				chartOptions[chartOption] = chartOptionValue
				continue
			} else {
				return "", "", nil, fmt.Errorf(
					"%w: option %v allows only %v",
					ErrOptionTypeIsWrong, chartOption, optionTypeSet,
				)
			}
		}

		options[ChartName(chartName)] = chartOptions
	}

	return inputFilePath, outputFilePath, options, nil
}

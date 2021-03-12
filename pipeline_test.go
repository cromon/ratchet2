package ratchet2_test

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cromon/ratchet2"
	"github.com/cromon/ratchet2/data"
	"github.com/cromon/ratchet2/logger"
	"github.com/cromon/ratchet2/processors"
)

// dummyProcessorDuration is the amount of time ProcessData will spend waiting before it returns.
const dummyProcessorDuration = 3

// dummyProcessorConcurrency is the number of concurrent calls to ProcessData a dummyConcurrentProcessor object can make at a time.
const dummyProcessorConcurrency = 2

// dummyReader is a simple stream which pulls values in order from an array.
type dummyReader struct {
	data [4]string
}

func (dr *dummyReader) String() string {
	return "dummyReader"
}

func (dr *dummyReader) ProcessData(d data.JSON, outputChan chan data.JSON, killChan chan error) {
	for i := range dr.data {
		outputChan <- data.JSON([]byte(dr.data[i]))
	}
}

func (dr *dummyReader) Finish(outputChan chan data.JSON, killChan chan error) {
}

// dummyConcurrentProcessor is an object designed to allow easy testing of the methods used by ConcurrentDataProcessors.
type dummyConcurrentProcessor struct{}

func (dp *dummyConcurrentProcessor) String() string {
	return "dummyConcurrentProcessor"
}

func (dp *dummyConcurrentProcessor) Concurrency() int {
	return dummyProcessorConcurrency
}

func (dp *dummyConcurrentProcessor) ProcessData(d data.JSON, outputChan chan data.JSON, killChan chan error) {
	time.Sleep(dummyProcessorDuration * time.Second)
	outputChan <- d
}

func (dp *dummyConcurrentProcessor) Finish(outputChan chan data.JSON, killChan chan error) {
}

// dummyProcessor is an object designed to allow easy testing of the methods used by DataProcessors.
type dummyProcessor struct{}

func (dp *dummyProcessor) String() string {
	return "dummyProcessor"
}

func (dp *dummyProcessor) ProcessData(d data.JSON, outputChan chan data.JSON, killChan chan error) {
	time.Sleep(dummyProcessorDuration * time.Second)
	outputChan <- d
}

func (dp *dummyProcessor) Finish(outputChan chan data.JSON, killChan chan error) {
}

// dummyWriter is a simple store of array values.
type dummyWriter struct {
	i    int
	data [4]string
}

func (dw *dummyWriter) String() string {
	return "dummyWriter"
}

func (dw *dummyWriter) ProcessData(d data.JSON, outputChan chan data.JSON, killChan chan error) {
	dw.data[dw.i] = string(d)
	dw.i++
}

func (dw *dummyWriter) Finish(outputChan chan data.JSON, killChan chan error) {
}

func TestDataProcessor(t *testing.T) {
	logger.LogLevel = logger.LevelDebug

	data := [4]string{"hi", "there", "guys", "!"}
	writer := dummyWriter{}
	pipeline := ratchet2.NewPipeline(&dummyReader{data: data}, &dummyProcessor{}, &writer)

	start := time.Now()
	err := <-pipeline.Run()
	end := time.Now()

	// This should take about
	// (len(data) * dummyProcessorDuration) + 1
	// seconds to finish.
	//
	// One second is added to account for other processing time.
	expectedDuration := time.Duration((len(data)*dummyProcessorDuration)+1) * time.Second
	if end.Sub(start) > expectedDuration {
		t.Errorf("Expected pipeline to finish in ~%s, finished in %s", expectedDuration, end.Sub(start))
	}
	if err != nil {
		t.Error("An error occurred in the ratchet2 pipeline:", err.Error())
	}
	if data != writer.data {
		t.Errorf("Expected %#v to be passed through the pipeline, got %#v", data, writer.data)
	}
}

func TestConcurrentDataProcessor(t *testing.T) {
	logger.LogLevel = logger.LevelDebug

	data := [4]string{"hi", "there", "guys", "!"}
	writer := dummyWriter{}
	pipeline := ratchet2.NewPipeline(&dummyReader{data: data}, &dummyConcurrentProcessor{}, &writer)

	start := time.Now()
	err := <-pipeline.Run()
	end := time.Now()

	// This should take about
	// (len(data) * dummyProcessorDuration / dummyConcurrentProcessorConcurrency) + 1
	// seconds to finish.
	//
	// One second is added to account for other processing time.
	expectedDuration := time.Duration((len(data)*dummyProcessorDuration/dummyProcessorConcurrency)+1) * time.Second
	if end.Sub(start) > expectedDuration {
		t.Errorf("Expected pipeline to finish in ~%s, finished in %s", expectedDuration, end.Sub(start))
	}
	if err != nil {
		t.Error("An error occurred in the ratchet2 pipeline:", err.Error())
	}
	if data != writer.data {
		t.Errorf("Expected %#v to be passed through the pipeline, got %#v", data, writer.data)
	}
}

func TestConcurrentFuncTransformer(t *testing.T) {
	logger.LogLevel = logger.LevelDebug

	dataSlice := [4]string{"hi", "there", "guys", "!"}
	expected := [4]string{"HI", "THERE", "GUYS", "!"}
	writer := dummyWriter{}

	// Use a real FuncTransformer instead of a dummyConcurrentProcessor
	transformer := processors.NewFuncTransformer(func(d data.JSON) data.JSON {
		time.Sleep(3 * time.Second)
		return data.JSON(strings.ToUpper(string(d)))
	})
	transformer.ConcurrencyLevel = dummyProcessorConcurrency

	pipeline := ratchet2.NewPipeline(&dummyReader{data: dataSlice}, transformer, &writer)

	start := time.Now()
	err := <-pipeline.Run()
	end := time.Now()

	// This should take about
	// (len(data) * dummyProcessorDuration / dummyConcurrentProcessorConcurrency) + 1
	// seconds to finish.
	//
	// One second is added to account for other processing time.
	expectedDuration := time.Duration((len(dataSlice)*dummyProcessorDuration/dummyProcessorConcurrency)+1) * time.Second
	if end.Sub(start) > expectedDuration {
		t.Errorf("Expected pipeline to finish in ~%s, finished in %s", expectedDuration, end.Sub(start))
	}
	if err != nil {
		t.Error("An error occurred in the ratchet2 pipeline:", err.Error())
	}
	if expected != writer.data {
		t.Errorf("Expected transform results %#v, got %#v", expected, writer.data)
	}
}

func ExampleNewPipeline() {
	logger.LogLevel = logger.LevelSilent

	// A basic pipeline is created using one or more DataProcessor instances.
	hello := processors.NewIoReader(strings.NewReader("Hello world!"))
	stdout := processors.NewIoWriter(os.Stdout)
	pipeline := ratchet2.NewPipeline(hello, stdout)

	err := <-pipeline.Run()

	if err != nil {
		fmt.Println("An error occurred in the ratchet2 pipeline:", err.Error())
	}

	// Output:
	// Hello world!
}

func ExampleNewBranchingPipeline() {
	logger.LogLevel = logger.LevelSilent

	// This example is very contrived, but we'll first create
	// DataProcessors that will spit out strings, do some basic
	// transformation, and then filter out all the ones that don't
	// match "HELLO".
	hello := processors.NewIoReader(strings.NewReader("Hello world"))
	hola := processors.NewIoReader(strings.NewReader("Hola mundo"))
	bonjour := processors.NewIoReader(strings.NewReader("Bonjour monde"))
	upperCaser := processors.NewFuncTransformer(func(d data.JSON) data.JSON {
		return data.JSON(strings.ToUpper(string(d)))
	})
	lowerCaser := processors.NewFuncTransformer(func(d data.JSON) data.JSON {
		return data.JSON(strings.ToLower(string(d)))
	})
	helloMatcher := processors.NewRegexpMatcher("HELLO")
	stdout := processors.NewIoWriter(os.Stdout)

	// Create the PipelineLayout that will run the DataProcessors
	layout, err := ratchet2.NewPipelineLayout(
		// Stage 1 - spits out hello world in a few languages
		ratchet2.NewPipelineStage(
			ratchet2.Do(hello).Outputs(upperCaser, lowerCaser),
			ratchet2.Do(hola).Outputs(upperCaser),
			ratchet2.Do(bonjour).Outputs(lowerCaser),
		),
		// Stage 2 - transforms strings to upper and lower case
		ratchet2.NewPipelineStage(
			ratchet2.Do(upperCaser).Outputs(helloMatcher),
			ratchet2.Do(lowerCaser).Outputs(helloMatcher),
		),
		// Stage 3 - only lets through strings that match "hello"
		ratchet2.NewPipelineStage(
			ratchet2.Do(helloMatcher).Outputs(stdout),
		),
		// Stage 4 - prints to STDOUT
		ratchet2.NewPipelineStage(
			ratchet2.Do(stdout),
		),
	)
	if err != nil {
		panic(err.Error())
	}

	// Create and run the Pipeline
	pipeline := ratchet2.NewBranchingPipeline(layout)
	err = <-pipeline.Run()

	if err != nil {
		fmt.Println("An error occurred in the ratchet2 pipeline:", err.Error())
	}

	// Output:
	// HELLO WORLD
}

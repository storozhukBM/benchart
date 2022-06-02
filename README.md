# benchart [![Build Status](https://github.com/storozhukBM/benchart/workflows/build/badge.svg)](https://github.com/storozhukBM/benchart/actions)  [![Go Report Card](https://goreportcard.com/badge/github.com/storozhukBM/benchart)](https://goreportcard.com/report/github.com/storozhukBM/benchart) [![PkgGoDev](https://pkg.go.dev/badge/github.com/storozhukBM/benchart)](https://pkg.go.dev/github.com/storozhukBM/benchart)

benchart is a tool that takes [benchstat -csv](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat#section-readme)
output as an input and plots results of your benchmark in a html file.
All you need to do is format your benchmark's name properly.

<img width="1047" alt="Chart 1" src="https://user-images.githubusercontent.com/3532750/171732488-e6dac606-23fc-41f3-80ba-ce8619e70c52.png">


All measurement attributes that benchart parses should be split by `;` and every attribute consists of key and value
separated by `:`. Every measurement should have at least 2 attributes: one it `type` that will be used to label your
chart and one additional attribute that will act as `x-axis` by default the last attribute will be interpreted
as  `x-axis`.

```
name,time/op (ns/op),±
Hash/type:crc32;bytes:8-8,4.47967E+00,0%
     |________| |_____|
      mandatory  x-axis
      attribute
```

You can also provide additional chart option sets. Every chart option set should have chart name at the beginning and then set
of options separated by `;`. Every option in a set should have key and value separated by `=`.
Supported options:
 - `title` - string start will be used as a graph title instead of name from csv file
 - `xAxisName` - overrides x-axis name on graph
 - `xAxisType` - changes type of x-axis, by default we use linear scale, but you can specify `log` scale 
 - `yAxisType` - changes type of y-axis, by default we use linear scale, but you can specify `log` scale 

Example of a command with options specified 

> benchart 'Hash;title=Benchmark of hash functions;xAxisName=bytes size;xAxisType=log' input.csv result.html

Other examples of benchart usage are in the testdata directory.

<img width="1066" alt="Chart 2" src="https://user-images.githubusercontent.com/3532750/171732884-9a55592d-34f8-47fc-82dc-00c895549c03.png">

<img width="1038" alt="Chart 3" src="https://user-images.githubusercontent.com/3532750/171733053-506ab02b-5c56-45b5-8d62-ea615d1fde28.png">



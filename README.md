#gominer
CPU miner for simpleChain in go

## Binary releases

## Installation from source

### Prerequisites
* go version 1.10.1 or above (earlier version might work or not), check with `go version`
* gcc

```
go get git.dev.tencent.com/baoquan2017/gominer
```

## Run
```
gominer
```

Usage:
```
  gominer [global options] 
  
  VERSION:
     1.0
  
  COMMANDS:
     help  Shows a list of commands or help for one command
     
  GLOBAL OPTIONS:
     --stratumserver value  stratum server address,(host:port)
     --minername value      miner name registered to the stratum server
     --password value       password of stratum server if it's necessary
     --minerthreads value   Number of CPU threads to use for mining (default: 8)
     --verbosity value      Logging verbosity: 0=silent, 1=error, 2=warn, 3=info, 4=debug, 5=detail (default: 3) (default: 3)
     --help, -h             show help
```

##EXAMPLES
```
./gominer --stratumserver host:port --minername x --password  x --minerthreads 2
```
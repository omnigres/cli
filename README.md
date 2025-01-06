# Omnigres CLI

This is a Command Line Interface to interact with [Omnigres](https://github.com/omnigres/omnigres).
Also check the [Omnigres documentation](https://docs.omnigres.org/) for more info about the platform.

## Quick start

First ensure you have a working [go development environment](https://go.dev/dl/).

```sh 
go build -o og cmd/omnigres/main.go
```

The command above should compile a binary `og` at the project's root folder.

In order to provision an Omnigres cluster you should have the Docker CLI installed.
Check their [Get Started page](https://www.docker.com/get-started/) to install the Docker tools on your system.

Go ahead and try to start a transient cluster, all state will be erased at the end.

```sh 
./og run
```

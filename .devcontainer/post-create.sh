#!/bin/bash
set -e
lefthook install 
make install
make tidy 
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
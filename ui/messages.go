package ui

import (
	infraaws "lazyinfra/aws"

	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

type errMsg struct {
	Service string
	Err     error
}

type lambdaListLoadedMsg []types.FunctionConfiguration
type apiListLoadedMsg []infraaws.API
type logGroupsLoadedMsg []infraaws.LogGroup
type logLinesAppendedMsg []string
type distributionsLoadedMsg []infraaws.Distribution

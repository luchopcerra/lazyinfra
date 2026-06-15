package ui

import infraaws "lazyinfra/aws"

type errMsg struct {
	Service string
	Err     error
}

type lambdaListLoadedMsg []infraaws.LambdaFunction
type apiListLoadedMsg []infraaws.API
type logGroupsLoadedMsg []infraaws.LogGroup
type logLinesAppendedMsg []string
type distributionsLoadedMsg []infraaws.Distribution

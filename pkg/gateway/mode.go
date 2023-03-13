package gateway

import (
	"flag"
	"io"
	"os"
)

// EnvGatewayMode indicates environment name for gateway mode.
const EnvGatewayMode = "GATEWAY_MODE"

const (
	// DebugMode indicates gateway mode is debug.
	DebugMode = "debug"
	// ReleaseMode indicates gateway mode is release.
	ReleaseMode = "release"
	// TestMode indicates gateway mode is test.
	TestMode = "test"
)

const (
	debugCode = iota
	releaseCode
	testCode
)

// DefaultWriter is the default io.Writer used by Gateway for debug output and
// middleware output like Logger() or Recovery().
// Note that both Logger and Recovery provides custom ways to configure their
// output io.Writer.
// To support coloring in Windows use:
//
//	import "github.com/mattn/go-colorable"
//	gateway.DefaultWriter = colorable.NewColorableStdout()
var DefaultWriter io.Writer = os.Stdout

// DefaultErrorWriter is the default io.Writer used by Gateway to debug errors
var DefaultErrorWriter io.Writer = os.Stderr

var (
	gatewayMode = debugCode
	modeName    = DebugMode
)

func init() {
	mode := os.Getenv(EnvGatewayMode)
	SetMode(mode)
}

// SetMode sets gateway mode according to input string.
func SetMode(value string) {
	if value == "" {
		if flag.Lookup("test.v") != nil {
			value = TestMode
		} else {
			value = DebugMode
		}
	}

	switch value {
	case DebugMode:
		gatewayMode = debugCode
	case ReleaseMode:
		gatewayMode = releaseCode
	case TestMode:
		gatewayMode = testCode
	default:
		panic("gateway mode unknown: " + value + " (available mode: debug release test)")
	}

	modeName = value
}

// Mode returns current gin mode.
func Mode() string {
	return modeName
}
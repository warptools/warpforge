package healthcheck

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/wfapi"
)

const (
	CodeRunOkay      = "warpforge-error-healthcheck-run-okay"
	CodeRunFailure   = "warpforge-error-healthcheck-run-fail"
	CodeRunAmbiguous = "warpforge-error-healthcheck-run-ambiguous"
)

type HealthCheckStatus int

const (
	// StatusNone is the zero value and used for unset status value
	StatusNone HealthCheckStatus = iota
	StatusOkay
	StatusFail
	StatusAmbiguous
	StatusUnknown
)

// Characters used to display status
const (
	StatusCharacter_None      = "∅"
	StatusCharacter_Okay      = "✔"
	StatusCharacter_Failure   = "✘"
	StatusCharacter_Ambiguous = "?"
	StatusCharacter_Unknown   = "!"
)

func (s HealthCheckStatus) String() string {
	switch s {
	case StatusNone:
		return StatusCharacter_None
	case StatusOkay:
		return StatusCharacter_Okay
	case StatusAmbiguous:
		return StatusCharacter_Ambiguous
	case StatusFail:
		return StatusCharacter_Failure
	default:
		return StatusCharacter_Unknown
	}
}

type Runner interface {
	// Run returns a serum error that contains a human readable message and a status code.
	// Runner should not return a nil
	// Errors:
	//
	//    - warpforge-error-healthcheck-run-okay --
	//    - warpforge-error-healthcheck-run-fail --
	//    - warpforge-error-healthcheck-run-ambiguous --
	Run(context.Context) error
	// Should be a header of some kind for the checker type
	String() string
}

const DefaultRootWorkspacePrefix = "warpforge-health-check-root-workspace-"

var DefaultBasePath = os.TempDir()

type HealthCheck struct {
	Runners []Runner
	Results []serum.ErrorInterfaceWithMessage
}

// Run executes all the runners assigned to this health check
// Errors: none -- Errors are just stored for later
func (h *HealthCheck) Run(ctx context.Context) error {
	log := logging.Ctx(ctx)
	h.Results = make([]serum.ErrorInterfaceWithMessage, 0, len(h.Runners))
	for i, runnable := range h.Runners {
		log.Debug("", "healtcheck runner %d", i)
		err := runnable.Run(ctx)
		result, ok := err.(serum.ErrorInterfaceWithMessage)
		if !ok {
			result = serum.Errorf(CodeRunFailure, "runner has invalid interface: %w", err).(serum.ErrorInterfaceWithMessage)
		}
		h.Results = append(h.Results, result)
	}
	return nil
}

// Fprint emits formatted text of run results to the writer
// Errors:
//
//     - warpforge-error-internal -- when the health check was not run before printing results
func (h *HealthCheck) Fprint(w io.Writer) error {
	if len(h.Runners) != len(h.Results) {
		return serum.Error(wfapi.ECodeInternal,
			serum.WithMessageLiteral("HealtCheck must run before printing results"),
		)
	}
	headers := make([]string, 0, len(h.Runners))
	maxHeaderLen := 0
	for _, runner := range h.Runners {
		header := runner.String()
		headers = append(headers, header)
		if len(header) > maxHeaderLen {
			maxHeaderLen = len(header)
		}
	}
	for i, result := range h.Results {
		status := Status(result)
		statusStr := h.TermColor(status).Sprint(status)
		fmt.Fprintf(w, " %s  %-*s\t%s\n", statusStr, maxHeaderLen, headers[i], result.Message())
	}
	return nil
}

func (h *HealthCheck) TermColor(s HealthCheckStatus) *color.Color {
	result := color.New()
	switch s {
	case StatusNone:
		return result.Add(color.Reset)
	case StatusOkay:
		return result.Add(color.FgHiGreen, color.Bold)
	case StatusAmbiguous:
		return result.Add(color.FgHiYellow, color.Bold)
	case StatusFail:
		return result.Add(color.FgHiRed, color.Bold)
	default:
		return result.Add(color.FgHiMagenta, color.Bold)
	}
}

// Status converts serum codes to status enumeration values
func Status(err error) HealthCheckStatus {
	if err == nil {
		return StatusNone
	}
	_, ok := err.(serum.ErrorInterface)
	if !ok {
		return StatusNone
	}
	switch serum.Code(err) {
	case CodeRunFailure:
		return StatusFail
	case CodeRunOkay:
		return StatusOkay
	case CodeRunAmbiguous:
		return StatusAmbiguous
	default:
		return StatusUnknown
	}
}

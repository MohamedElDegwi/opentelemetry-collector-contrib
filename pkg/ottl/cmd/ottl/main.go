package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func main() {
	// Create telemetry settings with a development logger
	logger, _ := zap.NewDevelopment()
	settings := component.TelemetrySettings{
		Logger: logger,
	}

	// Create the OTTL parser with standard functions
	parser, err := ottllog.NewParser(
		ottlfuncs.StandardFuncs[*ottllog.TransformContext](),
		settings,
	)
	if err != nil {
		fmt.Printf("Failed to create parser: %v\n", err)
		return
	}

	// Create sample log data to work with
	tCtx := createSampleLogContext()
	defer tCtx.Close()

	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("OTTL REPL - OpenTelemetry Transformation Language Experiment")
	fmt.Println("Commands:")
	fmt.Println("  exit       - quit the REPL")
	fmt.Println("  show       - display current log record attributes")
	fmt.Println("  reset      - reset log data to initial state")
	fmt.Println("  <statement> - execute an OTTL statement")
	fmt.Println()
	printAttributes(tCtx)

	for {
		fmt.Print("\nottl> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "exit" {
			break
		}
		if input == "" {
			continue
		}
		if input == "show" {
			printAttributes(tCtx)
			continue
		}
		if input == "reset" {
			tCtx.Close()
			tCtx = createSampleLogContext()
			fmt.Println("Log data reset to initial state")
			printAttributes(tCtx)
			continue
		}

		// Try parsing as a statement (editor)
		stmt, err := parser.ParseStatement(input)
		if err != nil {
			// Try parsing as a value expression (converter)
			valueExpr, valueErr := parser.ParseValueExpression(input)
			if valueErr != nil {
				fmt.Printf("Parse error: %v\n", err)
				continue
			}

			// Evaluate the value expression
			result, evalErr := valueExpr.Eval(context.Background(), tCtx)
			if evalErr != nil {
				fmt.Printf("Eval error: %v\n", evalErr)
				continue
			}
			fmt.Printf("Result: %v\n", formatValue(result))
			continue
		}

		// Execute the statement
		result, conditionMatched, err := stmt.Execute(context.Background(), tCtx)
		if err != nil {
			fmt.Printf("Execution error: %v\n", err)
			continue
		}

		if conditionMatched {
			fmt.Println("✓ Statement executed (condition matched)")
			if result != nil {
				fmt.Printf("  Return value: %v\n", formatValue(result))
			}
		} else {
			fmt.Println("✗ Statement skipped (condition not matched)")
		}
	}

	fmt.Println("\nGoodbye!")
}

// createSampleLogContext creates a log record with sample data for experimentation
func createSampleLogContext() *ottllog.TransformContext {
	rLogs := plog.NewResourceLogs()
	rLogs.Resource().Attributes().PutStr("host.name", "localhost")
	rLogs.Resource().Attributes().PutStr("service.name", "my-service")

	scope := rLogs.ScopeLogs().AppendEmpty().Scope()
	scope.SetName("my-scope")
	scope.SetVersion("1.0.0")

	logRecord := rLogs.ScopeLogs().At(0).LogRecords().AppendEmpty()
	logRecord.Body().SetStr("This is a sample log message")
	logRecord.SetSeverityNumber(plog.SeverityNumberInfo)
	logRecord.SetSeverityText("INFO")

	// Add some sample attributes
	logRecord.Attributes().PutStr("http.method", "GET")
	logRecord.Attributes().PutStr("http.path", "/api/users")
	logRecord.Attributes().PutInt("http.status_code", 200)
	logRecord.Attributes().PutStr("user.id", "user-123")
	logRecord.Attributes().PutBool("cache.hit", true)
	logRecord.Attributes().PutDouble("response.time_ms", 45.7)

	// Nested map
	m := logRecord.Attributes().PutEmptyMap("request")
	m.PutStr("id", "req-456")
	m.PutStr("source", "web")

	// Slice
	tags := logRecord.Attributes().PutEmptySlice("tags")
	tags.AppendEmpty().SetStr("production")
	tags.AppendEmpty().SetStr("api")
	tags.AppendEmpty().SetStr("v2")

	return ottllog.NewTransformContextPtr(rLogs, rLogs.ScopeLogs().At(0), logRecord)
}

// printAttributes displays the current log record attributes
func printAttributes(tCtx *ottllog.TransformContext) {
	fmt.Println("\n--- Current Log Record ---")
	fmt.Printf("Body: %s\n", tCtx.GetLogRecord().Body().AsString())
	fmt.Printf("Severity: %s (%d)\n", tCtx.GetLogRecord().SeverityText(), tCtx.GetLogRecord().SeverityNumber())
	fmt.Println("Attributes:")
	tCtx.GetLogRecord().Attributes().Range(func(k string, v pcommon.Value) bool {
		fmt.Printf("  %s: %v\n", k, v.AsRaw())
		return true
	})
	fmt.Println("--------------------------")
}

// formatValue formats any value for display
func formatValue(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

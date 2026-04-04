//go:build js && wasm

package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/faizanhussain/arbiter/pkg/engine"
)

// evaluate is the JavaScript-callable function.
// Signature: evaluate(treeJSON: string, contextJSON: string, ruleID: string, defaultValueJSON?: string) => string
//
// Returns a JSON string with the EvalResult shape:
//
//	{"value": ..., "path": [...], "default": bool, "error": "", "elapsed": "..."}
func evaluate(this js.Value, args []js.Value) any {
	if len(args) < 3 {
		return toJSResult(engine.EvalResult{
			Error: "evaluate requires at least 3 arguments: treeJSON, contextJSON, ruleID",
			Path:  []string{},
		})
	}

	treeJSON := args[0].String()
	contextJSON := args[1].String()
	ruleID := args[2].String()

	var defaultValue json.RawMessage
	if len(args) > 3 && args[3].Type() == js.TypeString {
		dv := args[3].String()
		if dv != "" {
			defaultValue = json.RawMessage(dv)
		}
	}

	// Parse context
	var ctx map[string]any
	if err := json.Unmarshal([]byte(contextJSON), &ctx); err != nil {
		return toJSResult(engine.EvalResult{
			Error: "invalid context JSON: " + err.Error(),
			Path:  []string{},
		})
	}

	result := engine.Evaluate(json.RawMessage(treeJSON), ctx, ruleID, defaultValue)
	return toJSResult(result)
}

// validateRule validates a rule's structure.
// Signature: validateRule(ruleJSON: string) => string (error message, empty if valid)
func validateRule(this js.Value, args []js.Value) any {
	if len(args) < 1 {
		return "validateRule requires 1 argument: ruleJSON"
	}

	var rule engine.Rule
	if err := json.Unmarshal([]byte(args[0].String()), &rule); err != nil {
		return "invalid JSON: " + err.Error()
	}

	if err := engine.ValidateRule(&rule); err != nil {
		return err.Error()
	}
	return ""
}

func toJSResult(result engine.EvalResult) string {
	b, _ := json.Marshal(result)
	return string(b)
}

func main() {
	// Register functions on the global arbiter object
	arbiter := js.Global().Get("Object").New()
	arbiter.Set("evaluate", js.FuncOf(evaluate))
	arbiter.Set("validateRule", js.FuncOf(validateRule))
	js.Global().Set("arbiter", arbiter)

	// Keep the Go program running
	select {}
}

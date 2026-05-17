package output

import (
	"encoding/json"
	"fmt"
	"io"
)

type Envelope struct {
	OK      bool        `json:"ok"`
	Data    any         `json:"data,omitempty"`
	Summary string      `json:"summary,omitempty"`
	Error   *ErrorValue `json:"error,omitempty"`
}

type ErrorValue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func Success(w io.Writer, jsonOutput bool, data any, summary string) error {
	if jsonOutput {
		return writeJSON(w, Envelope{OK: true, Data: data, Summary: summary})
	}
	if summary != "" {
		_, err := fmt.Fprintln(w, summary)
		return err
	}
	return nil
}

func Error(w io.Writer, jsonOutput bool, code, message string) error {
	if jsonOutput {
		return writeJSON(w, Envelope{
			OK:    false,
			Error: &ErrorValue{Code: code, Message: message},
		})
	}
	_, err := fmt.Fprintf(w, "Error: %s\n", message)
	return err
}

func JSONLine(w io.Writer, data any) error {
	content, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(content))
	return err
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

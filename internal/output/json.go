package output

import (
	"encoding/json"
	"io"
)

func RenderJSON(w io.Writer, report interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

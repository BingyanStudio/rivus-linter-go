package output

import (
	"encoding/json"
	"io"

	"github.com/BingyanStudio/rivus-linter-go/internal/model"
)

// JSON formats the analysis result as JSON.
func JSON(w io.Writer, result *model.AnalysisResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

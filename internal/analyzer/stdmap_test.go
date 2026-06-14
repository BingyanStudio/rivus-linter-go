package analyzer

import (
	"testing"

	"github.com/BingyanStudio/rivus-linter-go/internal/model"
)

func TestLookupStdFlags(t *testing.T) {
	tests := []struct {
		name     string
		wantOK   bool
		wantFlag model.FlagType
	}{
		{"os.Exit", true, model.FlagExit},
		{"os.Open", true, model.FlagIO},
		{"time.Now", true, model.FlagTime},
		{"context.Background", true, model.FlagContext},
		{"fmt.Println", false, 0},
		{"math.Abs", false, 0},
	}
	for _, tt := range tests {
		flags, ok := LookupStdFlags(tt.name)
		if ok != tt.wantOK {
			t.Errorf("LookupStdFlags(%q): ok = %v, want %v", tt.name, ok, tt.wantOK)
		}
		if ok && !flags.Has(tt.wantFlag) {
			t.Errorf("LookupStdFlags(%q): missing flag %v", tt.name, tt.wantFlag)
		}
	}
}

package analyzer

import "github.com/BingyanStudio/rivus-linter-go/internal/model"

// stdFuncFlags maps "package.Function" to its FlagSet.
// This is used for standard library functions so we don't need to analyze their SSA.
var stdFuncFlags = map[string]model.FlagSet{
	// Panic
	"runtime.gopanic": model.FlagSet(model.FlagPanic),

	// Exit
	"os.Exit":           model.FlagSet(model.FlagExit),
	"log.Fatal":         model.FlagSet(model.FlagExit),
	"log.Fatalf":        model.FlagSet(model.FlagExit),
	"log.Fatalln":       model.FlagSet(model.FlagExit),
	"log.Logger.Fatal":   model.FlagSet(model.FlagExit),
	"log.Logger.Fatalf":  model.FlagSet(model.FlagExit),
	"log.Logger.Fatalln": model.FlagSet(model.FlagExit),

	// I/O - os
	"os.Open":      model.FlagSet(model.FlagIO),
	"os.Create":    model.FlagSet(model.FlagIO),
	"os.ReadFile":  model.FlagSet(model.FlagIO),
	"os.WriteFile": model.FlagSet(model.FlagIO),
	"os.Mkdir":     model.FlagSet(model.FlagIO),
	"os.MkdirAll":  model.FlagSet(model.FlagIO),
	"os.Remove":    model.FlagSet(model.FlagIO),
	"os.RemoveAll": model.FlagSet(model.FlagIO),
	"os.Rename":    model.FlagSet(model.FlagIO),
	"os.Stat":      model.FlagSet(model.FlagIO),

	// I/O - net
	"net.Dial":        model.FlagSet(model.FlagIO),
	"net.DialTimeout": model.FlagSet(model.FlagIO),
	"net.Listen":      model.FlagSet(model.FlagIO),

	// I/O - database/sql
	"database/sql.Open": model.FlagSet(model.FlagIO),

	// I/O - io
	"io.ReadFull":    model.FlagSet(model.FlagIO),
	"io.ReadAtLeast": model.FlagSet(model.FlagIO),
	"io.Copy":        model.FlagSet(model.FlagIO),
	"io.CopyN":       model.FlagSet(model.FlagIO),

	// Side Effect - env
	"os.Getenv":   model.FlagSet(model.FlagSideEffect),
	"os.LookupEnv": model.FlagSet(model.FlagSideEffect),
	"os.Setenv":   model.FlagSet(model.FlagSideEffect),
	"os.Unsetenv": model.FlagSet(model.FlagSideEffect),
	"os.Environ":  model.FlagSet(model.FlagSideEffect),

	// Side Effect - random
	"math/rand.Int":     model.FlagSet(model.FlagSideEffect),
	"math/rand.Intn":    model.FlagSet(model.FlagSideEffect),
	"math/rand.Float64": model.FlagSet(model.FlagSideEffect),
	"math/rand.Read":    model.FlagSet(model.FlagSideEffect),

	// Unsafe
	"unsafe.Pointer":    model.FlagSet(model.FlagUnsafe),
	"unsafe.Sizeof":     model.FlagSet(model.FlagUnsafe),
	"unsafe.Offsetof":   model.FlagSet(model.FlagUnsafe),
	"unsafe.Alignof":    model.FlagSet(model.FlagUnsafe),
	"unsafe.Add":        model.FlagSet(model.FlagUnsafe),
	"unsafe.Slice":      model.FlagSet(model.FlagUnsafe),
	"unsafe.SliceData":  model.FlagSet(model.FlagUnsafe),
	"unsafe.String":     model.FlagSet(model.FlagUnsafe),
	"unsafe.StringData": model.FlagSet(model.FlagUnsafe),

	// Time
	"time.Now":       model.FlagSet(model.FlagTime),
	"time.After":     model.FlagSet(model.FlagTime),
	"time.Sleep":     model.FlagSet(model.FlagTime),
	"time.Tick":      model.FlagSet(model.FlagTime),
	"time.NewTicker": model.FlagSet(model.FlagTime),
	"time.Since":     model.FlagSet(model.FlagTime),
	"time.Until":     model.FlagSet(model.FlagTime),

	// Reflection
	"reflect.TypeOf":    model.FlagSet(model.FlagReflect),
	"reflect.ValueOf":   model.FlagSet(model.FlagReflect),
	"reflect.DeepEqual": model.FlagSet(model.FlagReflect),

	// Context (dangling)
	"context.Background": model.FlagSet(model.FlagContext),
	"context.TODO":       model.FlagSet(model.FlagContext),
}

// LookupStdFlags returns the flags for a standard library function.
// The key should be in "package.Function" or "package.Type.Method" format.
// Returns (flags, true) if found, (0, false) if not a known stdlib function.
func LookupStdFlags(name string) (model.FlagSet, bool) {
	flags, ok := stdFuncFlags[name]
	return flags, ok
}

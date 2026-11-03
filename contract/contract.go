// Package contract는 sion.schema.json과 픽스처를 테스트에서 읽는다. 서버 바이너리는 이 패키지를 쓰지 않는다.
package contract

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const schemaFile = "sion.schema.json"

var (
	mu       sync.Mutex
	compiler *jsonschema.Compiler
	schemas  = map[string]*jsonschema.Schema{}
)

func dir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

func ReadFile(t testing.TB, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir(), name))
	if err != nil {
		t.Fatalf("contract file %s: %v", name, err)
	}
	return raw
}

func LoadJSON(t testing.TB, name string, v any) {
	t.Helper()
	if err := json.Unmarshal(ReadFile(t, name), v); err != nil {
		t.Fatalf("contract file %s: %v", name, err)
	}
}

func schema(t testing.TB, def string) *jsonschema.Schema {
	t.Helper()
	mu.Lock()
	defer mu.Unlock()
	if s, ok := schemas[def]; ok {
		return s
	}
	if compiler == nil {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(ReadFile(t, schemaFile)))
		if err != nil {
			t.Fatalf("parse %s: %v", schemaFile, err)
		}
		c := jsonschema.NewCompiler()
		if err := c.AddResource(schemaFile, doc); err != nil {
			t.Fatalf("load %s: %v", schemaFile, err)
		}
		compiler = c
	}
	s, err := compiler.Compile(schemaFile + "#/$defs/" + def)
	if err != nil {
		t.Fatalf("compile %s: %v", def, err)
	}
	schemas[def] = s
	return s
}

// Check는 raw JSON이 $defs/def를 따르지 않으면 그 이유를 돌려준다.
func Check(t testing.TB, def string, raw []byte) error {
	t.Helper()
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	return schema(t, def).Validate(inst)
}

func Validate(t testing.TB, def string, raw []byte) {
	t.Helper()
	if err := Check(t, def, raw); err != nil {
		t.Fatalf("%s does not match $defs/%s: %v", raw, def, err)
	}
}

func ValidateValue(t testing.TB, def string, v any) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	Validate(t, def, raw)
}

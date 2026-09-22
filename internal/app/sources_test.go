package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSourceSelection(t *testing.T) {
	dir := fixture(t, map[string]string{"A.md": "[[Target]] [[Missing]]", "B.md": "[[Ignored]]", "Target.md": ""})
	var out, errOut bytes.Buffer
	code := Run([]string{"unresolved", "--root", dir, "--source", "A.md", "--source", "A.md", "--format", "json", "--strict"}, &out, &errOut)
	var result struct {
		Sources     []string
		Results     []struct{ Target string }
		Diagnostics []struct{ Phase string }
	}
	if code != 3 || json.Unmarshal(out.Bytes(), &result) != nil || !reflect.DeepEqual(result.Sources, []string{"A.md"}) || len(result.Results) != 1 || result.Results[0].Target != "Missing" || result.Diagnostics[0].Phase != "source" {
		t.Fatal(code, out.String(), errOut.String())
	}
	empty := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(empty, nil, 0600); err != nil {
		t.Fatal(err)
	}
	checkRun(t, []string{"unresolved", "--root", dir, "--sources0-from", empty, "--strict", "--format", "json"}, 0, "{\"schema_version\":1,\"command\":\"unresolved\",\"root\":\".\",\"sources\":[],\"results\":[],\"diagnostics\":[]}\n", false)
}

func TestInvalidSources(t *testing.T) {
	dir := fixture(t, map[string]string{"A.md": "[[Missing]]", "file.txt": ""})
	for _, source := range []string{"", "A", "missing.md", "file.txt", "/A.md", "./A.md", "../A.md", "x//A.md", "A.md/", "A.md\x00", "\xff.md", "A%2Emd"} {
		checkRun(t, []string{"unresolved", "--root", dir, "--source", "A.md", "--source", source}, 2, "", true)
	}
	for _, command := range []string{"outgoing", "backlinks"} {
		checkRun(t, []string{command, "--root", dir, "--source", "A.md", "A"}, 2, "", true)
	}
	for _, data := range []string{"A.md", "\x00", "A.md\x00\x00", "\xff.md\x00"} {
		file := filepath.Join(t.TempDir(), "paths")
		if err := os.WriteFile(file, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		checkRun(t, []string{"unresolved", "--root", dir, "--sources0-from", file}, 2, "", true)
	}
	checkRun(t, []string{"unresolved", "--root", dir, "--sources0-from", filepath.Join(dir, "absent")}, 1, "", true)
}

func TestSourcesNULAndCatalog(t *testing.T) {
	dir := fixture(t, map[string]string{"A\n.md": "[[Missing]]", "B\t.md": "[[Missing]]", "Ignored.md": "[[OtherMissing]]"})
	file := filepath.Join(t.TempDir(), "paths")
	if err := os.WriteFile(file, []byte("B\t.md\x00A\n.md\x00"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Run([]string{"unresolved", "--root", dir, "--sources0-from", file, "--format", "json"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), `"sources":["A\n.md","B\t.md"]`) || strings.Contains(out.String(), `"target":"Ignored"`) {
		t.Fatal(code, out.String(), errOut.String())
	}
	if err := os.Symlink("Ignored.md", filepath.Join(dir, "alias.md")); err != nil {
		t.Fatal(err)
	}
	checkRun(t, []string{"unresolved", "--root", dir, "--source", "alias.md"}, 2, "", true)
	checkRun(t, []string{"unresolved", "--root", dir, "--source", "Ignored.md", "--exclude", "Ignored.md"}, 2, "", true)
	out.Reset()
	errOut.Reset()
	if code := Run([]string{"unresolved", "--root", dir, "--source", "Ignored.md", "--strict"}, &out, &errOut); code != 3 || !strings.Contains(errOut.String(), "phase=catalog") || !strings.Contains(errOut.String(), "phase=source") {
		t.Fatal(code, out.String(), errOut.String())
	}
}

func TestSourceStdinAndInputFailures(t *testing.T) {
	dir := fixture(t, map[string]string{"A.md": "", "B.md": "[[Missing]]"})
	for _, test := range []struct {
		input string
		extra []string
		want  string
	}{
		{"", nil, `"sources":[]`},
		{"B.md\x00A.md\x00", []string{"--source", "B.md"}, `"sources":["A.md","B.md"]`},
	} {
		var out, errOut bytes.Buffer
		args := append([]string{"unresolved", "--root", dir, "--sources0-from", "-", "--format", "json"}, test.extra...)
		if code := run(args, strings.NewReader(test.input), &out, &errOut); code != 0 || !strings.Contains(out.String(), test.want) {
			t.Fatal(code, out.String(), errOut.String())
		}
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"unresolved", "--root", dir, "--sources0-from", "-"}, brokenSourceInput{}, &out, &errOut); code != 1 || out.Len() != 0 {
		t.Fatal(code, out.String(), errOut.String())
	}
	for _, extra := range [][]string{{"--sources0-from", ""}, {"--sources0-from", "-", "--sources0-from", "-"}} {
		checkRun(t, append([]string{"unresolved", "--root", dir}, extra...), 2, "", true)
	}
	for _, command := range []string{"outgoing", "backlinks"} {
		checkRun(t, []string{command, "--root", dir, "--sources0-from", "-", "A"}, 2, "", true)
	}
	checkRun(t, []string{"unresolved", "--root", dir, "--source", "A.md", "--strict"}, 0, "", false)
	for _, control := range []string{"\n", "\r", "\t"} {
		directory := fixture(t, map[string]string{"A" + control + ".md": ""})
		checkRun(t, []string{"unresolved", "--root", directory, "--source", "A" + control + ".md"}, 2, "", true)
	}
}

type brokenSourceInput struct{}

func (brokenSourceInput) Read(p []byte) (int, error) {
	copy(p, "A.md\x00")
	return min(len(p), 5), os.ErrPermission
}

func TestEmptySourcesRetainCatalogDiagnostics(t *testing.T) {
	dir := fixture(t, map[string]string{"A.md": "[[Missing]]"})
	if err := os.Symlink("A.md", filepath.Join(dir, "alias.md")); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"text", "json"} {
		var out, errOut bytes.Buffer
		code := run([]string{"unresolved", "--root", dir, "--sources0-from", "-", "--strict", "--format", format}, strings.NewReader(""), &out, &errOut)
		if code != 3 || strings.Contains(out.String(), "Missing") {
			t.Fatal(code, out.String(), errOut.String())
		}
		if format == "json" && (!strings.Contains(out.String(), `"phase":"catalog"`) || !strings.Contains(out.String(), `"sources":[]`)) {
			t.Fatal(out.String())
		}
		if format == "text" && !strings.Contains(errOut.String(), "phase=catalog") {
			t.Fatal(errOut.String())
		}
	}
}

func TestSourceValidationOrderingAndWalkerRetention(t *testing.T) {
	dir := fixture(t, map[string]string{"A.md": "\xff"})
	if err := os.Symlink("missing", filepath.Join(dir, "alias")); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	code := Run([]string{"unresolved", "--root", dir, "--source", "Missing.md", "--source", "A.md", "--format", "json"}, &out, &errOut)
	if code != 2 || out.Len() != 0 || !strings.Contains(errOut.String(), "phase=catalog") || strings.Contains(errOut.String(), "invalid-encoding") {
		t.Fatal(code, out.String(), errOut.String())
	}
	if code := Run([]string{"unresolved", "--root", dir, "--source", "Missing.md"}, &out, failingWriter{}); code != 1 {
		t.Fatal(code)
	}
	out.Reset()
	errOut.Reset()
	code = Run([]string{"unresolved", "--root", dir, "--follow-symlink", "alias", "--source", "../bad.md"}, &out, &errOut)
	if code != 2 || !strings.Contains(errOut.String(), "invalid source") {
		t.Fatal(code, out.String(), errOut.String())
	}
}

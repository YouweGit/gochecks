package tools

import (
	"testing"

	"github.com/YouweGit/gochecks/internal/check"
)

func TestVetParse(t *testing.T) {
	out := check.Output{ExitCode: 1, Stderr: []byte("# fx\n./a.go:3:2: unreachable code\nvet: ./b.go:4:1: declared and not used\n")}

	got, err := Vet().Parse(out)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 2 || got[0].File != "./a.go" || got[0].Line != 3 || got[1].File != "./b.go" {
		t.Errorf("findings = %+v", got)
	}
}

func TestVetParseToolError(t *testing.T) {
	if _, err := Vet().Parse(check.Output{ExitCode: 2, Stderr: []byte("go: no Go files")}); err == nil {
		t.Error("expected tool error when exit != 0 without findings")
	}
}

func TestGovulncheckParseOnlyReachable(t *testing.T) {
	stream := `{"config":{"scanner_name":"govulncheck"}}
{"finding":{"osv":"GO-1","trace":[{"module":"m","version":"v1"}]}}
{"finding":{"osv":"GO-2","trace":[{"module":"m","version":"v1","function":"F"}]}}
{"finding":{"osv":"GO-2","trace":[{"module":"m","version":"v1","function":"G"}]}}
`

	got, err := Govulncheck().Parse(check.Output{ExitCode: 3, Stdout: []byte(stream)})
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 1 || got[0].Message != "GO-2 in m@v1" {
		t.Errorf("findings = %+v, want only the reachable, de-duplicated GO-2", got)
	}
}

func TestGolangCILintParse(t *testing.T) {
	out := check.Output{ExitCode: 1, Stdout: []byte(`{"Issues":[{"FromLinter":"errcheck","Text":"unchecked","Pos":{"Filename":"a.go","Line":7}}]}` + "\n1 issues:\n* errcheck: 1\n")}

	got, err := GolangCILint().Parse(out)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 1 || got[0].Line != 7 || got[0].Message != "errcheck: unchecked" {
		t.Errorf("findings = %+v", got)
	}
}

func TestGosecParseCleanRunIsEmpty(t *testing.T) {
	got, err := Gosec().Parse(check.Output{})
	if err != nil || len(got) != 0 {
		t.Errorf("Parse(empty) = %v, %v; want no findings and no error", got, err)
	}
}

func TestGoTestParse(t *testing.T) {
	stream := `{"Action":"fail","Package":"p","Test":"TestX"}
{"Action":"fail","Package":"p"}
{"Action":"fail","Package":"q"}
`

	got, err := GoTest().Parse(check.Output{ExitCode: 1, Stdout: []byte(stream)})
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 2 {
		t.Errorf("findings = %+v, want one test failure and one package failure", got)
	}
}

func TestGofmtParse(t *testing.T) {
	got, err := Gofmt().Parse(check.Output{Stdout: []byte("a/x.go\nb/y.go\n")})
	if err != nil || len(got) != 2 || got[0].File != "a/x.go" {
		t.Fatalf("got %v, %v", got, err)
	}

	got, err = Gofmt().Parse(check.Output{ExitCode: 2, Stderr: []byte("a/x.go:3:1: expected declaration")})
	if err != nil || len(got) != 1 || got[0].Line != 3 {
		t.Fatalf("syntax error: got %v, %v", got, err)
	}
}

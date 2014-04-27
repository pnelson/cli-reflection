package cli

import (
	"flag"
	"testing"
)

type runFull struct {
	number *int
}

type runErrMissing struct {
	*NullFlags
}

type runErrString struct {
	*NullFlags
}

type runErrReturnValue struct {
	*NullFlags
}

func TestNew(t *testing.T) {
	app := New("myapp", "0.0.1")
	if len(app.rules) != 2 {
		t.Errorf("default rules\nhave %d\nwant %d", len(app.rules), 2)
	}
}

func TestRuleRunFull(t *testing.T) {
	app := New("myapp", "0.0.1")
	err := app.Rule(&runFull{}, "full", "<arg1> <arg2> [<extra>]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(app.rules) != 3 {
		t.Errorf("rules\nhave %d\nwant %d", len(app.rules), 3)
	}
}

func TestRuleRunMissing(t *testing.T) {
	app := New("myapp", "0.0.1")
	err := app.Rule(&runErrMissing{}, "missing", "")
	if err == nil {
		t.Errorf("error\nhave %v\nwant %v", nil, errRunMissing)
	}
}

func TestRuleRunStrings(t *testing.T) {
	app := New("myapp", "0.0.1")
	err := app.Rule(&runErrString{}, "string", "")
	if err == nil {
		t.Errorf("error\nhave %v\nwant %v", nil, errRunString)
	}
}

func TestRuleRunReturnValue(t *testing.T) {
	app := New("myapp", "0.0.1")
	err := app.Rule(&runErrReturnValue{}, "return", "")
	if err == nil {
		t.Errorf("error\nhave %v\nwant %v", nil, errRunReturnValue)
	}
}

func (c *runFull) Flags(flags *flag.FlagSet) {
	c.number = flags.Int("number", 0, "some number")
}

func (c *runFull) Run(arg1, arg2 string, extra []string) int {
	return 2
}

func (c *runFull) String() string {
	return "runFull help"
}

func (c *runErrString) Run(n int)        {}
func (c *runErrReturnValue) Run() string { return "fail" }

func (c *runErrMissing) String() string     { return "missing run method" }
func (c *runErrString) String() string      { return "invalid param type" }
func (c *runErrReturnValue) String() string { return "invalid return value" }

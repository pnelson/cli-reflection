/*
Package cli provides structure for command line applications with sub-commands.

This package uses a touch of reflection magic to dispatch to a method with
named arguments. Commands help and version are implemented by default. The
usage information is pretty printed in an opinionated format. That said, this
package still attempts to embrace the standard library flag package.

This package assumes that any arguments will remain strings. Any non-string
arguments are likely to be passed as optional flags in practice.

See the documentation of Rule for details and restrictions.
*/
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type application struct {
	name    string
	version string
	rules   map[string]*rule
}

type rule struct {
	command   command
	method    reflect.Method
	slice     bool
	name      string
	options   *flag.FlagSet
	arguments string
}

type command interface {
	fmt.Stringer
	Flags(flags *flag.FlagSet)
}

// NullFlags is an embeddable struct providing an empty FlagSet.
type NullFlags struct{}

var (
	errRunMissing     = fmt.Errorf("rule: missing Run method")
	errRunString      = fmt.Errorf("rule: parameters for Run must be strings")
	errRunReturnValue = fmt.Errorf("rule: first return value for Run must be int")
)

// New creates a basic application with help and version commands.
func New(name, version string) *application {
	app := &application{
		name:    name,
		version: version,
		rules:   make(map[string]*rule),
	}

	app.Rule(&commandHelp{usage: app.usage}, "help", "")
	app.Rule(&commandVersion{name: name, version: version}, "version", "")

	return app
}

// Rule registers a command with the application.
//
// The command being registered must meet the requirements of the fmt.Stringer
// interface. The command must also have a method Flags that accepts a new
// *flag.FlagSet. The Flags method is where you would define flags for this
// particular sub-command.
//
// Additionally, the command must have a Run method. If the Run method has no
// return value, the program will end with a successful exit code. If the Run
// method has one or more return values, only the first is considered and must
// be of type int. The first return value will be used as the exit code.
//
// The Run method may accept parameters of type string. If the Run method has
// more parameters than there are arguments, the extra parameters will just be
// empty strings. If the Run method has less parameters than there are
// arguments, they will silently be ignored. Optionally, the last parameter of
// the Run method can be of type []string. In this case, any extra parameters
// will be passed to the final argument.
func (a *application) Rule(command command, name, arguments string) error {
	// Find the Run method dynamically.
	method, ok := reflect.TypeOf(command).MethodByName("Run")
	if !ok {
		return errRunMissing
	}

	// Ensure that the parameters are all strings.
	in := method.Type.NumIn()
	for i := 1; i < in-1; i++ {
		kind := method.Type.In(i).Kind()
		if kind != reflect.String {
			return errRunString
		}
	}

	// The last parameter may optionally be a string slice.
	slice := false
	if in > 1 {
		final := method.Type.In(in - 1)
		if final.Kind() == reflect.Slice && final.Elem().Kind() == reflect.String {
			slice = true
		} else if final.Kind() != reflect.String {
			return errRunString
		}
	}

	// Ensure that the first return value, if any, is an int.
	if method.Type.NumOut() >= 1 && method.Type.Out(0).Kind() != reflect.Int {
		return errRunReturnValue
	}

	// Register a new FlagSet and define the flags provided by the command.
	options := flag.NewFlagSet(name, flag.ExitOnError)
	command.Flags(options)

	// Add the rule.
	a.rules[name] = &rule{
		command:   command,
		method:    method,
		slice:     slice,
		name:      name,
		options:   options,
		arguments: arguments,
	}

	return nil
}

// Run will parse flags and dispatch to the command.
func (a *application) Run() {
	flag.Usage = a.usage
	flag.Parse()

	// Run requires a command to dispatch to.
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	// Dispatch or error if the command was not registered.
	name := flag.Arg(0)
	rule, ok := a.rules[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: invalid command %s\n", name)
		flag.Usage()
		os.Exit(1)
	}

	// Parse the remaining arguments for the command.
	args := flag.Args()
	rule.options.Parse(args[1:])

	// Prepare the calling parameters.
	params := make([]reflect.Value, rule.method.Type.NumIn())

	// Method expressions take the receiver as the first argument.
	params[0] = reflect.ValueOf(rule.command)

	// Set all but the last parameter.
	args = rule.options.Args()
	for i := 1; i < len(params)-1; i++ {
		if i < len(args)+1 {
			params[i] = reflect.ValueOf(args[i-1])
		} else {
			params[i] = reflect.ValueOf("")
		}
	}

	// Set the final parameter. May be a slice of the remaining args.
	i := len(params) - 1
	if rule.slice {
		params[i] = reflect.Zero(reflect.SliceOf(reflect.TypeOf("")))
		for j := i - 1; j < len(args); j++ {
			params[i] = reflect.Append(params[i], reflect.ValueOf(args[j]))
		}
	} else if i > 1 {
		if i < len(args)+1 {
			params[i] = reflect.ValueOf(args[i-1])
		} else {
			params[i] = reflect.ValueOf("")
		}
	}

	// Call the command Run method.
	rv := rule.method.Func.Call(params)

	// Exit with an appropriate error code.
	code := 0
	if len(rv) > 0 {
		code = int(rv[0].Int())
	}

	os.Exit(code)
}

// Find the longest rule and return its length.
func (a *application) getRuleLength() int {
	max := 0
	for _, rule := range a.rules {
		length := len(rule.String())
		if length > max {
			max = length
		}
	}

	// Add some padding for distinction.
	return max + 3
}

// PrintUsage pretty prints the application usage across all commands.
func (a *application) printUsage(w io.Writer) {
	length := a.getRuleLength()
	fmt.Fprintf(w, "Usage: %s <cmd> [options] [<args>]\n", a.name)
	for _, rule := range a.rules {
		spaces := strings.Repeat(" ", length-len(rule.String()))
		fmt.Fprintf(w, "  %s%s%s\n", rule, spaces, rule.command)

		rule.options.VisitAll(func(flag *flag.Flag) {
			value := flag.DefValue
			if value == "" {
				value = "<value>"
			} else if value == "false" {
				value = ""
			} else if _, err := strconv.Atoi(value); err == nil {
				value = "<n>"
			} else {
				value = "\"" + value + "\""
			}

			option := "-" + flag.Name
			if value != "" {
				option += "=" + value
			}

			spaces := strings.Repeat(" ", length-len(option)-2)
			fmt.Fprintf(w, "    %s%s%s\n", option, spaces, flag.Usage)
		})
	}

	fmt.Fprintf(w, "\n")
}

// Usage is called on flag parsing errors.
func (a *application) usage() {
	a.printUsage(os.Stderr)
}

// String formats the rule for usage printing.
func (r *rule) String() string {
	command := r.name

	options := false
	r.options.VisitAll(func(flag *flag.Flag) {
		options = true
	})

	if options {
		command += " [options]"
	}

	if len(r.arguments) > 0 {
		command += " " + r.arguments
	}

	return command
}

// Flags is a no-op on the command FlagSet.
func (c *NullFlags) Flags(flags *flag.FlagSet) {}

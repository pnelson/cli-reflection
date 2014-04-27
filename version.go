package cli

import "fmt"

type commandVersion struct {
	*NullFlags
	name    string
	version string
}

func (c *commandVersion) Run() {
	fmt.Printf("%s v%s\n", c.name, c.version)
}

func (c *commandVersion) String() string {
	return "Output the application version."
}

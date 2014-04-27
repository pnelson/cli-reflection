package cli

type commandHelp struct {
	*NullFlags
	usage func()
}

func (c *commandHelp) Run() {
	c.usage()
}

func (c *commandHelp) String() string {
	return "Output this usage information."
}

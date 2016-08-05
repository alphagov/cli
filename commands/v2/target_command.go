package v2

import (
	"os"

	"code.cloudfoundry.org/cli/cf/cmd"
)

type TargetCommand struct {
	Organization string `short:"o" description:"Organization"`
	Space        string `short:"s" description:"Space"`
}

func (_ TargetCommand) Execute(args []string) error {
	cmd.Main(os.Getenv("CF_TRACE"), os.Args)
	return nil
}

package cli

var Bin = "cc-util"

func Cmd(args string) string {
	return Bin + " " + args
}

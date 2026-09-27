package cli

var (
	Bin  = "cc-util"
	JSON bool
)

func Cmd(args string) string {
	return Bin + " " + args
}

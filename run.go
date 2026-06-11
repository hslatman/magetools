package magetools

// NOTE: this function is not compiled as a target by Mage, as it
// doesn't have the right signature. It allows one to conveniently
// call magetools.Run(...) to run one of the tools. Unfortunately,
// there doesn't seem to be a way to explicitly exclude a function
// from being compiled, so when running Mage with `-debug`, a line
// is emitted saying this function is skipped.
func Run(args ...string) error {
	r, err := newRunnerFromBinaryName(args[0]) // TODO: add validation?
	if err != nil {
		return err
	}
	return r.run(args[1:]...)
}

// TODO: find workaround? but may be Mage limitation for finding targets
// Also see https://github.com/magefile/mage/pull/491 for additional args
//
// Otherwise we need to do something like this: 'mage tools:run "golangci-lint run"'
// func Run(args string) error {
// 	splittedArgs := strings.Split(args, " ")
// 	r, err := newRunnerFromBinaryName(splittedArgs[0]) // TODO: add validation?
// 	if err != nil {
// 		return err
// 	}
// 	//return r.tool(args[1:]...)
// 	return r.tool(splittedArgs[1:]...)
// }

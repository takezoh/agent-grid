package app

// commands.go wires each verb to the package that performs it. The bodies stay
// thin on purpose: dispatch and the exit-code assignment live here, and every
// decision a command makes lives in a package this one calls.

func runList(deps Deps, streams Streams, args []string) error {
	options, err := parseList(args)
	if err != nil {
		return err
	}
	return listSessions(deps, streams, options)
}

func runAttach(deps Deps, streams Streams, args []string) error {
	options, err := parseAttach(args)
	if err != nil {
		return err
	}
	return attachSession(deps, streams, options)
}

func runNew(deps Deps, streams Streams, args []string) error {
	options, err := parseNew(args)
	if err != nil {
		return err
	}
	return createSession(deps, streams, options)
}

func runKill(deps Deps, streams Streams, args []string) error {
	options, err := parseKill(args)
	if err != nil {
		return err
	}
	return terminateSession(deps, streams, options)
}

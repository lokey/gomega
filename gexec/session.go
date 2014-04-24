/*
Package gexec provides support for testing external processes.
*/
package gexec

import (
	"io"
	"os/exec"
	"reflect"
	"sync"
	"syscall"

	"github.com/onsi/gomega/gbytes"
)

type Session struct {
	//The wrapped command
	Command *exec.Cmd

	//A *gbytes.Buffer connected to the command's stdout
	Out *gbytes.Buffer

	//A *gbytes.Buffer connected to the command's stderr
	Err *gbytes.Buffer

	lock     *sync.Mutex
	exitCode int
}

/*
Start starts the passed-in *exec.Cmd command.  It wraps the command in a *gexec.Session.

The session pipes the command's stdout and stderr to two *gbytes.Buffers available as properties on the session: session.Out and session.Err.
These buffers can be used with the gbytes.Say matcher to match against unread output:

	Ω(session.Out).Should(gbytes.Say("foo"))

When outWriter and/or errWriter are non-nil, the session will pipe stdout and/or stderr output both into the session *gybtes.Buffers and to the passed-in outWriter/errWriter.
This is useful for capturing the process's output or logging it to screen.  In particular, when using Ginkgo it can be convenient to direct output to the GinkgoWriter:

	session, err := Start(command, GinkgoWriter, GinkgoWriter)

This will log output when running tests in verbose mode, but - otherwise - will only log output when a test fails.

The session wrapper is responsible for waiting on the *exec.Cmd command.  You *should not* call command.Wait() yourself.
Instead, to assert that the command has exited you can use the gexec.Exit matcher:

	Ω(session).Should(gexec.Exit())
*/
func Start(command *exec.Cmd, outWriter io.Writer, errWriter io.Writer) (*Session, error) {
	session := &Session{
		Command:  command,
		Out:      gbytes.NewBuffer(),
		Err:      gbytes.NewBuffer(),
		lock:     &sync.Mutex{},
		exitCode: -1,
	}

	var commandOut, commandErr io.Writer

	commandOut, commandErr = session.Out, session.Err

	if outWriter != nil && !reflect.ValueOf(outWriter).IsNil() {
		commandOut = io.MultiWriter(commandOut, outWriter)
	}

	if errWriter != nil && !reflect.ValueOf(errWriter).IsNil() {
		commandErr = io.MultiWriter(commandErr, errWriter)
	}

	command.Stdout = commandOut
	command.Stderr = commandErr

	err := command.Start()
	if err == nil {
		go session.monitorForExit()
	}

	return session, err
}

func (s *Session) monitorForExit() {
	s.Command.Wait()
	s.lock.Lock()
	s.exitCode = s.Command.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
	s.lock.Unlock()
}

func (s *Session) getExitCode() int {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.exitCode
}

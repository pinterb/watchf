package main

import (
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"

	"code.google.com/p/go.exp/fsnotify"
	"github.com/mgutz/ansi"
)

const (
	// VarFilename is used for printing file names
	VarFilename = "%f"
	// VarEventType is used for printing event types
	VarEventType = "%t"
)

// Executor struct models the command(s) to be executed by our watcher
type Executor struct {
	Stdout io.Writer
	Stderr io.Writer
}

func (e *Executor) execute(command string, evt *fsnotify.FileEvent) error {
	command = evaluateVariables(command, evt)
	commandArgs := strings.Split(command, " ")

	var cmd *exec.Cmd
	if len(commandArgs) > 1 {
		cmd = exec.Command(commandArgs[0], commandArgs[1:]...)
	} else {
		cmd = exec.Command(commandArgs[0])
	}
	cmd.Stderr = e.Stderr
	cmd.Stdout = e.Stdout

	msg := fmt.Sprintf("exec: \"%s %s\"", cmd.Args[0], strings.Join(cmd.Args[1:], " "))
	log.Println(ansi.Color("", "cyan+b"))
	log.Println(ansi.Color(evt.String(), "cyan+b"))
	log.Println(ansi.Color(msg, "cyan+b"))
	err := cmd.Run()

	if err != nil {
		msg := fmt.Sprintf("exec: \"%s %s\" failed, err: %s", cmd.Args[0], strings.Join(cmd.Args[1:], " "), err)
		log.Println(ansi.Color(msg, "red+b"))
	}

	return err
}

func evaluateVariables(command string, evt *fsnotify.FileEvent) string {
	command = strings.Replace(command, VarFilename, evt.Name, -1)
	command = strings.Replace(command, VarEventType, getEventType(evt), -1)
	return command
}

package util

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/jedib0t/go-pretty/table"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func GetFilesAndDirectories(path string) (string, []string, []string, error) {
	filenames := []string{}
	directories := []string{}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return "", filenames, directories, fmt.Errorf("%+v", err)
	}

	for _, f := range files {
		if f.IsDir() {
			directories = append(directories, f.Name())
		} else {
			filenames = append(filenames, f.Name())
		}
	}

	return path, directories, filenames, nil
}

// TODO: explore https://github.com/oklog/ulid
func GenerateRandomString(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyz" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)

}

// ChmodX executes chmod +x on given filepath
func ChmodX(filePath string) (string, error) {
	cmd := GetExecCommandFromString(fmt.Sprintf("chmod +x %s", filePath))
	return RunCommand(cmd)
}

func GetExecCommandFromString(fullCmd string) *exec.Cmd {
	cmd := strings.Fields(fullCmd)
	cmdName := cmd[0]
	cmdArgs := cmd[1:]
	return exec.Command(cmdName, cmdArgs...)
}

func RunCommand(cmd *exec.Cmd) (string, error) {
	currDirectory := cmd.Dir
	if 0 == len(currDirectory) {
		currDirectory, _ = os.Executable()
	}

	log.Infof("running command: '%s' in directory: '%s'", cmd.String(), currDirectory)
	cmdOutput, err := cmd.CombinedOutput()
	cmdOutputStr := string(cmdOutput)
	log.Tracef("command: '%s' output:\n%s", cmd.String(), cmdOutput)
	return cmdOutputStr, errors.Wrapf(err, "unable to run command '%s': %s", cmd.String(), cmdOutputStr)
}

// RunAndCaptureProgress runs a long running command and continuously streams its output
func RunAndCaptureProgress(cmd *exec.Cmd) error {
	var stdoutBuf, stderrBuf bytes.Buffer
	stdoutIn, _ := cmd.StdoutPipe()
	stderrIn, _ := cmd.StderrPipe()
	// TODO: not sure why but this is needed, otherwise stdin is constantly fed input
	_, _ = cmd.StdinPipe()

	var errStdout, errStderr error
	stdout := io.MultiWriter(os.Stdout, &stdoutBuf)
	stderr := io.MultiWriter(os.Stderr, &stderrBuf)

	err := cmd.Start()
	if err != nil {
		return errors.Wrapf(err, "cmd.Start() failed for %s", cmd.String())
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		_, errStdout = io.Copy(stdout, stdoutIn)
		wg.Done()
	}()

	_, errStderr = io.Copy(stderr, stderrIn)
	wg.Wait()

	err = cmd.Wait()
	if err != nil {
		return errors.Wrapf(err, "cmd.Wait() failed for %s", cmd.String())
	}

	if errStdout != nil || errStderr != nil {
		return errors.Errorf("failed to capture stdout or stderr from command '%s'", cmd.String())
	}
	// outStr, errStr := string(stdoutBuf.Bytes()), string(stderrBuf.Bytes())
	// log.Debugf("command: %s:\nout:\n%s\nerr:\n%s\n", cmd.String(), outStr, errStr)
	return nil
}

func tableP() {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"#", "First Name", "Last Name", "Salary"})
	t.AppendRows([]table.Row{
		{1, "Arya", "Stark", 3000},
		{20, "Jon", "Snow", 2000, "You know nothing, Jon Snow!"},
	})
	// t.AppendSeparator()
	t.AppendRow([]interface{}{300, "Tyrion", "Lannister", 5000})
	t.AppendFooter(table.Row{"", "", "Total", 10000})
	t.Render()
}

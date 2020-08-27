package util

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/table"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func GetFilesAndDirectories(path string) (string, []string, []string, error) {
	var filenames []string
	var directories []string

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
	var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))
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
	stop := make(chan struct{})
	currDirectory := cmd.Dir
	if 0 == len(currDirectory) {
		currDirectory, _ = os.Executable()
	}
	log.Infof("started command: '%s' in directory: '%s'", cmd.String(), currDirectory)
	go func() {
	ForLoop:
		for {
			log.Infof("waiting for command '%s' ...", cmd.String())
			select {
			case <-stop:
				break ForLoop
			default:
			}
			time.Sleep(5 * time.Second)
		}
	}()
	cmdOutput, err := cmd.CombinedOutput()
	close(stop)
	cmdOutputStr := string(cmdOutput)
	log.Tracef("command: '%s' output:\n%s", cmd.String(), cmdOutput)
	return cmdOutputStr, errors.Wrapf(err, "unable to run command '%s': %s", cmd.String(), cmdOutputStr)
}

// RunCommandAndCaptureProgress runs a long running command and continuously streams its output
func RunCommandAndCaptureProgress(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// can't use CommandRun(cmd) here -- attaching to os pipes interferes with cmd.CombinedOutput()
	log.Infof("running command '%s' with pipes attached in directory: '%s'", cmd.String(), cmd.Dir)
	return errors.Wrapf(cmd.Run(), "unable to run command '%s'", cmd.String())
}

// RunAndCaptureProgress runs a long running command and continuously streams its output
// func RunAndCaptureProgress(cmd *exec.Cmd) error {
// 	var stdoutBuf, stderrBuf bytes.Buffer
// 	stdoutIn, _ := cmd.StdoutPipe()
// 	stderrIn, _ := cmd.StderrPipe()
// 	// TODO: not sure why but this is needed, otherwise stdin is constantly fed input
// 	_, _ = cmd.StdinPipe()
//
// 	var errStdout, errStderr error
// 	stdout := io.MultiWriter(os.Stdout, &stdoutBuf)
// 	stderr := io.MultiWriter(os.Stderr, &stderrBuf)
//
// 	err := cmd.Start()
// 	if err != nil {
// 		return errors.Wrapf(err, "cmd.Start() failed for %s", cmd.String())
// 	}
//
// 	var wg sync.WaitGroup
// 	wg.Add(1)
//
// 	go func() {
// 		_, errStdout = io.Copy(stdout, stdoutIn)
// 		wg.Done()
// 	}()
//
// 	_, errStderr = io.Copy(stderr, stderrIn)
// 	wg.Wait()
//
// 	err = cmd.Wait()
// 	if err != nil {
// 		return errors.Wrapf(err, "cmd.Wait() failed for %s", cmd.String())
// 	}
//
// 	if errStdout != nil || errStderr != nil {
// 		return errors.Errorf("failed to capture stdout or stderr from command '%s'", cmd.String())
// 	}
// 	// outStr, errStr := string(stdoutBuf.Bytes()), string(stderrBuf.Bytes())
// 	// log.Debugf("command: %s:\nout:\n%s\nerr:\n%s\n", cmd.String(), outStr, errStr)
// 	return nil
// }

type ScanStatusTableValues struct {
	ImageName    string
	BlackDuckURL string
}

func PrintScanStatusTable(tableValues <-chan *ScanStatusTableValues, printingFinishedChannel chan<- bool) {
	log.Tracef("inside table printer")
	t := table.NewWriter()
	// t.SetOutputMirror(os.Stdout)
	// t.SetAutoIndex(true)
	t.AppendHeader(table.Row{"Image Name", "BlackDuck URL"})

	// process output structs concurrently
	log.Tracef("waiting for values over channel")
	for tableValue := range tableValues {
		log.Tracef("processing table value for image: %s, url: %s", tableValue.ImageName, tableValue.BlackDuckURL)
		t.AppendRow([]interface{}{
			fmt.Sprintf("%s", tableValue.ImageName),
			fmt.Sprintf("%s", tableValue.BlackDuckURL),
		})
	}
	// TODO: to be able to render concurrently
	log.Tracef("rendering the table")
	fmt.Printf("\n%s\n\n", t.Render())
	printingFinishedChannel <- true
	close(printingFinishedChannel)
}

func SetUpLogger(logLevelStr string) error {
	logLevel, err := log.ParseLevel(logLevelStr)
	if err != nil {
		return errors.Wrapf(err, "unable to parse the specified log level: '%s'", logLevel)
	}
	log.SetLevel(logLevel)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	log.Infof("log level set to '%s'", log.GetLevel())
	return nil
}

func DoOrDie(err error) {
	DoOrDieWithMsg(err, "Fatal error: ")
}

func DoOrDieWithMsg(err error, msg string) {
	if err != nil {
		log.Fatalf("%s; err: %+v\n", msg, err)
	}
}

func GetHomeDir() string {
	homeDir, err := os.UserHomeDir()
	DoOrDieWithMsg(err, "error getting user home directory")
	return homeDir
}

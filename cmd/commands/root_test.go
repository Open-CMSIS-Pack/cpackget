/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-cmsis-pack/cpackget/cmd/commands"
	"github.com/open-cmsis-pack/cpackget/cmd/installer"
	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

// Copy of cmd/log.go
type LogFormatter struct{}

func (s *LogFormatter) Format(entry *log.Entry) ([]byte, error) {
	level := strings.ToUpper(entry.Level.String())
	msg := fmt.Sprintf("%s: %s\n", level[0:1], entry.Message)
	return []byte(msg), nil
}

var testingDir = filepath.Join("..", "..", "testdata", "integration")

type TestCase struct {
	args           []string
	name           string
	defaultMode    bool
	createPackRoot bool
	expectedStdout []string
	expectedStderr []string
	expectedErr    error
	setUpFunc      func(t *TestCase)
	tearDownFunc   func()
	validationFunc func(t *testing.T)
	assert         *assert.Assertions
	env            map[string]string
}

type Server struct {
	routes     map[string][]byte
	httpServer *httptest.Server
}

func (s *Server) URL() string {
	return s.httpServer.URL + "/"
}

func (s *Server) AddRoute(route string, content []byte) {
	s.routes[route] = content
}

// NewServer is a generic dev server that takes in a routes map and returns 404 if the route[path] is nil
// Acessing server.URL should return "Default content"
// Acessing server.URL + "/should-return-404" should return HTTP 404
func NewServer() Server {
	server := Server{}
	server.routes = make(map[string][]byte)
	server.httpServer = httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				path := r.URL.Path
				if len(path) > 1 {
					path = path[1:]
				}
				content, ok := server.routes[path]
				if !ok {
					defaultContent, ok := server.routes["*"]
					if !ok {
						w.WriteHeader(http.StatusNotFound)
						return
					}

					content = defaultContent
				}

				if content == nil {
					w.WriteHeader(http.StatusNotFound)
					return
				}

				reader := bytes.NewReader(content)
				_, _ = io.Copy(w, reader)
			},
		),
	)

	return server
}

var rootCmdTests = []TestCase{
	{
		name:           "test no parameter given",
		expectedStdout: []string{"Please refer to the upstream repository for further information"},
	},
	{
		name:        "test no parameter given",
		args:        []string{"help"},
		expectedErr: nil,
	},
	{
		name:        "test unknown command",
		args:        []string{"this-command-does-not-exist"},
		expectedErr: errors.New("unknown command \"this-command-does-not-exist\" for \"cpackget\""),
	},
	{
		name:           "test get version",
		args:           []string{"--version"},
		expectedStdout: []string{"cpackget version testing#123"},
		setUpFunc: func(t *TestCase) {
			commands.Version = "testing#123"
		},
		tearDownFunc: func() {
			commands.Version = ""
		},
	},
}

func runTests(t *testing.T, tests []TestCase) {
	assert := assert.New(t)

	for i := range tests {
		test := tests[i]
		test.assert = assert
		t.Run(test.name, func(t *testing.T) {
			localTestingDir := strings.ReplaceAll(test.name, " ", "_")
			if test.defaultMode {
				os.Setenv("CPACKGET_DEFAULT_MODE_PATH", localTestingDir)
			}

			os.Setenv("CMSIS_PACK_ROOT", localTestingDir)
			if test.createPackRoot {
				assert.Nil(installer.SetPackRoot(localTestingDir, test.createPackRoot))
				installer.UnlockPackRoot()
			}

			if test.env != nil {
				for envVar := range test.env {
					os.Setenv(envVar, test.env[envVar])
				}
			}

			if test.setUpFunc != nil {
				test.setUpFunc(&test)
			}

			if test.tearDownFunc != nil {
				defer test.tearDownFunc()
			}

			defer func() {
				utils.UnsetReadOnlyR(localTestingDir)
				os.RemoveAll(localTestingDir)
			}()

			cmd := commands.NewCli()

			stdout := bytes.NewBufferString("")
			stderr := bytes.NewBufferString("")

			cmd.SetOut(stdout)
			cmd.SetErr(stderr)
			cmd.SetArgs(test.args)

			cmdErr := cmd.Execute()
			// Very important: resets all flags, as apparently
			// Cobra doesn't do that.
			// Otherwise, the first time a command uses a flag,
			// it will taint the others.
			// Ref: https://github.com/spf13/cobra/issues/1488
			for _, c := range cmd.Commands() {
				c.Flags().VisitAll(func(f *pflag.Flag) {
					if f.Changed {
						_ = f.Value.Set(f.DefValue)
						f.Changed = false
					}
				})
			}

			outBytes, err1 := ioutil.ReadAll(stdout)
			errBytes, err2 := ioutil.ReadAll(stderr)
			assert.Nil(err1)
			assert.Nil(err2)

			outStr := string(outBytes)
			errStr := string(errBytes)

			assert.Equal(test.expectedErr, cmdErr)
			for _, expectedStr := range test.expectedStdout {
				assert.Contains(outStr, expectedStr)
			}

			for _, expectedStr := range test.expectedStderr {
				assert.Contains(errStr, expectedStr)
			}

			if test.validationFunc != nil {
				test.validationFunc(t)
			}
		})
	}
}

func TestRootCmd(t *testing.T) {
	runTests(t, rootCmdTests)
}

func init() {
	logLevel := log.InfoLevel
	if os.Getenv("LOG_LEVEL") == "debug" {
		logLevel = log.DebugLevel
	}
	log.SetLevel(logLevel)
	log.SetFormatter(new(LogFormatter))
}

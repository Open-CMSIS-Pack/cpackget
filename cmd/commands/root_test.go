/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package commands_test

import (
	"bytes"
	"errors"
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
	"github.com/stretchr/testify/assert"
)

var testingDir = filepath.Join("..", "..", "testdata", "integration")

type TestCase struct {
	args           []string
	name           string
	createPackRoot bool
	expectedStdout []string
	expectedStderr []string
	expectedErr    error
	setUpFunc      func(t *TestCase)
	tearDownFunc   func()
	validationFunc func(t *testing.T)
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
// Ex:
// server := NewServer(map[string][]byte{
// 	"*": []byte("Default content"),
// 	"should-return-404": nil,
// })
//
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
		t.Run(test.name, func(t *testing.T) {
			if test.createPackRoot {
				localTestingDir := strings.ReplaceAll(test.name, " ", "_")
				assert.Nil(installer.SetPackRoot(localTestingDir, test.createPackRoot))
				defer os.RemoveAll(localTestingDir)
				os.Setenv("CMSIS_PACK_ROOT", localTestingDir)
			} else {
				os.Setenv("CMSIS_PACK_ROOT", "")
			}

			if test.setUpFunc != nil {
				test.setUpFunc(&test)
			}

			if test.tearDownFunc != nil {
				defer test.tearDownFunc()
			}

			cmd := commands.NewCli()

			stdout := bytes.NewBufferString("")
			stderr := bytes.NewBufferString("")

			cmd.SetOut(stdout)
			cmd.SetErr(stderr)
			cmd.SetArgs(test.args)

			cmdErr := cmd.Execute()

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

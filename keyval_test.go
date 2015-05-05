package keyval_test

import (
	. "github.com/smartystreets/goconvey/convey"

	"github.com/evergreen-ci/evergreen"
	"github.com/evergreen-ci/evergreen/agent"
	"github.com/evergreen-ci/evergreen/apiserver"
	"github.com/evergreen-ci/evergreen/db"
	"github.com/evergreen-ci/evergreen/plugin"
	"github.com/evergreen-ci/evergreen/plugin/testutil"
	"github.com/evergreen-ci/evergreen/util"
	"github.com/10gen-labs/slogger/v1"
	"github.com/mpobrien/keyval"
	"testing"
)

func init() {
	db.SetGlobalSessionProvider(db.SessionFactoryFromConfig(evergreen.TestConfig()))
}

func TestIncKey(t *testing.T) {
	Convey("With keyval plugin installed", t, func() {
		err := db.Clear(keyval.KeyValCollection)
		util.HandleTestingErr(err, t, "Couldn't clear test collection: %v")
		registry := plugin.NewSimpleRegistry()
		kvPlugin := &keyval.KeyValPlugin{}
		err = registry.Register(kvPlugin)
		util.HandleTestingErr(err, t, "Couldn't register plugin: %v")

		server, err := apiserver.CreateTestServer(evergreen.TestConfig(), nil, []plugin.Plugin{kvPlugin}, false)
		httpCom := testutil.TestAgentCommunicator("mocktaskid", "mocktasksecret", server.URL)
		sliceAppender := &evergreen.SliceAppender{[]*slogger.Log{}}
		logger := agent.NewTestAgentLogger(sliceAppender)
		taskConfig, err := testutil.CreateTestConfig("testdata/plugin_keyval.yml", t)
		util.HandleTestingErr(err, t, "failed to create test config")

		Convey("Inc command should increment a key successfully", func() {
			for _, task := range taskConfig.Project.Tasks {
				So(len(task.Commands), ShouldNotEqual, 0)
				for _, command := range task.Commands {
					pluginCmds, err := registry.GetCommands(command, nil)
					util.HandleTestingErr(err, t, "Couldn't get plugin command: %v")
					So(pluginCmds, ShouldNotBeNil)
					So(err, ShouldBeNil)
					for _, cmd := range pluginCmds {
						pluginCom := &agent.TaskJSONCommunicator{cmd.Plugin(), httpCom}
						err = cmd.Execute(logger, pluginCom, taskConfig, make(chan bool))
						So(err, ShouldBeNil)
					}
				}
				So(taskConfig.Expansions.Get("testkey"), ShouldEqual, "2")
				So(taskConfig.Expansions.Get("testkey_x"), ShouldEqual, "1")
			}
		})
	})
}

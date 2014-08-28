package keyval

import (
	"10gen.com/mci"
	"10gen.com/mci/db"
	"10gen.com/mci/model"
	"10gen.com/mci/plugin"
	"10gen.com/mci/util"
	"10gen.com/mci/web"
	"fmt"
	"github.com/10gen-labs/slogger/v1"
	"github.com/mitchellh/mapstructure"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"net/http"
)

const (
	KeyValCollection = "keyval_plugin"
	KeyValPluginName = "keyval"
	IncCommandName   = "inc"
	IncRoute         = "inc"
)

type KeyVal struct {
	Key   string      `bson:"_id" json:"key"`
	Value interface{} `bson:"value" json:"value"`
}

func init() {
	plugin.Publish(&KeyValPlugin{})
}

type KeyValPlugin struct{}

func (self *KeyValPlugin) GetUIConfig() (*plugin.UIConfig, error) {
	return nil, nil
}

func (self *KeyValPlugin) Name() string {
	return KeyValPluginName
}

type IncCommand struct {
	Key         string `mapstructure:"key"`
	Destination string `mapstructure:"destination"`
}

func (self *IncCommand) Name() string {
	return IncCommandName
}

// ParseParams validates the input to the IncCommand, returning an error
// if something is incorrect. Fulfills PluginCommand interface.
func (incCmd *IncCommand) ParseParams(params map[string]interface{}) error {
	err := mapstructure.Decode(params, incCmd)
	if err != nil {
		return err
	}

	if incCmd.Key == "" || incCmd.Destination == "" {
		return fmt.Errorf("error parsing '%v' params: key and destination may not be blank",
			IncCommandName)
	}

	return nil
}

// GetRoutes returns the routes to be bound by the API server
func (self *KeyValPlugin) GetRoutes() []plugin.PluginRoute {
	return []plugin.PluginRoute{
		{fmt.Sprintf("/%v", IncRoute), IncKeyHandler, []string{"POST"}},
	}
}

// IncKeyHandler increments the value stored in the given key, and returns it
func IncKeyHandler(request *http.Request) web.HTTPResponse {
	key := ""

	err := util.ReadJSONInto(request.Body, &key)
	if err != nil {
		mci.Logger.Logf(slogger.ERROR, "Error geting key: %v", err)
		return web.JSONResponse{fmt.Sprintf("Error: %v", err), http.StatusInternalServerError}
	}

	change := mgo.Change{
		Update: bson.M{
			"$inc": bson.M{"value": 1},
		},
		ReturnNew: true,
		Upsert:    true,
	}

	keyVal := &KeyVal{}
	_, err = db.FindAndModify(KeyValCollection, bson.M{"_id": key}, change, keyVal)
	if err != nil {
		return web.JSONResponse{fmt.Sprintf("Error: %v", err), http.StatusInternalServerError}
	}

	return web.JSONResponse{keyVal, http.StatusOK}
}

// Execute fetches the expansions from the API server
func (incCmd *IncCommand) Execute(pluginLogger plugin.PluginLogger,
	pluginCom plugin.PluginCommunicator, conf *model.TaskConfig,
	stop chan bool) error {

	err := plugin.ExpandValues(incCmd, conf.Expansions)
	if err != nil {
		return err
	}

	keyVal := &KeyVal{}
	resp, err := pluginCom.TaskPostJSON(IncRoute, incCmd.Key)
	if err != nil {
		return err
	}
	if resp == nil {
		return fmt.Errorf("received nil response from inc API call")
	} else {
		defer resp.Body.Close()
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %v", resp.StatusCode)
	}

	err = util.ReadJSONInto(resp.Body, keyVal)
	if err != nil {
		return fmt.Errorf("Failed to read JSON reply: %v", err)
	}

	conf.Expansions.Put(incCmd.Destination, fmt.Sprintf("%v", keyVal.Value))
	return nil
}

func (self *KeyValPlugin) NewPluginCommand(cmdName string) (plugin.PluginCommand, error) {
	if cmdName == IncCommandName {
		return &IncCommand{}, nil
	}
	return nil, &plugin.UnknownCommandError{cmdName}
}

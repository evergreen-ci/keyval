package keyval

import (
	"10gen.com/mci"
	"10gen.com/mci/db"
	"10gen.com/mci/model"
	"10gen.com/mci/plugin"
	"10gen.com/mci/util"
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
	Key   string `bson:"_id" json:"key"`
	Value int64  `bson:"value" json:"value"`
}

func init() {
	plugin.Publish(&KeyValPlugin{})
}

type KeyValPlugin struct{}

func (self *KeyValPlugin) GetPanelConfig() (*plugin.PanelConfig, error) {
	return nil, nil
}

func (self *KeyValPlugin) Configure(map[string]interface{}) error {
	return nil
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

func (self *IncCommand) Plugin() string {
	return KeyValPluginName
}

// ParseParams validates the input to the IncCommand, returning an error
// if something is incorrect. Fulfills Command interface.
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

func (self *KeyValPlugin) GetUIHandler() http.Handler {
	return nil
}

// GetRoutes returns the routes to be bound by the API server
func (self *KeyValPlugin) GetAPIHandler() http.Handler {
	r := http.NewServeMux()
	r.HandleFunc("/inc", IncKeyHandler)
	r.HandleFunc("/", http.NotFound)
	return r
}

// IncKeyHandler increments the value stored in the given key, and returns it
func IncKeyHandler(w http.ResponseWriter, r *http.Request) {
	key := ""
	err := util.ReadJSONInto(r.Body, &key)
	if err != nil {
		mci.Logger.Logf(slogger.ERROR, "Error geting key: %v", err)
		plugin.WriteJSON(w, http.StatusInternalServerError, err.Error())
		return
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
		mci.Logger.Logf(slogger.ERROR, "error doing findAndModify: %v", err)
		plugin.WriteJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	plugin.WriteJSON(w, http.StatusOK, keyVal)
}

// Execute fetches the expansions from the API server
func (incCmd *IncCommand) Execute(pluginLogger plugin.Logger,
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

	conf.Expansions.Put(incCmd.Destination, fmt.Sprintf("%d", keyVal.Value))
	return nil
}

func (self *KeyValPlugin) NewCommand(cmdName string) (plugin.Command, error) {
	if cmdName == IncCommandName {
		return &IncCommand{}, nil
	}
	return nil, &plugin.ErrUnknownCommand{cmdName}
}

package hummelapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const actDir = "./acts"

type (
	LEDInfectedAct struct {
		ActID        string                   `json:"act_id"`
		Description  string                   `json:"description"`
		StartSceneID string                   `json:"start_scene_id"`
		Scenes       []*LEDInfectedScene      `json:"scenes"`
		Triggers     []*LEDInfectedActTrigger `json:"triggers"`
		Status       *LEDInfectedActStatus    `json:"status"`

		getAllAbstracts func() []*LEDInfectedAbstract
	}
)

func GetAllActs(getAllAbstracts func() []*LEDInfectedAbstract) []*LEDInfectedAct {
	acts := []*LEDInfectedAct{}

	matches, err := filepath.Glob(actDir + "/*.json")
	if err != nil {
		return acts
	}
	for _, m := range matches {
		buf, err := os.ReadFile(m)
		if err != nil {
			fmt.Printf("failed to read %s: %s\n", m, err)
			continue
		}
		act := &LEDInfectedAct{}
		err = json.Unmarshal(buf, act)
		if err != nil {
			fmt.Printf("failed to unmarshal %s: %s\n", m, err)
			continue
		}

		if act.ActID == "" {
			fmt.Printf("invalid act with no act ID %s: %s\n", m, act)
			continue
		}

		act.getAllAbstracts = getAllAbstracts

		// fill missing fields
		act.Status = &LEDInfectedActStatus{
			State: "INACTIVE",
		}
		presets := GetAllPresets()
		for _, s := range act.Scenes {
			for _, e := range s.Effects {
				for _, p := range presets {
					if p.PresetID == e.PresetID {
						e.preset = p
					}
				}
				for _, a := range act.getAllAbstracts() {
					if a.AbstractID == e.AbstractID {
						e.abstract = a
					}
				}
			}
			for _, t := range s.Transitions {
				t.timer = time.NewTimer(time.Minute)
				t.timer.Stop()
				t.act = act
				t.stop = make(chan struct{})
				if t.Trigger.ActTriggerID != nil {
					actTrigger, err := act.getActTrigger(*t.Trigger.ActTriggerID)
					if err != nil {
						act.Status.appendError(fmt.Errorf("failed to get act trigger %s: %s", m, err))
					}
					t.actTrigger = actTrigger
				}
				for _, is := range act.Scenes {
					if is.SceneID == t.SceneID {
						t.nextScene = is
					}
				}
				if t.nextScene == nil {
					act.Status.appendError(fmt.Errorf("failed to get next scene %s", t.SceneID))
				}
			}
		}

		acts = append(acts, act)
	}
	return acts
}

func UpdateAct(acts []*LEDInfectedAct, act *LEDInfectedAct) error {
	if act == nil || act.ActID == "" {
		return fmt.Errorf("invalid act given")
	}
	for i, a := range acts {
		if a.ActID == act.ActID {
			// write the file
			if err := writeJson(actDir+"/"+act.ActID+".json", act); err != nil {
				return err
			}

			// update the array
			acts[i] = act
			return nil
		}
	}

	return fmt.Errorf("act %s not found", act.ActID)
}

func (o *LEDInfectedAct) Start() error {
	switch o.Status.State {
	case "RUNNING":
		return fmt.Errorf("failed to start act %s: already running", o.ActID)
	case "STOP", "INACTIVE":
	default:
		o.Status.appendError(fmt.Errorf("WARN: unhandled act (%s) state %s", o.ActID, o.Status.State))
	}
	o.Status.State = "RUNNING"
	return o.enableScene()
}

func (o *LEDInfectedAct) Stop() error {
	switch o.Status.State {
	case "RUNNING":
	case "STOP", "INACTIVE":
		return fmt.Errorf("failed to stop act %s: not running", o.ActID)
	default:
		o.Status.appendError(fmt.Errorf("WARN: unhandled act (%s) state %s", o.ActID, o.Status.State))
	}

	o.Status.State = "STOP"
	return o.disableScene()
}

func (o *LEDInfectedAct) enableScene() error {
	if o.Status.ActiveScene == nil {
		found := false
		for _, s := range o.Scenes {
			if s.SceneID == o.StartSceneID {
				o.Status.ActiveScene = s
				found = true
			}
		}
		if !found {
			return fmt.Errorf("start scene %s not found", o.StartSceneID)
		}
	}

	return o.Status.ActiveScene.Run()
}

func (o *LEDInfectedAct) disableScene() error {
	if o.Status.ActiveScene != nil {
		return o.Status.ActiveScene.Stop()
	}

	return fmt.Errorf("currently no scene active")
}

func (o *LEDInfectedAct) getActTrigger(actTriggerID string) (*LEDInfectedActTrigger, error) {
	for _, actTrigger := range o.Triggers {
		if actTrigger.ActTriggerID == actTriggerID {
			return actTrigger, nil
		}
	}

	return nil, fmt.Errorf("act trigger %s not found", actTriggerID)
}
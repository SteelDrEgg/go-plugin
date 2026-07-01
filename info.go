package goplugin

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ReadInfo reads an info.yaml file into Info.
// Required fields are mapped to Info directly; all other fields are kept in Metadata.
func ReadInfo(infoFile string) (Info, error) {
	infoBytes, err := os.ReadFile(infoFile)
	if err != nil {
		return Info{}, fmt.Errorf("read info.yaml: %w", err)
	}

	var info Info
	if err := yaml.Unmarshal(infoBytes, &info); err != nil {
		return Info{}, fmt.Errorf("parse info.yaml: %w", err)
	}

	if err := validateInfo(info); err != nil {
		return Info{}, err
	}

	return info, nil
}

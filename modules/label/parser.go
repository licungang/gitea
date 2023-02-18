// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package label

import (
	"errors"
	"fmt"
	"strings"

	"code.gitea.io/gitea/modules/options"

	"gopkg.in/yaml.v3"
)

type labelFile struct {
	Labels []*Label `yaml:"labels"`
}

// ErrTemplateLoad represents a "ErrTemplateLoad" kind of error.
type ErrTemplateLoad struct {
	TemplateFile  string
	OriginalError error
}

// IsErrTemplateLoad checks if an error is a ErrTemplateLoad.
func IsErrTemplateLoad(err error) bool {
	_, ok := err.(ErrTemplateLoad)
	return ok
}

func (err ErrTemplateLoad) Error() string {
	return fmt.Sprintf("Failed to load label template file '%s': %v", err.TemplateFile, err.OriginalError)
}

// GetTemplateFile loads the label template file by given name,
// then parses and returns a list of name-color pairs and optionally description.
func GetTemplateFile(name string) ([]*Label, error) {
	data, err := options.GetRepoInitFile("label", name+".yaml")
	if err == nil && len(data) > 0 {
		return parseYamlFormat(name+".yaml", data)
	}

	data, err = options.GetRepoInitFile("label", name+".yml")
	if err == nil && len(data) > 0 {
		return parseYamlFormat(name+".yml", data)
	}

	data, err = options.GetRepoInitFile("label", name)
	if err != nil {
		return nil, ErrTemplateLoad{name, fmt.Errorf("GetRepoInitFile: %w", err)}
	}

	return parseDefaultFormat(name, data)
}

func parseYamlFormat(name string, data []byte) ([]*Label, error) {
	lf := &labelFile{}

	if err := yaml.Unmarshal(data, lf); err != nil {
		return nil, err
	}

	// Validate label data and fix colors
	for _, l := range lf.Labels {
		l.Color = strings.TrimSpace(l.Color)
		if len(l.Name) == 0 || len(l.Color) == 0 {
			return nil, ErrTemplateLoad{name, errors.New("label name and color are required fields")}
		}
		if len(l.Color) == 6 || len(l.Color) == 3 {
			l.Color = "#" + l.Color
		}
		if !ColorPattern.MatchString(l.Color) {
			return nil, ErrTemplateLoad{name, fmt.Errorf("bad HTML color code in label: %s", l.Name)}
		}
		l.Color = NormalizeColor(l.Color)
	}

	return lf.Labels, nil
}

func parseDefaultFormat(name string, data []byte) ([]*Label, error) {
	lines := strings.Split(string(data), "\n")
	list := make([]*Label, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}

		parts := strings.SplitN(line, ";", 2)

		fields := strings.SplitN(parts[0], " ", 2)
		if len(fields) != 2 {
			return nil, ErrTemplateLoad{name, fmt.Errorf("line is malformed: %s", line)}
		}

		color := strings.Trim(fields[0], " ")
		if len(color) == 6 {
			color = "#" + color
		}
		if !ColorPattern.MatchString(color) {
			return nil, ErrTemplateLoad{name, fmt.Errorf("bad HTML color code in line: %s", line)}
		}

		var description string

		if len(parts) > 1 {
			description = strings.TrimSpace(parts[1])
		}

		fields[1] = strings.TrimSpace(fields[1])
		list = append(list, &Label{
			Name:        fields[1],
			Color:       NormalizeColor(color),
			Description: description,
		})
	}

	return list, nil
}

// LoadFormatted loads the labels' list of a template file as a string separated by comma
func LoadFormatted(name string) (string, error) {
	var buf strings.Builder
	list, err := GetTemplateFile(name)
	if err != nil {
		return "", err
	}

	for i := 0; i < len(list); i++ {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(list[i].Name)
	}
	return buf.String(), nil
}

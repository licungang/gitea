// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package convert

import (
	"code.gitea.io/gitea/models/auth"
	api "code.gitea.io/gitea/modules/structs"
)

// ToAuthSources convert a list of auth.Source to a list of api.AuthSource
func ToAuthSources(sources []*auth.Source) ([]*api.AuthSource, error) {
	result := make([]*api.AuthSource, len(sources))
	for i, source := range sources {
		authSource, err := ToAuthSource(source)
		if err != nil {
			return nil, err
		}
		result[i] = authSource
	}
	return result, nil
}

// ToAuthSource convert a auth.Source to a api.AuthSource
func ToAuthSource(source *auth.Source) (*api.AuthSource, error) {
	cfg, err := source.Cfg.ToDB()
	if err != nil {
		return nil, err
	}
	authSource := &api.AuthSource{
		ID:            source.ID,
		Name:          source.Name,
		Type:          auth.Names[source.Type],
		IsActive:      source.IsActive,
		IsSyncEnabled: source.IsSyncEnabled,
		CreatedTime:   source.CreatedUnix.AsTime(),
		UpdatedTime:   source.UpdatedUnix.AsTime(),
		Cfg:           cfg,
	}
	return authSource, nil
}

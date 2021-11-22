// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func dropTableRemoteVersion(x *xorm.Engine) error {
	// drop the orphaned table introduced in `v199`, now the update checker also uses AppState, do not need this table
	_ = x.DropTables("remote_version")
	return nil
}

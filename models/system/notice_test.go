// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package system_test

import (
	"testing"

	"code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/db"

	"github.com/stretchr/testify/assert"
)

func TestNotice_TrStr(t *testing.T) {
	notice := &system.Notice{
		Type:        system.NoticeRepository,
		Description: "test description",
	}
	assert.Equal(t, "admin.notices.type_1", notice.TrStr())
}

func TestCreateNotice(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	noticeBean := &system.Notice{
		Type:        system.NoticeRepository,
		Description: "test description",
	}
	unittest.AssertNotExistsBean(t, noticeBean)
	assert.NoError(t, system.CreateNotice(db.DefaultContext, noticeBean.Type, noticeBean.Description))
	unittest.AssertExistsAndLoadBean(t, noticeBean)
}

func TestCreateRepositoryNotice(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	noticeBean := &system.Notice{
		Type:        system.NoticeRepository,
		Description: "test description",
	}
	unittest.AssertNotExistsBean(t, noticeBean)
	assert.NoError(t, system.CreateRepositoryNotice(noticeBean.Description))
	unittest.AssertExistsAndLoadBean(t, noticeBean)
}

// TODO TestRemoveAllWithNotice

func TestCountNotices(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	assert.Equal(t, int64(3), system.CountNotices())
}

func TestNotices(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	notices, err := system.Notices(1, 2)
	assert.NoError(t, err)
	if assert.Len(t, notices, 2) {
		assert.Equal(t, int64(3), notices[0].ID)
		assert.Equal(t, int64(2), notices[1].ID)
	}

	notices, err = system.Notices(2, 2)
	assert.NoError(t, err)
	if assert.Len(t, notices, 1) {
		assert.Equal(t, int64(1), notices[0].ID)
	}
}

func TestDeleteNotice(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	unittest.AssertExistsAndLoadBean(t, &system.Notice{ID: 3})
	assert.NoError(t, system.DeleteNotice(3))
	unittest.AssertNotExistsBean(t, &system.Notice{ID: 3})
}

func TestDeleteNotices(t *testing.T) {
	// delete a non-empty range
	assert.NoError(t, unittest.PrepareTestDatabase())

	unittest.AssertExistsAndLoadBean(t, &system.Notice{ID: 1})
	unittest.AssertExistsAndLoadBean(t, &system.Notice{ID: 2})
	unittest.AssertExistsAndLoadBean(t, &system.Notice{ID: 3})
	assert.NoError(t, system.DeleteNotices(1, 2))
	unittest.AssertNotExistsBean(t, &system.Notice{ID: 1})
	unittest.AssertNotExistsBean(t, &system.Notice{ID: 2})
	unittest.AssertExistsAndLoadBean(t, &system.Notice{ID: 3})
}

func TestDeleteNotices2(t *testing.T) {
	// delete an empty range
	assert.NoError(t, unittest.PrepareTestDatabase())

	unittest.AssertExistsAndLoadBean(t, &system.Notice{ID: 1})
	unittest.AssertExistsAndLoadBean(t, &system.Notice{ID: 2})
	unittest.AssertExistsAndLoadBean(t, &system.Notice{ID: 3})
	assert.NoError(t, system.DeleteNotices(3, 2))
	unittest.AssertExistsAndLoadBean(t, &system.Notice{ID: 1})
	unittest.AssertExistsAndLoadBean(t, &system.Notice{ID: 2})
	unittest.AssertExistsAndLoadBean(t, &system.Notice{ID: 3})
}

func TestDeleteNoticesByIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	unittest.AssertExistsAndLoadBean(t, &system.Notice{ID: 1})
	unittest.AssertExistsAndLoadBean(t, &system.Notice{ID: 2})
	unittest.AssertExistsAndLoadBean(t, &system.Notice{ID: 3})
	assert.NoError(t, system.DeleteNoticesByIDs([]int64{1, 3}))
	unittest.AssertNotExistsBean(t, &system.Notice{ID: 1})
	unittest.AssertExistsAndLoadBean(t, &system.Notice{ID: 2})
	unittest.AssertNotExistsBean(t, &system.Notice{ID: 3})
}

// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT


package admin

import (
	"net/http"
	"strconv"

	system_model "code.gitea.io/gitea/models/system"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
)

const (
	tplNotices base.TplName = "admin/notice"
)

// Notices show notices for admin
func Notices(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.notices")
	ctx.Data["PageIsAdmin"] = true
	ctx.Data["PageIsAdminNotices"] = true

	total := system_model.CountNotices()
	page := ctx.FormInt("page")
	if page <= 1 {
		page = 1
	}

	notices, err := system_model.Notices(page, setting.UI.Admin.NoticePagingNum)
	if err != nil {
		ctx.ServerError("Notices", err)
		return
	}
	ctx.Data["Notices"] = notices

	ctx.Data["Total"] = total

	ctx.Data["Page"] = context.NewPagination(int(total), setting.UI.Admin.NoticePagingNum, page, 5)

	ctx.HTML(http.StatusOK, tplNotices)
}

// DeleteNotices delete the specific notices
func DeleteNotices(ctx *context.Context) {
	strs := ctx.FormStrings("ids[]")
	ids := make([]int64, 0, len(strs))
	for i := range strs {
		id, _ := strconv.ParseInt(strs[i], 10, 64)
		if id > 0 {
			ids = append(ids, id)
		}
	}

	if err := system_model.DeleteNoticesByIDs(ids); err != nil {
		ctx.Flash.Error("DeleteNoticesByIDs: " + err.Error())
		ctx.Status(http.StatusInternalServerError)
	} else {
		ctx.Flash.Success(ctx.Tr("admin.notices.delete_success"))
		ctx.Status(http.StatusOK)
	}
}

// EmptyNotices delete all the notices
func EmptyNotices(ctx *context.Context) {
	if err := system_model.DeleteNotices(0, 0); err != nil {
		ctx.ServerError("DeleteNotices", err)
		return
	}

	log.Trace("System notices deleted by admin (%s): [start: %d]", ctx.Doer.Name, 0)
	ctx.Flash.Success(ctx.Tr("admin.notices.delete_success"))
	ctx.Redirect(setting.AppSubURL + "/admin/notices")
}
